package api

import (
	"context"
	"testing"

	"github.com/godeps/agentkit/pkg/agent"
	"github.com/godeps/agentkit/pkg/message"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/orchestration"
)

func TestPolicyModelRequestCanAlterFields(t *testing.T) {
	stub := &stubModel{responses: []*model.Response{{
		Message: model.Message{Role: "assistant", Content: "ok"},
	}}}
	conv := &conversationModel{
		base:    stub,
		history: message.NewHistory(),
		prompt:  "hello",
		modelRequestPolicy: ModelRequestPolicyFunc(func(_ context.Context, input ModelRequestPolicyInput) (model.Request, error) {
			req := input.Request
			req.System = "policy-system"
			req.MaxTokens = 123
			return req, nil
		}),
		hooks: &runtimeHookAdapter{},
	}

	if _, err := conv.Generate(context.Background(), &agent.Context{}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(stub.requests) != 1 {
		t.Fatalf("expected 1 model request, got %d", len(stub.requests))
	}
	if stub.requests[0].System != "policy-system" || stub.requests[0].MaxTokens != 123 {
		t.Fatalf("unexpected model request: %+v", stub.requests[0])
	}
}

func TestPolicyToolSelectionFiltersToolsPerIteration(t *testing.T) {
	stub := &stubModel{responses: []*model.Response{{
		Message: model.Message{Role: "assistant", Content: "ok"},
	}}}
	conv := &conversationModel{
		base:    stub,
		history: message.NewHistory(),
		prompt:  "hello",
		tools: []model.ToolDefinition{
			{Name: "echo"},
			{Name: "grep"},
		},
		toolSelectionPolicy: ToolSelectionPolicyFunc(func(_ context.Context, input ToolSelectionPolicyInput) ([]model.ToolDefinition, error) {
			return []model.ToolDefinition{input.Tools[1]}, nil
		}),
		hooks: &runtimeHookAdapter{},
	}

	if _, err := conv.Generate(context.Background(), &agent.Context{Iteration: 2}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(stub.requests) != 1 || len(stub.requests[0].Tools) != 1 || stub.requests[0].Tools[0].Name != "grep" {
		t.Fatalf("unexpected tool selection: %+v", stub.requests)
	}
}

func TestPolicyIterationCanStopRun(t *testing.T) {
	root := newClaudeProject(t)
	stub := &stubModel{responses: []*model.Response{{
		Message: model.Message{Role: "assistant", Content: "should not run"},
	}}}
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       stub,
		IterationPolicy: IterationPolicyFunc(func(_ context.Context, input IterationPolicyInput) (IterationDecision, error) {
			return IterationDecision{Continue: false, StopReason: "policy_stop", Output: "stopped"}, nil
		}),
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	defer rt.Close()

	resp, err := rt.Run(context.Background(), Request{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result.StopReason != "policy_stop" || resp.Result.Output != "stopped" {
		t.Fatalf("unexpected policy stop response: %+v", resp.Result)
	}
	if len(stub.requests) != 0 {
		t.Fatalf("expected model call to be skipped, got %d requests", len(stub.requests))
	}
}

func TestPolicyOutputCanAugmentMetadata(t *testing.T) {
	root := newClaudeProject(t)
	stub := &stubModel{responses: []*model.Response{{
		Message: model.Message{Role: "assistant", Content: "done"},
	}}}
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       stub,
		OutputPolicy: OutputPolicyFunc(func(_ context.Context, input OutputPolicyInput) (*orchestration.ResultEnvelope, error) {
			env := input.Envelope
			if env.Metadata == nil {
				env.Metadata = map[string]any{}
			}
			env.Metadata["policy"] = "applied"
			return env, nil
		}),
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	defer rt.Close()

	resp, err := rt.Run(context.Background(), Request{Prompt: "hello"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result.Envelope == nil || resp.Result.Envelope.Metadata["policy"] != "applied" {
		t.Fatalf("expected output metadata to be injected, got %+v", resp.Result.Envelope)
	}
}
