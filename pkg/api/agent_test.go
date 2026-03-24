package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godeps/agentkit/pkg/agent"
	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/config"
	coreevents "github.com/godeps/agentkit/pkg/core/events"
	corehooks "github.com/godeps/agentkit/pkg/core/hooks"
	"github.com/godeps/agentkit/pkg/message"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/pipeline"
	"github.com/godeps/agentkit/pkg/runtime/commands"
	"github.com/godeps/agentkit/pkg/runtime/checkpoint"
	"github.com/godeps/agentkit/pkg/runtime/skills"
	"github.com/godeps/agentkit/pkg/security"
	"github.com/godeps/agentkit/pkg/tool"
)

func TestRuntimeRequiresModelFactory(t *testing.T) {
	_, err := New(context.Background(), Options{ProjectRoot: t.TempDir()})
	if err == nil {
		t.Fatal("expected model error")
	}
}

func TestRuntimeLoadsSettingsFallback(t *testing.T) {
	opts := Options{ProjectRoot: t.TempDir(), Model: &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}}}}
	rt, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })
	if rt.Settings() == nil {
		t.Fatal("expected fallback settings")
	}
}

func TestRuntimeRunSimple(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "done"}}}}
	rt, err := New(context.Background(), Options{ProjectRoot: root, Model: mdl})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil || resp.Result.Output != "done" {
		t.Fatalf("unexpected result: %+v", resp.Result)
	}
	if rt.Sandbox() == nil {
		t.Fatal("sandbox manager missing")
	}
}

func TestRuntimeInjectsOutputSchemaIntoModelRequest(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "done"}}}}
	outputSchema := &model.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &model.OutputJSONSchema{
			Name: "storyboard",
			Schema: map[string]any{
				"type": "array",
			},
			Strict: true,
		},
	}
	rt, err := New(context.Background(), Options{ProjectRoot: root, Model: mdl, OutputSchema: outputSchema})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	if _, err := rt.Run(context.Background(), Request{Prompt: "hello"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(mdl.requests) == 0 {
		t.Fatal("expected model request")
	}
	if mdl.requests[0].ResponseFormat == nil {
		t.Fatal("expected response format on request")
	}
	if mdl.requests[0].ResponseFormat.JSONSchema == nil || mdl.requests[0].ResponseFormat.JSONSchema.Name != "storyboard" {
		t.Fatalf("unexpected response format %+v", mdl.requests[0].ResponseFormat)
	}
}

func TestRuntimeRequestOutputSchemaOverridesDefault(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "done"}}}}
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       mdl,
		OutputSchema: &model.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &model.OutputJSONSchema{
				Name:   "default_schema",
				Schema: map[string]any{"type": "object"},
			},
		},
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	_, err = rt.Run(context.Background(), Request{
		Prompt: "hello",
		OutputSchema: &model.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &model.OutputJSONSchema{
				Name:   "request_schema",
				Schema: map[string]any{"type": "array"},
			},
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(mdl.requests) == 0 {
		t.Fatal("expected model request")
	}
	if mdl.requests[0].ResponseFormat == nil || mdl.requests[0].ResponseFormat.JSONSchema == nil {
		t.Fatalf("unexpected response format %+v", mdl.requests[0].ResponseFormat)
	}
	if mdl.requests[0].ResponseFormat.JSONSchema.Name != "request_schema" {
		t.Fatalf("ResponseFormat=%+v, want request_schema", mdl.requests[0].ResponseFormat)
	}
}

func TestRuntimeRequestOutputSchemaTextDisablesDefault(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "done"}}}}
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       mdl,
		OutputSchema: &model.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &model.OutputJSONSchema{
				Name:   "default_schema",
				Schema: map[string]any{"type": "object"},
			},
		},
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	_, err = rt.Run(context.Background(), Request{
		Prompt: "hello",
		OutputSchema: &model.ResponseFormat{
			Type: "text",
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(mdl.requests) == 0 {
		t.Fatal("expected model request")
	}
	if mdl.requests[0].ResponseFormat == nil {
		t.Fatal("expected response format on request")
	}
	if mdl.requests[0].ResponseFormat.Type != "text" {
		t.Fatalf("ResponseFormat=%+v, want text", mdl.requests[0].ResponseFormat)
	}
	if mdl.requests[0].ResponseFormat.JSONSchema != nil {
		t.Fatalf("ResponseFormat.JSONSchema=%+v, want nil", mdl.requests[0].ResponseFormat.JSONSchema)
	}
}

func TestRuntimePostProcessOutputSchemaFormatsFinalText(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{
		{
			Message:    model.Message{Role: "assistant", Content: "summary text"},
			Usage:      model.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
			StopReason: "end_turn",
		},
		{
			Message:    model.Message{Role: "assistant", Content: `{"summary":"ok"}`},
			Usage:      model.Usage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7},
			StopReason: "stop",
		},
	}}
	schema := &model.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &model.OutputJSONSchema{
			Name:   "summary",
			Schema: map[string]any{"type": "object"},
		},
	}
	rt, err := New(context.Background(), Options{
		ProjectRoot:      root,
		Model:            mdl,
		OutputSchema:     schema,
		OutputSchemaMode: OutputSchemaModePostProcess,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil || resp.Result.Output != `{"summary":"ok"}` {
		t.Fatalf("unexpected result: %+v", resp.Result)
	}
	if resp.Result.Usage.InputTokens != 4 || resp.Result.Usage.OutputTokens != 6 || resp.Result.Usage.TotalTokens != 10 {
		t.Fatalf("unexpected merged usage: %+v", resp.Result.Usage)
	}
	if resp.Result.StopReason != "stop" {
		t.Fatalf("StopReason=%q, want stop", resp.Result.StopReason)
	}
	if len(mdl.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(mdl.requests))
	}
	if mdl.requests[0].ResponseFormat != nil {
		t.Fatalf("expected loop request without response format, got %+v", mdl.requests[0].ResponseFormat)
	}
	if mdl.requests[1].ResponseFormat == nil || mdl.requests[1].ResponseFormat.JSONSchema == nil || mdl.requests[1].ResponseFormat.JSONSchema.Name != "summary" {
		t.Fatalf("unexpected formatting response format: %+v", mdl.requests[1].ResponseFormat)
	}
	if len(mdl.requests[1].Tools) != 0 {
		t.Fatalf("expected formatting request without tools, got %+v", mdl.requests[1].Tools)
	}
}

