package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ApprovalState is the human approval lifecycle.
type ApprovalState string

const (
	ApprovalPending  ApprovalState = "pending"
	ApprovalApproved ApprovalState = "approved"
	ApprovalDenied   ApprovalState = "denied"
)

// ApprovalRecord captures one approval decision.
type ApprovalRecord struct {
	ID           string        `json:"id"`
	SessionID    string        `json:"session_id"`
	Command      string        `json:"command"`
	Paths        []string      `json:"paths"`
	State        ApprovalState `json:"state"`
	RequestedAt  time.Time     `json:"requested_at"`
	ApprovedAt   *time.Time    `json:"approved_at,omitempty"`
	Approver     string        `json:"approver,omitempty"`
	Reason       string        `json:"reason,omitempty"`
	ExpiresAt    *time.Time    `json:"expires_at,omitempty"`
	AutoApproved bool          `json:"auto_approved"`
}

// ApprovalQueue persists approvals and session-level whitelists.
type ApprovalQueue struct {
	mu        sync.Mutex
	cond      *sync.Cond
	storePath string
	records   map[string]*ApprovalRecord
	whitelist map[string]time.Time
	clock     func() time.Time
}

// NewApprovalQueue restores queue state from disk or creates a fresh one.
func NewApprovalQueue(storePath string) (*ApprovalQueue, error) {
	q := &ApprovalQueue{
		storePath: storePath,
		records:   make(map[string]*ApprovalRecord),
		whitelist: make(map[string]time.Time),
		clock:     time.Now,
	}
	q.cond = sync.NewCond(&q.mu)
	if err := q.load(); err != nil {
		return nil, err
	}
	return q, nil
}

// Request enqueues a command for approval. Whitelisted sessions auto-pass.
func (q *ApprovalQueue) Request(sessionID, command string, paths []string) (*ApprovalRecord, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("security: session id required")
	}
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("security: command required")
	}

	sanitized := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		sanitized = append(sanitized, normalizePath(p))
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	q.ensureCondLocked()

	now := q.clock()
	record := &ApprovalRecord{
		ID:          newApprovalID(),
		SessionID:   sessionID,
		Command:     command,
		Paths:       sanitized,
		State:       ApprovalPending,
		RequestedAt: now,
	}

	if expiry, ok := q.whitelist[sessionID]; ok && expiry.After(now) {
		record.State = ApprovalApproved
		record.AutoApproved = true
		when := now
		record.ApprovedAt = &when
		record.Reason = "session whitelisted"
	}

	q.records[record.ID] = record
	if err := q.persistLocked(); err != nil {
		return nil, err
	}
	return cloneRecord(record), nil
}

// Approve marks a pending record as approved and optionally whitelists the session.
func (q *ApprovalQueue) Approve(id, approver string, whitelistTTL time.Duration) (*ApprovalRecord, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ensureCondLocked()

	rec, ok := q.records[id]
	if !ok {
		return nil, fmt.Errorf("security: approval %s not found", id)
	}
	if rec.State == ApprovalDenied {
		return nil, fmt.Errorf("security: approval %s already denied", id)
	}

	now := q.clock()
	rec.State = ApprovalApproved
	rec.Approver = approver
	rec.Reason = "manual approval"
	rec.AutoApproved = false
	rec.ApprovedAt = &now

	if whitelistTTL > 0 {
		expiry := now.Add(whitelistTTL)
		q.whitelist[rec.SessionID] = expiry
		rec.ExpiresAt = &expiry
	} else {
		delete(q.whitelist, rec.SessionID)
		rec.ExpiresAt = nil
	}

	if err := q.persistLocked(); err != nil {
		return nil, err
	}
	q.cond.Broadcast()
	return cloneRecord(rec), nil
}

