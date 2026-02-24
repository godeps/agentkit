package subagents_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/runtime/skills"
	"github.com/godeps/agentkit/pkg/runtime/subagents"
	"github.com/godeps/agentkit/pkg/tool"
	toolbuiltin "github.com/godeps/agentkit/pkg/tool/builtin"
)

func TestTaskIntegration_NoTaskTool_NoDispatch(t *testing.T) {
	var calls atomic.Int32
	handler := subagents.HandlerFunc(func(context.Context, subagents.Context, subagents.Request) (subagents.Result, error) {
		calls.Add(1)
		return subagents.Result{Output: "should-not-run"}, nil
	})

	rt := newRuntimeWithSubagent(t, handler, subagents.TypeGeneralPurpose, &scriptedModel{
		responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}},
	})

	_, err := rt.Run(context.Background(), api.Request{
		Prompt:         "regular prompt",
		TargetSubagent: subagents.TypeGeneralPurpose,
	})
	if err != nil {
		t.Fatalf("runtime run failed: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("expected subagent to stay idle without Task tool, got %d calls", calls.Load())
	}
}

func TestTaskIntegration_TaskToolDispatchesViaAgent(t *testing.T) {
	var (
		calls    atomic.Int32
		recordMu sync.Mutex
		record   struct {
			req  subagents.Request
			ctx  subagents.Context
			meta subagents.Context
		}
	)

	handler := subagents.HandlerFunc(func(ctx context.Context, subCtx subagents.Context, req subagents.Request) (subagents.Result, error) {
		calls.Add(1)
		recordMu.Lock()
		defer recordMu.Unlock()
		record.req = req
		record.ctx = subCtx
		if meta, ok := subagents.FromContext(ctx); ok {
			record.meta = meta
		}
		return subagents.Result{Output: "subagent-output"}, nil
	})

	taskArgs := map[string]any{
		"description":   "explore repo quickly",
		"prompt":        "scan workspace",
		"subagent_type": subagents.TypeExplore,
		"model":         "haiku",
		"resume":        "session-7",
	}

	rt := newRuntimeWithSubagent(t, handler, subagents.TypeExplore, &scriptedModel{
		responses: []*model.Response{
			taskToolCall("call-1", taskArgs),
			{Message: model.Message{Role: "assistant", Content: "done"}},
		},
	})

	_, err := rt.Run(context.Background(), api.Request{Prompt: "kick off task"})
	if err != nil {
		t.Fatalf("runtime run failed: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected one subagent dispatch, got %d", calls.Load())
	}

	recordMu.Lock()
	defer recordMu.Unlock()
	if record.req.Target != subagents.TypeExplore {
		t.Fatalf("expected target %s, got %s", subagents.TypeExplore, record.req.Target)
	}
	if record.req.Instruction != "scan workspace" {
		t.Fatalf("expected instruction from prompt, got %q", record.req.Instruction)
	}
	if desc, ok := record.req.Metadata["task.description"].(string); !ok || desc != "explore repo quickly" {
		t.Fatalf("expected description metadata, got %+v", record.req.Metadata)
	}
	if entry, ok := record.req.Metadata["entrypoint"].(api.EntryPoint); !ok || entry != api.EntryPointCLI {
		t.Fatalf("expected entrypoint metadata CLI, got %+v", record.req.Metadata["entrypoint"])
	}
	if record.meta.SessionID != "session-7" {
		t.Fatalf("expected resume session propagated, got %q", record.meta.SessionID)
	}
	if !record.meta.Allows("glob") {
		t.Fatalf("explore context should preserve its tool whitelist")
	}
	if model := strings.TrimSpace(record.meta.Model); model == "" || !strings.Contains(model, "haiku") {
		t.Fatalf("expected task model mapping applied, got %q", record.meta.Model)
	}
}