func TestRuntimePostProcessOutputSchemaSkipsFormattingForValidJSON(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{
		Message:    model.Message{Role: "assistant", Content: `{"summary":"ok"}`},
		Usage:      model.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
		StopReason: "end_turn",
	}}}
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       mdl,
		OutputSchema: &model.ResponseFormat{
			Type: "json_object",
		},
		OutputSchemaMode: OutputSchemaModePostProcess,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil || resp.Result.Output != `{"summary":"ok"}` {
		t.Fatalf("unexpected result: %+v", resp.Result)
	}
	if len(mdl.requests) != 1 {
		t.Fatalf("expected 1 model request, got %d", len(mdl.requests))
	}
	if mdl.requests[0].ResponseFormat != nil {
		t.Fatalf("expected loop request without response format, got %+v", mdl.requests[0].ResponseFormat)
	}
}

func TestRuntimePostProcessOutputSchemaFallsBackWhenFormattingFails(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{
		responses: []*model.Response{{
			Message:    model.Message{Role: "assistant", Content: "summary text"},
			Usage:      model.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
			StopReason: "end_turn",
		}},
		errSequence: []error{nil, errors.New("formatter unavailable")},
	}
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       mdl,
		OutputSchema: &model.ResponseFormat{
			Type: "json_object",
		},
		OutputSchemaMode: OutputSchemaModePostProcess,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil || resp.Result.Output != "summary text" {
		t.Fatalf("unexpected result: %+v", resp.Result)
	}
	if resp.Result.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected usage after fallback: %+v", resp.Result.Usage)
	}
	if len(mdl.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(mdl.requests))
	}
	if mdl.requests[1].ResponseFormat == nil || mdl.requests[1].ResponseFormat.Type != "json_object" {
		t.Fatalf("unexpected formatting request: %+v", mdl.requests[1].ResponseFormat)
	}
}
func TestRuntimePropagatesModelError(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{err: errors.New("model refused")}
	rt, err := New(context.Background(), Options{ProjectRoot: root, Model: mdl})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, runErr := rt.Run(context.Background(), Request{Prompt: "please help"})
	if !errors.Is(runErr, mdl.err) {
		t.Fatalf("expected model error, got %v", runErr)
	}
	if resp != nil {
		t.Fatalf("expected no response on model error, got %+v", resp)
	}
}

func TestRuntimeToolFlow(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{
		{Message: model.Message{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "1", Name: "echo", Arguments: map[string]any{"text": "hi"}}}}},
		{Message: model.Message{Role: "assistant", Content: "done"}},
	}}

	toolImpl := &echoTool{}
	opts := Options{ProjectRoot: root, Model: mdl, Tools: []tool.Tool{toolImpl}, Sandbox: SandboxOptions{AllowedPaths: []string{root}, Root: root, NetworkAllow: []string{"localhost"}}}
	rt, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{Prompt: "call tool", ToolWhitelist: []string{"echo"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil || resp.Result.Output != "done" {
		t.Fatalf("unexpected output: %+v", resp.Result)
	}
	if len(resp.HookEvents) == 0 {
		t.Fatal("expected hook events")
	}
	if toolImpl.calls == 0 {
		t.Fatal("expected tool execution")
	}
}

func TestRuntimePermissionAskHandlerAllows(t *testing.T) {
	root := newClaudeProjectWithSettings(t, `{"permissions":{"ask":["echo"]},"sandbox":{"enabled":true}}`)
	mdl := &stubModel{responses: []*model.Response{
		{Message: model.Message{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "1", Name: "echo", Arguments: map[string]any{"text": "hi"}}}}},
		{Message: model.Message{Role: "assistant", Content: "done"}},
	}}

	toolImpl := &echoTool{}
	var called int
	opts := Options{
		ProjectRoot: root,
		Model:       mdl,
		Tools:       []tool.Tool{toolImpl},
		PermissionRequestHandler: func(ctx context.Context, req PermissionRequest) (coreevents.PermissionDecisionType, error) {
			called++
			if req.ToolName != "echo" {
				t.Fatalf("unexpected tool name %q", req.ToolName)
			}
			return coreevents.PermissionAllow, nil
		},
	}
	rt, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	if _, err := rt.Run(context.Background(), Request{Prompt: "call tool", ToolWhitelist: []string{"echo"}}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected handler call, got %d", called)
	}
	if toolImpl.calls != 1 {
		t.Fatalf("expected tool execution, got %d", toolImpl.calls)
	}
}