// Deny rejects a pending record.
func (q *ApprovalQueue) Deny(id, approver, reason string) (*ApprovalRecord, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ensureCondLocked()

	rec, ok := q.records[id]
	if !ok {
		return nil, fmt.Errorf("security: approval %s not found", id)
	}
	if rec.State == ApprovalApproved {
		return nil, fmt.Errorf("security: approval %s already approved", id)
	}

	rec.State = ApprovalDenied
	rec.Approver = approver
	rec.Reason = reason
	rec.ApprovedAt = nil

	if err := q.persistLocked(); err != nil {
		return nil, err
	}
	q.cond.Broadcast()
	return cloneRecord(rec), nil
}

// ListPending returns outstanding approvals for review.
func (q *ApprovalQueue) ListPending() []*ApprovalRecord {
	q.mu.Lock()
	defer q.mu.Unlock()

	var pending []*ApprovalRecord
	for _, rec := range q.records {
		if rec.State == ApprovalPending {
			pending = append(pending, cloneRecord(rec))
		}
	}
	return pending
}

// IsWhitelisted reports whether the session currently bypasses manual review.
func (q *ApprovalQueue) IsWhitelisted(sessionID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ensureCondLocked()

	expiry, ok := q.whitelist[sessionID]
	if !ok {
		return false
	}
	if expiry.Before(q.clock()) {
		delete(q.whitelist, sessionID)
		if err := q.persistLocked(); err != nil {
			_ = err // best-effort cleanup; in-memory whitelist already expired
		}
		return false
	}
	return true
}

// Wait blocks until the approval is resolved or the context is cancelled.
func (q *ApprovalQueue) Wait(ctx context.Context, id string) (*ApprovalRecord, error) {
	if q == nil {
		return nil, fmt.Errorf("security: approval queue is nil")
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("security: approval id required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	q.mu.Lock()
	q.ensureCondLocked()
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			q.mu.Lock()
			q.ensureCondLocked()
			q.cond.Broadcast()
			q.mu.Unlock()
		case <-done:
		}
	}()
	defer close(done)
	defer q.mu.Unlock()

	for {
		rec, ok := q.records[id]
		if !ok {
			return nil, fmt.Errorf("security: approval %s not found", id)
		}
		if rec.State != ApprovalPending {
			return cloneRecord(rec), nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		q.cond.Wait()
	}
}

func (q *ApprovalQueue) load() error {
	if q.storePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(q.storePath), 0o755); err != nil {
		return fmt.Errorf("security: create approval dir: %w", err)
	}

	data, err := os.ReadFile(q.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("security: load approvals: %w", err)
	}

	var snapshot approvalSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("security: parse approvals: %w", err)
	}

	for _, rec := range snapshot.Records {
		q.records[rec.ID] = rec
	}
	for session, expiry := range snapshot.Whitelist {
		q.whitelist[session] = expiry
	}
	return nil
}

func (q *ApprovalQueue) persistLocked() error {
	if q.storePath == "" {
		return nil
	}
	snapshot := approvalSnapshot{
		Records:   make([]*ApprovalRecord, 0, len(q.records)),
		Whitelist: make(map[string]time.Time, len(q.whitelist)),
	}
	for _, rec := range q.records {
		snapshot.Records = append(snapshot.Records, rec)
	}
	for session, expiry := range q.whitelist {
		snapshot.Whitelist[session] = expiry
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("security: encode approvals: %w", err)
	}

	tmp := q.storePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("security: write approvals: %w", err)
	}
	if err := os.Rename(tmp, q.storePath); err != nil {
		return fmt.Errorf("security: atomically replace approvals: %w", err)
	}
	return nil
}

func (q *ApprovalQueue) ensureCondLocked() {
	if q.cond == nil {
		q.cond = sync.NewCond(&q.mu)
	}
}

type approvalSnapshot struct {
	Records   []*ApprovalRecord    `json:"records"`
	Whitelist map[string]time.Time `json:"whitelist"`
}

func newApprovalID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("failover-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func cloneRecord(rec *ApprovalRecord) *ApprovalRecord {
	if rec == nil {
		return nil
	}
	cp := *rec
	if rec.Paths != nil {
		cp.Paths = append([]string(nil), rec.Paths...)
	}
	return &cp
}
