package subagents

import (
	"context"
	"fmt"
	"maps"
	"sync/atomic"
	"time"
)

type Executor struct {
	profiles *Manager
	store    Store
	runner   Runner
	now      func() time.Time
	seq      atomic.Uint64
}

func NewExecutor(profiles *Manager, store Store, runner Runner) *Executor {
	if store == nil {
		store = NewMemoryStore()
	}
	return &Executor{
		profiles: profiles,
		store:    store,
		runner:   runner,
		now:      time.Now,
	}
}

func (e *Executor) Store() Store {
	return e.store
}

func (e *Executor) Spawn(ctx context.Context, req SpawnRequest) (SpawnHandle, error) {
	if e == nil || e.profiles == nil {
		return SpawnHandle{}, ErrUnknownSubagent
	}
	if e.runner == nil {
		return SpawnHandle{}, ErrExecutorClosed
	}
	if dispatchSource(ctx) != DispatchSourceTaskTool {
		return SpawnHandle{}, ErrDispatchUnauthorized
	}
	instruction := req.Instruction
	target, err := e.profiles.selectTarget(Request{
		Target:      req.Target,
		Instruction: req.Instruction,
		Activation:  req.Activation,
	})
	if err != nil {
		return SpawnHandle{}, err
	}
	now := e.now()
	id := fmt.Sprintf("subagent-%d", e.seq.Add(1))
	inst := Instance{
		ID:              id,
		Profile:         target.definition.Name,
		ParentSessionID: req.ParentContext.SessionID,
		SessionID:       childSessionID(req.ParentContext, id),
		Status:          StatusQueued,
		CreatedAt:       now,
		Metadata:        cloneMetadata(req.Metadata),
	}
	if err := e.store.Create(inst); err != nil {
		return SpawnHandle{}, err
	}

	go e.run(id, RunRequest{
		InstanceID:    id,
		Target:        req.Target,
		Instruction:   instruction,
		Activation:    req.Activation,
		ToolWhitelist: append([]string(nil), req.ToolWhitelist...),
		Metadata:      cloneMetadata(req.Metadata),
		ParentContext: req.ParentContext.Clone(),
	})

	return SpawnHandle{ID: id}, nil
}

func cloneMetadata(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	return maps.Clone(meta)
}

func (e *Executor) run(id string, req RunRequest) {
	now := e.now()
	_ = e.store.Update(id, func(inst *Instance) error {
		inst.Status = StatusRunning
		inst.StartedAt = &now
		return nil
	})

	res, err := e.runner.RunSubagent(WithTaskDispatch(context.Background()), req)
	finished := e.now()
	_ = e.store.Update(id, func(inst *Instance) error {
		inst.FinishedAt = &finished
		if err != nil {
			inst.Status = StatusFailed
			inst.Error = err.Error()
			if len(res.Metadata) == 0 {
				res.Metadata = map[string]any{}
			}
			res.Metadata["subagent_id"] = id
			res.Error = err.Error()
			inst.Result = &res
			return nil
		}
		inst.Status = StatusCompleted
		if len(res.Metadata) == 0 {
			res.Metadata = map[string]any{}
		}
		res.Metadata["subagent_id"] = id
		inst.Result = &res
		return nil
	})
}

func (e *Executor) Wait(ctx context.Context, req WaitRequest) (WaitResult, error) {
	if e == nil || e.store == nil {
		return WaitResult{}, ErrUnknownInstance
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		inst, ok := e.store.Get(req.ID)
		if !ok {
			return WaitResult{}, ErrUnknownInstance
		}
		switch inst.Status {
		case StatusCompleted, StatusFailed, StatusCancelled:
			return WaitResult{Instance: inst}, nil
		}
		select {
		case <-ctx.Done():
			return WaitResult{}, ctx.Err()
		case <-deadline.C:
			inst, _ := e.store.Get(req.ID)
			return WaitResult{Instance: inst, TimedOut: true}, nil
		case <-ticker.C:
		}
	}
}

func (e *Executor) Get(ctx context.Context, id string) (Instance, error) {
	if err := ctx.Err(); err != nil {
		return Instance{}, err
	}
	inst, ok := e.store.Get(id)
	if !ok {
		return Instance{}, ErrUnknownInstance
	}
	return inst, nil
}