func TestRuntimePermissionAskHandlerDenies(t *testing.T) {
	root := newClaudeProjectWithSettings(t, `{"permissions":{"ask":["echo"]},"sandbox":{"enabled":true}}`)
	mdl := &stubModel{responses: []*model.Response{
		{Message: model.Message{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "1", Name: "echo", Arguments: map[string]any{"text": "hi"}}}}},
		{Message: model.Message{Role: "assistant", Content: "done"}},
	}}

	toolImpl := &echoTool{}
	var called int
	opts := Options{
		ProjectRoot: root,
		Model:       mdl,
		Tools:       []tool.Tool{toolImpl},
		PermissionRequestHandler: func(context.Context, PermissionRequest) (coreevents.PermissionDecisionType, error) {
			called++
			return coreevents.PermissionDeny, nil
		},
	}
	rt, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	if _, err := rt.Run(context.Background(), Request{Prompt: "call tool", ToolWhitelist: []string{"echo"}}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected handler call, got %d", called)
	}
	if toolImpl.calls != 0 {
		t.Fatalf("tool should not execute when denied, got %d", toolImpl.calls)
	}
}

func TestRuntimePermissionAskAutoWhitelist(t *testing.T) {
	root := newClaudeProjectWithSettings(t, `{"permissions":{"ask":["echo"]},"sandbox":{"enabled":true}}`)
	mdl := &stubModel{responses: []*model.Response{
		{Message: model.Message{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "1", Name: "echo", Arguments: map[string]any{"text": "hi"}}}}},
		{Message: model.Message{Role: "assistant", Content: "done"}},
	}}

	queue, err := security.NewApprovalQueue(filepath.Join(t.TempDir(), "approvals.json"))
	if err != nil {
		t.Fatalf("approval queue: %v", err)
	}
	rec, err := queue.Request("sess-1", "echo", nil)
	if err != nil {
		t.Fatalf("queue request: %v", err)
	}
	if _, err := queue.Approve(rec.ID, "tester", time.Hour); err != nil {
		t.Fatalf("queue approve: %v", err)
	}

	toolImpl := &echoTool{}
	opts := Options{
		ProjectRoot:          root,
		Model:                mdl,
		Tools:                []tool.Tool{toolImpl},
		ApprovalQueue:        queue,
		ApprovalWhitelistTTL: time.Hour,
	}
	rt, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	if _, err := rt.Run(context.Background(), Request{Prompt: "call tool", SessionID: "sess-1", ToolWhitelist: []string{"echo"}}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if toolImpl.calls != 1 {
		t.Fatalf("expected tool execution via whitelist, got %d", toolImpl.calls)
	}
}

func TestRuntimeHookAskUsesPermissionHandler(t *testing.T) {
	root := newClaudeProject(t)
	dir := t.TempDir()
	script := writeScript(t, dir, "ask.sh", shScript(
		"#!/bin/sh\nprintf '{\"hookSpecificOutput\":{\"permissionDecision\":\"ask\"}}'\n",
		"@echo {\"hookSpecificOutput\":{\"permissionDecision\":\"ask\"}}\r\n",
	))
	mdl := &stubModel{responses: []*model.Response{
		{Message: model.Message{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "1", Name: "echo", Arguments: map[string]any{"text": "hi"}}}}},
		{Message: model.Message{Role: "assistant", Content: "done"}},
	}}

	toolImpl := &echoTool{}
	var called int
	opts := Options{
		ProjectRoot: root,
		Model:       mdl,
		Tools:       []tool.Tool{toolImpl},
		TypedHooks: []corehooks.ShellHook{{
			Event:   coreevents.PreToolUse,
			Command: script,
		}},
		PermissionRequestHandler: func(context.Context, PermissionRequest) (coreevents.PermissionDecisionType, error) {
			called++
			return coreevents.PermissionAllow, nil
		},
	}
	rt, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	if _, err := rt.Run(context.Background(), Request{Prompt: "call tool", ToolWhitelist: []string{"echo"}}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected handler call, got %d", called)
	}
	if toolImpl.calls != 1 {
		t.Fatalf("expected tool execution, got %d", toolImpl.calls)
	}
}

