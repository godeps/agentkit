package api

import (
	"context"
	"testing"

	"github.com/godeps/agentkit/pkg/agent"
	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/pipeline"
	"github.com/godeps/agentkit/pkg/tool"
)

func TestResultOutputRemainsFinalTextAnswer(t *testing.T) {
	got := convertRunResult(runResult{
		output: &agent.ModelOutput{Content: "final answer"},
		usage:  model.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
		reason: "stop",
	})
	if got == nil {
		t.Fatal("expected result")
	}
	if got.Output != "final answer" {
		t.Fatalf("expected text output to remain in Output, got %+v", got)
	}
	if got.Structured != nil {
		t.Fatalf("expected Structured to stay empty for text run, got %+v", got.Structured)
	}
	if len(got.Artifacts) != 0 {
		t.Fatalf("expected Artifacts to stay empty for text run, got %+v", got.Artifacts)
	}
}

func TestResultStructuredAndArtifactsAreSeparatedFromOutput(t *testing.T) {
	root := newClaudeProject(t)
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       &stubModel{},
		CustomTools: []tool.Tool{&pipelineStepTool{outputs: map[string]*tool.ToolResult{
			"finalize": {
				Output:     "final answer",
				Structured: map[string]any{"status": "ok"},
				Artifacts:  []artifact.ArtifactRef{artifact.NewGeneratedRef("art_done", artifact.ArtifactKindDocument)},
			},
		}}},
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{
		Pipeline: &pipeline.Step{Name: "finalize", Tool: "pipeline_step"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
	if resp.Result.Output != "final answer" {
		t.Fatalf("expected Output to remain final text, got %+v", resp.Result)
	}
	structured, ok := resp.Result.Structured.(map[string]any)
	if !ok || structured["status"] != "ok" {
		t.Fatalf("expected Structured payload to be carried separately, got %+v", resp.Result.Structured)
	}
	if len(resp.Result.Artifacts) != 1 || resp.Result.Artifacts[0].ArtifactID != "art_done" {
		t.Fatalf("expected Artifacts to be carried separately, got %+v", resp.Result.Artifacts)
	}
}
