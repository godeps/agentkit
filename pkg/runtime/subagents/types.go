package subagents

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"

	"github.com/godeps/agentkit/pkg/runtime/skills"
)

var (
	ErrUnknownInstance = errors.New("subagents: unknown instance")
	ErrInstanceExists  = errors.New("subagents: instance already exists")
	ErrExecutorClosed  = errors.New("subagents: executor runner is not configured")
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Instance struct {
	ID              string
	Profile         string
	ParentSessionID string
	SessionID       string
	Status          Status
	CreatedAt       time.Time
	StartedAt       *time.Time
	FinishedAt      *time.Time
	Result          *Result
	Error           string
	Metadata        map[string]any
}

func (i Instance) clone() Instance {
	if len(i.Metadata) > 0 {
		i.Metadata = maps.Clone(i.Metadata)
	}
	if i.Result != nil {
		res := i.Result.clone()
		i.Result = &res
	}
	if i.StartedAt != nil {
		started := *i.StartedAt
		i.StartedAt = &started
	}
	if i.FinishedAt != nil {
		finished := *i.FinishedAt
		i.FinishedAt = &finished
	}
	return i
}

type SpawnRequest struct {
	Target        string
	Instruction   string
	Activation    skills.ActivationContext
	ToolWhitelist []string
	Metadata      map[string]any
	ParentContext Context
}

type SpawnHandle struct {
	ID string
}

type WaitRequest struct {
	ID      string
	Timeout time.Duration
}

type WaitResult struct {
	Instance Instance
	TimedOut bool
}

type RunRequest struct {
	InstanceID    string
	Target        string
	Instruction   string
	Activation    skills.ActivationContext
	ToolWhitelist []string
	Metadata      map[string]any
	ParentContext Context
}

type Runner interface {
	RunSubagent(context.Context, RunRequest) (Result, error)
}

func childSessionID(parent Context, instanceID string) string {
	parentID := strings.TrimSpace(parent.SessionID)
	if parentID == "" {
		return instanceID
	}
	return parentID + ":" + instanceID
}