func TestRuntimeHookAskDeniedByPermissionHandler(t *testing.T) {
	root := newClaudeProject(t)
	dir := t.TempDir()
	script := writeScript(t, dir, "ask.sh", shScript(
		"#!/bin/sh\nprintf '{\"hookSpecificOutput\":{\"permissionDecision\":\"ask\"}}'\n",
		"@echo {\"hookSpecificOutput\":{\"permissionDecision\":\"ask\"}}\r\n",
	))
	mdl := &stubModel{responses: []*model.Response{
		{Message: model.Message{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "1", Name: "echo", Arguments: map[string]any{"text": "hi"}}}}},
		{Message: model.Message{Role: "assistant", Content: "done"}},
	}}

	toolImpl := &echoTool{}
	opts := Options{
		ProjectRoot: root,
		Model:       mdl,
		Tools:       []tool.Tool{toolImpl},
		TypedHooks: []corehooks.ShellHook{{
			Event:   coreevents.PreToolUse,
			Command: script,
		}},
		PermissionRequestHandler: func(context.Context, PermissionRequest) (coreevents.PermissionDecisionType, error) {
			return coreevents.PermissionDeny, nil
		},
	}
	rt, err := New(context.Background(), opts)
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	if _, err := rt.Run(context.Background(), Request{Prompt: "call tool", ToolWhitelist: []string{"echo"}}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if toolImpl.calls != 0 {
		t.Fatalf("tool should not execute when denied, got %d", toolImpl.calls)
	}
}

func TestRuntimeToolExecutor_ErrorHistory(t *testing.T) {
	cases := []struct {
		name   string
		errMsg string
	}{
		{name: "records error output", errMsg: "network unreachable"},
		{name: "escapes quotes for json", errMsg: `input "invalid"`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reg := tool.NewRegistry()
			fail := &failingTool{err: errors.New(tc.errMsg)}
			if err := reg.Register(fail); err != nil {
				t.Fatalf("register tool: %v", err)
			}
			exec := tool.NewExecutor(reg, nil)
			history := message.NewHistory()
			rtExec := &runtimeToolExecutor{
				executor: exec,
				hooks:    &runtimeHookAdapter{},
				history:  history,
				host:     "localhost",
			}

			call := agent.ToolCall{ID: "c1", Name: fail.Name(), Input: map[string]any{"k": "v"}}
			res, err := rtExec.Execute(context.Background(), call, agent.NewContext())
			if err == nil {
				t.Fatal("expected tool execution error")
			}
			if res.Metadata == nil || res.Metadata["error"] != fail.err.Error() {
				t.Fatalf("expected error metadata, got %+v", res.Metadata)
			}

			msgs := history.All()
			if len(msgs) != 1 {
				t.Fatalf("expected history entry, got %d", len(msgs))
			}
			// Result is now stored in ToolCall.Result instead of Message.Content
			if len(msgs[0].ToolCalls) == 0 {
				t.Fatal("expected at least one ToolCall in history")
			}
			var payload map[string]string
			if unmarshalErr := json.Unmarshal([]byte(msgs[0].ToolCalls[0].Result), &payload); unmarshalErr != nil {
				t.Fatalf("history tool result not valid json: %v", unmarshalErr)
			}
			if payload["error"] != fail.err.Error() {
				t.Fatalf("expected error field, got %+v", payload)
			}
			if msgs[0].Role != "tool" || len(msgs[0].ToolCalls) != 1 || msgs[0].ToolCalls[0].Name != call.Name {
				t.Fatalf("tool history entry malformed: %+v", msgs[0])
			}
		})
	}
}

func TestRuntimeToolExecutor_PreToolUseDenialAddsToolResult(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "deny.sh", shScript(
		"#!/bin/sh\nprintf '{\"decision\":\"deny\"}'\n",
		"@echo {\"decision\":\"deny\"}\r\n",
	))

	reg := tool.NewRegistry()
	impl := &echoTool{}
	if err := reg.Register(impl); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := tool.NewExecutor(reg, nil)

	hookExec := corehooks.NewExecutor()
	hookExec.Register(corehooks.ShellHook{
		Event:   coreevents.PreToolUse,
		Command: script,
	})

	history := message.NewHistory()
	rtExec := &runtimeToolExecutor{
		executor: exec,
		hooks:    &runtimeHookAdapter{executor: hookExec},
		history:  history,
		host:     "localhost",
	}

	call := agent.ToolCall{ID: "c1", Name: impl.Name(), Input: map[string]any{"text": "hi"}}
	_, err := rtExec.Execute(context.Background(), call, agent.NewContext())
	if err == nil {
		t.Fatal("expected hook denial error")
	}
	if !errors.Is(err, ErrToolUseDenied) {
		t.Fatalf("expected ErrToolUseDenied, got %v", err)
	}
	if impl.calls != 0 {
		t.Fatalf("expected tool not to execute, got %d calls", impl.calls)
	}

	msgs := history.All()
	if len(msgs) != 1 {
		t.Fatalf("expected history entry, got %d", len(msgs))
	}
	if len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("expected tool history entry, got %+v", msgs[0])
	}
	var payload map[string]string
	if unmarshalErr := json.Unmarshal([]byte(msgs[0].ToolCalls[0].Result), &payload); unmarshalErr != nil {
		t.Fatalf("history tool result not valid json: %v", unmarshalErr)
	}
	if got := payload["error"]; got == "" {
		t.Fatalf("expected error field, got %+v", payload)
	}
}