func TestTaskIntegration_ErrorsAndCancellation(t *testing.T) {
	t.Run("error propagation", func(t *testing.T) {
		subMgr := subagents.NewManager()
		if err := subMgr.Register(subagents.Definition{Name: subagents.TypeGeneralPurpose}, subagents.HandlerFunc(func(context.Context, subagents.Context, subagents.Request) (subagents.Result, error) {
			return subagents.Result{}, errors.New("boom")
		})); err != nil {
			t.Fatalf("register: %v", err)
		}
		task := toolbuiltin.NewTaskTool()
		task.SetRunner(taskRunnerThroughManager(subMgr))

		_, err := task.Execute(context.Background(), map[string]any{
			"description":   "propagate task error",
			"prompt":        "fail now",
			"subagent_type": subagents.TypeGeneralPurpose,
		})
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected boom error, got %v", err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		subMgr := subagents.NewManager()
		started := make(chan struct{})
		if err := subMgr.Register(subagents.Definition{Name: subagents.TypePlan}, subagents.HandlerFunc(func(ctx context.Context, subCtx subagents.Context, req subagents.Request) (subagents.Result, error) {
			close(started)
			<-ctx.Done()
			return subagents.Result{}, ctx.Err()
		})); err != nil {
			t.Fatalf("register: %v", err)
		}
		task := toolbuiltin.NewTaskTool()
		task.SetRunner(taskRunnerThroughManager(subMgr))

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-started
			cancel()
		}()

		_, err := task.Execute(ctx, map[string]any{
			"description":   "cancel active task",
			"prompt":        "wait for cancel",
			"subagent_type": subagents.TypePlan,
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	})
}

func TestTaskIntegration_ConcurrentIsolation(t *testing.T) {
	subMgr := subagents.NewManager()
	def, _ := subagents.BuiltinDefinition(subagents.TypeGeneralPurpose)
	var (
		mu    sync.Mutex
		seen  = map[string]string{}
		delay = 20 * time.Millisecond
	)
	if err := subMgr.Register(def, subagents.HandlerFunc(func(ctx context.Context, subCtx subagents.Context, req subagents.Request) (subagents.Result, error) {
		time.Sleep(delay)
		mu.Lock()
		seen[req.Instruction] = subCtx.SessionID
		mu.Unlock()
		return subagents.Result{Output: req.Instruction}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	task := toolbuiltin.NewTaskTool()
	task.SetRunner(taskRunnerThroughManager(subMgr))

	const workers = 6
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			prompt := fmt.Sprintf("worker-%d", i)
			resume := fmt.Sprintf("session-%d", i)
			_, err := task.Execute(context.Background(), map[string]any{
				"description":   "concurrent task run",
				"prompt":        prompt,
				"subagent_type": subagents.TypeGeneralPurpose,
				"resume":        resume,
			})
			if err != nil {
				t.Errorf("task %d failed: %v", i, err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != workers {
		t.Fatalf("expected %d isolated dispatches, got %d", workers, len(seen))
	}
	for i := 0; i < workers; i++ {
		prompt := fmt.Sprintf("worker-%d", i)
		expectSession := fmt.Sprintf("session-%d", i)
		if seen[prompt] != expectSession {
			t.Fatalf("expected %s to keep session %s, got %s", prompt, expectSession, seen[prompt])
		}
	}
}

func newRuntimeWithSubagent(t *testing.T, handler subagents.Handler, target string, mdl model.Model) *api.Runtime {
	t.Helper()
	def, ok := subagents.BuiltinDefinition(target)
	if !ok {
		t.Fatalf("missing builtin definition for %s", target)
	}
	rt, err := api.New(context.Background(), api.Options{
		ProjectRoot: t.TempDir(),
		EntryPoint:  api.EntryPointCLI,
		Model:       mdl,
		Subagents: []api.SubagentRegistration{{
			Definition: def,
			Handler:    handler,
		}},
	})
	if err != nil {
		t.Fatalf("runtime init: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

type scriptedModel struct {
	mu        sync.Mutex
	responses []*model.Response
	err       error
}

func (s *scriptedModel) Complete(context.Context, model.Request) (*model.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	if len(s.responses) == 0 {
		return nil, errors.New("no scripted responses")
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func (s *scriptedModel) CompleteStream(ctx context.Context, req model.Request, cb model.StreamHandler) error {
	resp, err := s.Complete(ctx, req)
	if err != nil {
		return err
	}
	if cb != nil {
		return cb(model.StreamResult{Final: true, Response: resp})
	}
	return nil
}

func taskToolCall(id string, args map[string]any) *model.Response {
	return &model.Response{
		Message: model.Message{
			Role: "assistant",
			ToolCalls: []model.ToolCall{{
				ID:        id,
				Name:      "Task",
				Arguments: args,
			}},
		},
	}
}

func taskRunnerThroughManager(m *subagents.Manager) toolbuiltin.TaskRunner {
	return func(ctx context.Context, req toolbuiltin.TaskRequest) (*tool.ToolResult, error) {
		meta := map[string]any{
			"entrypoint": api.EntryPointCLI,
		}
		if strings.TrimSpace(req.Description) != "" {
			meta["task.description"] = strings.TrimSpace(req.Description)
		}
		if resume := strings.TrimSpace(req.Resume); resume != "" {
			meta["session_id"] = resume
		}
		if req.Model != "" {
			meta["task.model"] = req.Model
		}
		instruction := strings.TrimSpace(req.Prompt)
		res, err := m.Dispatch(subagents.WithTaskDispatch(ctx), subagents.Request{
			Target:      req.SubagentType,
			Instruction: instruction,
			Metadata:    meta,
			Activation:  skills.ActivationContext{Prompt: instruction},
		})
		if err != nil {
			return nil, err
		}
		return &tool.ToolResult{
			Success: res.Error == "",
			Output:  fmt.Sprint(res.Output),
			Data: map[string]any{
				"subagent": res.Subagent,
				"metadata": res.Metadata,
			},
		}, nil
	}
}
