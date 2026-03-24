package pipeline

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/tool"
)

func TestExecutorSequentialStepExecution(t *testing.T) {
	var calls []string
	exec := Executor{
		RunTool: func(ctx context.Context, step Step, input []artifact.ArtifactRef) (*tool.ToolResult, error) {
			calls = append(calls, step.Name)
			switch step.Name {
			case "extract":
				return &tool.ToolResult{
					Output: "extracted",
					Artifacts: []artifact.ArtifactRef{
						artifact.NewGeneratedRef("art_extract", artifact.ArtifactKindText),
					},
				}, nil
			case "summarize":
				if len(input) != 1 || input[0].ArtifactID != "art_extract" {
					t.Fatalf("expected summarize step to receive previous artifacts, got %+v", input)
				}
				return &tool.ToolResult{
					Output: "summary complete",
					Artifacts: []artifact.ArtifactRef{
						artifact.NewGeneratedRef("art_summary", artifact.ArtifactKindDocument),
					},
				}, nil
			default:
				return nil, fmt.Errorf("unexpected step %q", step.Name)
			}
		},
	}

	result, err := exec.Execute(context.Background(), Step{
		Batch: &Batch{
			Steps: []Step{
				{Name: "extract", Tool: "extractor"},
				{Name: "summarize", Tool: "summarizer"},
			},
		},
	}, Input{})
	if err != nil {
		t.Fatalf("execute batch: %v", err)
	}

	if fmt.Sprint(calls) != "[extract summarize]" {
		t.Fatalf("expected sequential call order, got %v", calls)
	}
	if result.Output != "summary complete" {
		t.Fatalf("expected final step output, got %+v", result)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].ArtifactID != "art_summary" {
		t.Fatalf("expected final artifacts to come from last step, got %+v", result.Artifacts)
	}
}

func TestExecutorFanOutOverArtifactSets(t *testing.T) {
	exec := Executor{
		RunTool: func(ctx context.Context, step Step, input []artifact.ArtifactRef) (*tool.ToolResult, error) {
			if len(input) != 1 {
				t.Fatalf("expected single fan-out artifact, got %+v", input)
			}
			return &tool.ToolResult{
				Output: fmt.Sprintf("caption:%s", input[0].ArtifactID),
				Artifacts: []artifact.ArtifactRef{
					artifact.NewGeneratedRef("caption_"+input[0].ArtifactID, artifact.ArtifactKindText),
				},
			}, nil
		},
	}

	result, err := exec.Execute(context.Background(), Step{
		FanOut: &FanOut{
			Collection: "frames",
			Step: Step{Name: "caption", Tool: "captioner"},
		},
	}, Input{
		Collections: map[string][]artifact.ArtifactRef{
			"frames": {
				artifact.NewGeneratedRef("f1", artifact.ArtifactKindImage),
				artifact.NewGeneratedRef("f2", artifact.ArtifactKindImage),
			},
		},
	})
	if err != nil {
		t.Fatalf("execute fan-out: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected two fan-out item results, got %+v", result.Items)
	}
	if result.Items[0].Output != "caption:f1" || result.Items[1].Output != "caption:f2" {
		t.Fatalf("expected ordered per-item outputs, got %+v", result.Items)
	}
	if len(result.Artifacts) != 2 || result.Artifacts[0].ArtifactID != "caption_f1" || result.Artifacts[1].ArtifactID != "caption_f2" {
		t.Fatalf("expected fan-out artifacts to remain ordered, got %+v", result.Artifacts)
	}
}

func TestExecutorFanInAggregationOrdering(t *testing.T) {
	exec := Executor{
		RunTool: func(ctx context.Context, step Step, input []artifact.ArtifactRef) (*tool.ToolResult, error) {
			return &tool.ToolResult{
				Output: fmt.Sprintf("caption:%s", input[0].ArtifactID),
			}, nil
		},
	}

	result, err := exec.Execute(context.Background(), Step{
		Batch: &Batch{
			Steps: []Step{
				{
					FanOut: &FanOut{
						Collection: "frames",
						Step:       Step{Name: "caption", Tool: "captioner"},
					},
				},
				{
					FanIn: &FanIn{
						Strategy: "ordered",
						Into:     "joined_captions",
					},
				},
			},
		},
	}, Input{
		Collections: map[string][]artifact.ArtifactRef{
			"frames": {
				artifact.NewGeneratedRef("f1", artifact.ArtifactKindImage),
				artifact.NewGeneratedRef("f2", artifact.ArtifactKindImage),
			},
		},
	})
	if err != nil {
		t.Fatalf("execute fan-in batch: %v", err)
	}

	joined, ok := result.Structured.(map[string]any)
	if !ok {
		t.Fatalf("expected structured fan-in output, got %+v", result.Structured)
	}
	values, ok := joined["joined_captions"].([]string)
	if !ok {
		t.Fatalf("expected ordered caption slice, got %+v", joined)
	}
	if fmt.Sprint(values) != "[caption:f1 caption:f2]" {
		t.Fatalf("expected ordered fan-in aggregation, got %+v", values)
	}
}

func TestExecutorRetryingFailedStepOnly(t *testing.T) {
	attempts := 0
	var calls []string
	exec := Executor{
		RunTool: func(ctx context.Context, step Step, input []artifact.ArtifactRef) (*tool.ToolResult, error) {
			calls = append(calls, step.Name)
			if step.Name == "stable" {
				return &tool.ToolResult{Output: "ok"}, nil
			}
			attempts++
			if attempts < 3 {
				return nil, errors.New("temporary failure")
			}
			return &tool.ToolResult{Output: "recovered"}, nil
		},
	}

	result, err := exec.Execute(context.Background(), Step{
		Batch: &Batch{
			Steps: []Step{
				{Name: "stable", Tool: "noop"},
				{
					Retry: &Retry{
						Attempts: 3,
						Step:     Step{Name: "unstable", Tool: "flaky"},
					},
				},
			},
		},
	}, Input{})
	if err != nil {
		t.Fatalf("execute retry batch: %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected retry wrapper to retry failed step only, got %d attempts", attempts)
	}
	if fmt.Sprint(calls) != "[stable unstable unstable unstable]" {
		t.Fatalf("expected stable step once and flaky step retried, got %v", calls)
	}
	if result.Output != "recovered" {
		t.Fatalf("expected retry result to surface final success, got %+v", result)
	}
}