func TestRuntimeToolExecutor_WhitelistRejectionAddsToolResult(t *testing.T) {
	history := message.NewHistory()
	reg := tool.NewRegistry()
	exec := tool.NewExecutor(reg, nil)
	rtExec := &runtimeToolExecutor{
		executor: exec,
		hooks:    &runtimeHookAdapter{},
		history:  history,
		allow:    map[string]struct{}{"file_read": {}},
		host:     "localhost",
	}

	call := agent.ToolCall{ID: "c1", Name: "glob", Input: map[string]any{"pattern": "*.go"}}
	res, err := rtExec.Execute(context.Background(), call, agent.NewContext())
	if err == nil {
		t.Fatal("expected whitelist error")
	}
	if got, want := err.Error(), "tool glob is not whitelisted"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if res.Name != "" || res.Output != "" || len(res.Metadata) != 0 {
		t.Fatalf("expected zero-value result on early error, got %+v", res)
	}

	msgs := history.All()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(msgs))
	}
	if msgs[0].Role != "tool" || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("expected tool history entry, got %+v", msgs[0])
	}
	if msgs[0].ToolCalls[0].ID != call.ID || msgs[0].ToolCalls[0].Name != call.Name {
		t.Fatalf("unexpected tool call in history: %+v", msgs[0].ToolCalls[0])
	}
	if got, want := msgs[0].ToolCalls[0].Result, "Tool execution failed: tool glob is not whitelisted"; got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRuntimeToolExecutor_UninitializedExecutorAddsToolResult(t *testing.T) {
	history := message.NewHistory()
	rtExec := &runtimeToolExecutor{
		hooks:   &runtimeHookAdapter{},
		history: history,
		host:    "localhost",
	}

	call := agent.ToolCall{ID: "c1", Name: "echo", Input: map[string]any{"text": "hi"}}
	res, err := rtExec.Execute(context.Background(), call, agent.NewContext())
	if err == nil {
		t.Fatal("expected initialization error")
	}
	if got, want := err.Error(), "tool executor not initialised"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if res.Name != "" || res.Output != "" || len(res.Metadata) != 0 {
		t.Fatalf("expected zero-value result on early error, got %+v", res)
	}

	msgs := history.All()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(msgs))
	}
	if msgs[0].Role != "tool" || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("expected tool history entry, got %+v", msgs[0])
	}
	if msgs[0].ToolCalls[0].ID != call.ID || msgs[0].ToolCalls[0].Name != call.Name {
		t.Fatalf("unexpected tool call in history: %+v", msgs[0].ToolCalls[0])
	}
	if got, want := msgs[0].ToolCalls[0].Result, "Tool execution failed: tool executor not initialised"; got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRuntimeToolExecutor_PropagatesOutputRef(t *testing.T) {
	reg := tool.NewRegistry()
	ref := &tool.OutputRef{Path: "/tmp/out", SizeBytes: 123, Truncated: true}
	impl := &outputRefTool{ref: ref}
	if err := reg.Register(impl); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := tool.NewExecutor(reg, nil)
	rtExec := &runtimeToolExecutor{
		executor: exec,
		hooks:    &runtimeHookAdapter{},
		host:     "localhost",
	}

	call := agent.ToolCall{ID: "c1", Name: impl.Name(), Input: map[string]any{}}
	res, err := rtExec.Execute(context.Background(), call, agent.NewContext())
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if res.Output != "ok" {
		t.Fatalf("unexpected output: %q", res.Output)
	}
	got, ok := res.Metadata["output_ref"].(*tool.OutputRef)
	if !ok || got == nil {
		t.Fatalf("expected output_ref metadata, got %+v", res.Metadata)
	}
	if got.Path != ref.Path || got.SizeBytes != ref.SizeBytes || got.Truncated != ref.Truncated {
		t.Fatalf("output_ref mismatch: got=%+v want=%+v", got, ref)
	}
}

func TestRuntimeToolExecutor_AppendsToolContentBlocksToHistory(t *testing.T) {
	reg := tool.NewRegistry()
	impl := &multimodalTool{}
	if err := reg.Register(impl); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := tool.NewExecutor(reg, nil)
	history := message.NewHistory()
	rtExec := &runtimeToolExecutor{
		executor: exec,
		hooks:    &runtimeHookAdapter{},
		history:  history,
		host:     "localhost",
	}

	call := agent.ToolCall{ID: "c1", Name: impl.Name(), Input: map[string]any{}}
	res, err := rtExec.Execute(context.Background(), call, agent.NewContext())
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if res.Output != "image loaded" {
		t.Fatalf("unexpected output: %q", res.Output)
	}

	msgs := history.All()
	if len(msgs) != 2 {
		t.Fatalf("expected tool message plus multimodal follow-up, got %d entries", len(msgs))
	}
	if msgs[0].Role != "tool" || len(msgs[0].ToolCalls) != 1 {
		t.Fatalf("unexpected tool history entry: %+v", msgs[0])
	}
	if msgs[0].ToolCalls[0].Result != "image loaded" {
		t.Fatalf("unexpected tool result in history: %+v", msgs[0].ToolCalls[0])
	}
	if msgs[1].Role != "user" {
		t.Fatalf("expected multimodal follow-up as user message, got %+v", msgs[1])
	}
	if len(msgs[1].ContentBlocks) != 1 {
		t.Fatalf("expected 1 content block, got %+v", msgs[1].ContentBlocks)
	}
	if msgs[1].ContentBlocks[0].Type != message.ContentBlockImage {
		t.Fatalf("unexpected content block type: %+v", msgs[1].ContentBlocks[0])
	}
	if msgs[1].ContentBlocks[0].MediaType != "image/png" || msgs[1].ContentBlocks[0].Data != "aGVsbG8=" {
		t.Fatalf("unexpected content block payload: %+v", msgs[1].ContentBlocks[0])
	}
	if len(msgs[1].Artifacts) != 1 || msgs[1].Artifacts[0].ArtifactID != "art_image" {
		t.Fatalf("expected artifact refs to be appended to history, got %+v", msgs[1].Artifacts)
	}
}

