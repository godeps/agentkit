package subagents

import (
	"context"
	"testing"
	"time"
)

type stubRunner struct {
	result Result
	err    error
}

func (s stubRunner) RunSubagent(context.Context, RunRequest) (Result, error) {
	return s.result, s.err
}

func TestExecutorSpawnAndWaitCompletesInstance(t *testing.T) {
	profiles := NewManager()
	if err := profiles.Register(Definition{Name: "plan"}, HandlerFunc(func(context.Context, Context, Request) (Result, error) {
		return Result{}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}
	exec := NewExecutor(profiles, NewMemoryStore(), stubRunner{
		result: Result{Output: "done"},
	})

	handle, err := exec.Spawn(WithTaskDispatch(context.Background()), SpawnRequest{
		Target:        "plan",
		Instruction:   "outline this",
		ParentContext: Context{SessionID: "parent"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if handle.ID == "" {
		t.Fatal("expected instance id")
	}

	waited, err := exec.Wait(context.Background(), WaitRequest{ID: handle.ID, Timeout: time.Second})
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if waited.TimedOut {
		t.Fatal("expected completed wait")
	}
	if waited.Instance.Status != StatusCompleted {
		t.Fatalf("expected completed instance, got %+v", waited.Instance)
	}
	if waited.Instance.Result == nil || waited.Instance.Result.Output != "done" {
		t.Fatalf("expected result output, got %+v", waited.Instance.Result)
	}
}
