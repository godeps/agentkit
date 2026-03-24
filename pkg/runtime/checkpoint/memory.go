package checkpoint

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore keeps checkpoint state in memory for the lifetime of the runtime.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: map[string]Entry{}}
}

func (m *MemoryStore) Save(_ context.Context, entry Entry) (string, error) {
	if m == nil {
		m = NewMemoryStore()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id := entry.ID
	if id == "" {
		id = uuid.NewString()
	}
	entry.ID = id
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	m.entries[id] = cloneEntry(entry)
	return id, nil
}

func (m *MemoryStore) Load(_ context.Context, id string) (Entry, error) {
	if m == nil {
		return Entry{}, ErrNotFound
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.entries[id]
	if !ok {
		return Entry{}, ErrNotFound
	}
	return cloneEntry(entry), nil
}

func (m *MemoryStore) Delete(_ context.Context, id string) error {
	if m == nil {
		return ErrNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[id]; !ok {
		return ErrNotFound
	}
	delete(m.entries, id)
	return nil
}

func cloneEntry(entry Entry) Entry {
	cloned := entry
	if entry.Remaining != nil {
		step := *entry.Remaining
		cloned.Remaining = &step
	}
	cloned.Input = entry.Input
	cloned.Result = entry.Result
	return cloned
}