func TestRuntimeToolExecutor_PreservesMultimodalToolResultMetadata(t *testing.T) {
	reg := tool.NewRegistry()
	impl := &multimodalTool{}
	if err := reg.Register(impl); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := tool.NewExecutor(reg, nil)
	rtExec := &runtimeToolExecutor{
		executor: exec,
		hooks:    &runtimeHookAdapter{},
		host:     "localhost",
	}

	call := agent.ToolCall{ID: "c1", Name: impl.Name(), Input: map[string]any{}}
	res, err := rtExec.Execute(context.Background(), call, agent.NewContext())
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}

	if res.Output != "image loaded" {
		t.Fatalf("unexpected output: %q", res.Output)
	}
	if got := res.Metadata["summary"]; got != "generated image preview" {
		t.Fatalf("expected summary metadata, got %+v", got)
	}
	if got := res.Metadata["structured"]; got == nil {
		t.Fatalf("expected structured metadata, got %+v", res.Metadata)
	}
	if got := res.Metadata["preview"]; got == nil {
		t.Fatalf("expected preview metadata, got %+v", res.Metadata)
	}
	arts, ok := res.Metadata["artifacts"].([]artifact.ArtifactRef)
	if !ok || len(arts) != 1 || arts[0].ArtifactID != "art_image" {
		t.Fatalf("expected artifact metadata, got %+v", res.Metadata["artifacts"])
	}
}

func TestNewRejectsDisallowedMCPServer(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}}}
	opts := Options{
		ProjectRoot: root,
		Model:       mdl,
		Sandbox:     SandboxOptions{NetworkAllow: []string{"allowed.example"}},
		MCPServers:  []string{"http://bad.example"},
	}
	if _, err := New(context.Background(), opts); err == nil {
		t.Fatal("expected MCP host guard error")
	}
}

func TestRegisterToolsFiltersDisallowedTools(t *testing.T) {
	reg := tool.NewRegistry()
	allowed := &echoTool{}
	blocked := &failingTool{err: errors.New("boom")}
	opts := Options{
		Tools:           []tool.Tool{allowed, blocked},
		DisallowedTools: []string{"FAIL"},
	}
	if _, err := registerTools(reg, opts, nil, nil, nil); err != nil {
		t.Fatalf("register tools: %v", err)
	}
	if _, err := reg.Get(allowed.Name()); err != nil {
		t.Fatalf("expected allowed tool to register: %v", err)
	}
	if _, err := reg.Get(blocked.Name()); err == nil {
		t.Fatalf("expected blocked tool to be skipped")
	}
}

func TestSettingsLoaderLoadsDisallowedTools(t *testing.T) {
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatalf("claude dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte(`{"disallowedTools":["echo"]}`), 0o600); err != nil {
		t.Fatalf("settings write: %v", err)
	}
	loader := &config.SettingsLoader{ProjectRoot: root}
	settings, err := loader.Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if len(settings.DisallowedTools) != 1 || settings.DisallowedTools[0] != "echo" {
		t.Fatalf("unexpected disallowed tools %+v", settings.DisallowedTools)
	}
}

func TestRuntimeCommandAndSkillIntegration(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}}}

	skill := SkillRegistration{
		Definition: skills.Definition{Name: "tagger", Matchers: []skills.Matcher{skills.KeywordMatcher{Any: []string{"trigger"}}}},
		Handler: skills.HandlerFunc(func(context.Context, skills.ActivationContext) (skills.Result, error) {
			return skills.Result{Output: "skill-prefix", Metadata: map[string]any{"api.tags": map[string]string{"skill": "true"}}}, nil
		}),
	}
	command := CommandRegistration{
		Definition: commands.Definition{Name: "tag"},
		Handler: commands.HandlerFunc(func(context.Context, commands.Invocation) (commands.Result, error) {
			return commands.Result{Metadata: map[string]any{"api.tags": map[string]string{"severity": "info"}}}, nil
		}),
	}

	rt, err := New(context.Background(), Options{ProjectRoot: root, Model: mdl, Skills: []SkillRegistration{skill}, Commands: []CommandRegistration{command}})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{Prompt: "/tag\ntrigger"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Tags["skill"] != "true" || resp.Tags["severity"] != "info" {
		t.Fatalf("tags missing: %+v", resp.Tags)
	}
}

func TestCheckpointPipelineRunInterruptsAtCheckpoint(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{}
	store := checkpoint.NewMemoryStore()
	first := &pipelineStepTool{outputs: map[string]*tool.ToolResult{
		"prepare": {Output: "prepared", Artifacts: []artifact.ArtifactRef{artifact.NewGeneratedRef("art_prepare", artifact.ArtifactKindText)}},
		"review":  {Output: "reviewed", Artifacts: []artifact.ArtifactRef{artifact.NewGeneratedRef("art_review", artifact.ArtifactKindText)}},
	}}
	rt, err := New(context.Background(), Options{
		ProjectRoot:     root,
		Model:           mdl,
		CustomTools:     []tool.Tool{first},
		CheckpointStore: store,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{
		Pipeline: &pipeline.Step{
			Batch: &pipeline.Batch{
				Steps: []pipeline.Step{
					{Name: "prepare", Tool: first.Name()},
					{Checkpoint: &pipeline.Checkpoint{Name: "after-review", Step: pipeline.Step{Name: "review", Tool: first.Name()}}},
					{Name: "finalize", Tool: first.Name()},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if resp.Result == nil || !resp.Result.Interrupted || resp.Result.CheckpointID == "" {
		t.Fatalf("expected interrupted result with checkpoint id, got %+v", resp.Result)
	}
	if resp.Result.Output != "reviewed" {
		t.Fatalf("expected checkpoint step output to be surfaced, got %+v", resp.Result)
	}
	if fmt.Sprint(first.calls) != "[prepare review]" {
		t.Fatalf("expected execution to stop after checkpoint, got %v", first.calls)
	}
}

func TestResumePipelineRunContinuesRemainingStepsOnly(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{}
	store := checkpoint.NewMemoryStore()
	first := &pipelineStepTool{outputs: map[string]*tool.ToolResult{
		"prepare":  {Output: "prepared", Artifacts: []artifact.ArtifactRef{artifact.NewGeneratedRef("art_prepare", artifact.ArtifactKindText)}},
		"review":   {Output: "reviewed", Artifacts: []artifact.ArtifactRef{artifact.NewGeneratedRef("art_review", artifact.ArtifactKindText)}},
		"finalize": {Output: "done", Artifacts: []artifact.ArtifactRef{artifact.NewGeneratedRef("art_done", artifact.ArtifactKindDocument)}},
	}}
	rt, err := New(context.Background(), Options{
		ProjectRoot:     root,
		Model:           mdl,
		CustomTools:     []tool.Tool{first},
		CheckpointStore: store,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	firstResp, err := rt.Run(context.Background(), Request{
		SessionID: "pipeline-session",
		Pipeline: &pipeline.Step{
			Batch: &pipeline.Batch{
				Steps: []pipeline.Step{
					{Name: "prepare", Tool: first.Name()},
					{Checkpoint: &pipeline.Checkpoint{Name: "after-review", Step: pipeline.Step{Name: "review", Tool: first.Name()}}},
					{Name: "finalize", Tool: first.Name()},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}

	resumed, err := rt.Run(context.Background(), Request{
		SessionID:             "pipeline-session",
		ResumeFromCheckpoint:  firstResp.Result.CheckpointID,
	})
	if err != nil {
		t.Fatalf("resume pipeline: %v", err)
	}
	if resumed.Result == nil || resumed.Result.Output != "done" || resumed.Result.Interrupted {
		t.Fatalf("expected resumed completion, got %+v", resumed.Result)
	}
	if fmt.Sprint(first.calls) != "[prepare review finalize]" {
		t.Fatalf("expected only remaining step after resume, got %v", first.calls)
	}
}

func TestInterruptResultCarriesCheckpointIdentifier(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{}
	store := checkpoint.NewMemoryStore()
	first := &pipelineStepTool{outputs: map[string]*tool.ToolResult{
		"human-review": {Output: "awaiting approval"},
	}}
	rt, err := New(context.Background(), Options{
		ProjectRoot:     root,
		Model:           mdl,
		CustomTools:     []tool.Tool{first},
		CheckpointStore: store,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{
		Pipeline: &pipeline.Step{
			Checkpoint: &pipeline.Checkpoint{
				Name: "await-approval",
				Step: pipeline.Step{Name: "human-review", Tool: first.Name()},
			},
		},
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if resp.Result == nil || !resp.Result.Interrupted || resp.Result.StopReason != "interrupted" || resp.Result.CheckpointID == "" {
		t.Fatalf("expected interrupt payload with checkpoint identifier, got %+v", resp.Result)
	}
}

func newClaudeProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatalf("claude dir: %v", err)
	}
	settings := []byte(`{"model":"claude-3-opus"}`)
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), settings, 0o600); err != nil {
		t.Fatalf("settings: %v", err)
	}
	return root
}

func newClaudeProjectWithSettings(t *testing.T, raw string) string {
	t.Helper()
	root := t.TempDir()
	claude := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatalf("claude dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte(raw), 0o600); err != nil {
		t.Fatalf("settings: %v", err)
	}
	return root
}

func TestRuntimeCacheConfigPriority(t *testing.T) {
	root := newClaudeProject(t)

	tests := []struct {
		name               string
		defaultEnableCache bool
		reqEnableCache     *bool
		wantCache          bool
	}{
		{
			name:               "global default enabled, request not set",
			defaultEnableCache: true,
			reqEnableCache:     nil,
			wantCache:          true,
		},
		{
			name:               "global default disabled, request not set",
			defaultEnableCache: false,
			reqEnableCache:     nil,
			wantCache:          false,
		},
		{
			name:               "request overrides global (enable)",
			defaultEnableCache: false,
			reqEnableCache:     boolPtr(true),
			wantCache:          true,
		},
		{
			name:               "request overrides global (disable)",
			defaultEnableCache: true,
			reqEnableCache:     boolPtr(false),
			wantCache:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "done"}}}}
			rt, err := New(context.Background(), Options{
				ProjectRoot:        root,
				Model:              mdl,
				DefaultEnableCache: tt.defaultEnableCache,
			})
			if err != nil {
				t.Fatalf("runtime: %v", err)
			}
			t.Cleanup(func() { _ = rt.Close() })

			req := Request{
				Prompt:            "test",
				EnablePromptCache: tt.reqEnableCache,
			}

			_, err = rt.Run(context.Background(), req)
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			// Verify model request had correct cache setting
			if len(mdl.requests) == 0 {
				t.Fatal("expected model request")
			}
			got := mdl.requests[0].EnablePromptCache
			if got != tt.wantCache {
				t.Errorf("EnablePromptCache = %v, want %v", got, tt.wantCache)
			}
		})
	}
}

type stubModel struct {
	responses   []*model.Response
	requests    []model.Request
	errSequence []error
	idx         int
	err         error
}

func (s *stubModel) Complete(_ context.Context, req model.Request) (*model.Response, error) {
	s.requests = append(s.requests, req)
	if len(s.errSequence) > 0 {
		i := len(s.requests) - 1
		if i < len(s.errSequence) && s.errSequence[i] != nil {
			return nil, s.errSequence[i]
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	if len(s.responses) == 0 {
		return &model.Response{Message: model.Message{Role: "assistant"}}, nil
	}
	if s.idx >= len(s.responses) {
		return s.responses[len(s.responses)-1], nil
	}
	resp := s.responses[s.idx]
	s.idx++
	return resp, nil
}

func (s *stubModel) CompleteStream(_ context.Context, req model.Request, cb model.StreamHandler) error {
	resp, err := s.Complete(context.Background(), req)
	if err != nil {
		return err
	}
	return cb(model.StreamResult{Final: true, Response: resp})
}

type echoTool struct {
	calls int
}

func (e *echoTool) Name() string             { return "echo" }
func (e *echoTool) Description() string      { return "echo text" }
func (e *echoTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (e *echoTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	e.calls++
	text := params["text"]
	return &tool.ToolResult{Output: fmt.Sprint(text)}, nil
}

type outputRefTool struct {
	ref *tool.OutputRef
}

func (o *outputRefTool) Name() string             { return "output_ref" }
func (o *outputRefTool) Description() string      { return "returns tool output ref" }
func (o *outputRefTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (o *outputRefTool) Execute(context.Context, map[string]interface{}) (*tool.ToolResult, error) {
	return &tool.ToolResult{Success: true, Output: "ok", OutputRef: o.ref}, nil
}

type failingTool struct {
	err error
}

func (f *failingTool) Name() string             { return "fail" }
func (f *failingTool) Description() string      { return "always fails" }
func (f *failingTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (f *failingTool) Execute(context.Context, map[string]interface{}) (*tool.ToolResult, error) {
	return nil, f.err
}

type multimodalTool struct{}

func (m *multimodalTool) Name() string             { return "image_read" }
func (m *multimodalTool) Description() string      { return "returns image blocks" }
func (m *multimodalTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (m *multimodalTool) Execute(context.Context, map[string]interface{}) (*tool.ToolResult, error) {
	return &tool.ToolResult{
		Success: true,
		Output:  "image loaded",
		Summary: "generated image preview",
		Structured: map[string]any{
			"dominant_color": "blue",
		},
		Artifacts: []artifact.ArtifactRef{
			artifact.NewGeneratedRef("art_image", artifact.ArtifactKindImage),
		},
		ContentBlocks: []model.ContentBlock{{
			Type:      model.ContentBlockImage,
			MediaType: "image/png",
			Data:      "aGVsbG8=",
		}},
		Preview: &tool.Preview{
			Title:     "Preview image",
			Summary:   "generated image preview",
			MediaType: "image/png",
		},
	}, nil
}

type pipelineStepTool struct {
	calls   []string
	outputs map[string]*tool.ToolResult
}

func (p *pipelineStepTool) Name() string             { return "pipeline_step" }
func (p *pipelineStepTool) Description() string      { return "returns outputs based on pipeline step name" }
func (p *pipelineStepTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (p *pipelineStepTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	name, _ := params["step"].(string)
	p.calls = append(p.calls, name)
	if res, ok := p.outputs[name]; ok {
		return res, nil
	}
	return &tool.ToolResult{Output: name}, nil
}
