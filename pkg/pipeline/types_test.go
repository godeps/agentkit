package pipeline

import (
	"testing"

	"github.com/godeps/agentkit/pkg/artifact"
)

func TestStepDeclaresToolAndSkillWork(t *testing.T) {
	step := Step{
		Name:  "caption-image",
		Tool:  "image_caption",
		Input: []artifact.ArtifactRef{artifact.NewGeneratedRef("art_image", artifact.ArtifactKindImage)},
		With: map[string]any{
			"prompt": "describe the image",
		},
	}

	if step.Name != "caption-image" || step.Tool != "image_caption" {
		t.Fatalf("expected step to preserve identity and target, got %+v", step)
	}
	if len(step.Input) != 1 || step.Input[0].ArtifactID != "art_image" {
		t.Fatalf("expected step to preserve input artifacts, got %+v", step.Input)
	}
	if step.With["prompt"] != "describe the image" {
		t.Fatalf("expected step params to be preserved, got %+v", step.With)
	}
}

func TestBatchDeclaresOrderedSteps(t *testing.T) {
	batch := Batch{
		Steps: []Step{
			{Name: "extract"},
			{Name: "summarize"},
		},
	}

	if len(batch.Steps) != 2 || batch.Steps[0].Name != "extract" || batch.Steps[1].Name != "summarize" {
		t.Fatalf("expected ordered batch steps, got %+v", batch.Steps)
	}
}

func TestFanOutDeclaresStepOverArtifactCollection(t *testing.T) {
	fanOut := FanOut{
		Collection: "input_artifacts",
		Step: Step{
			Name: "caption-each",
			Tool: "caption",
		},
	}

	if fanOut.Collection != "input_artifacts" {
		t.Fatalf("expected collection selector to be preserved, got %+v", fanOut)
	}
	if fanOut.Step.Tool != "caption" {
		t.Fatalf("expected fan-out step target to be preserved, got %+v", fanOut.Step)
	}
}

func TestFanInDeclaresAggregationShape(t *testing.T) {
	fanIn := FanIn{
		Strategy: "ordered",
		Into:     "combined_transcript",
	}

	if fanIn.Strategy != "ordered" || fanIn.Into != "combined_transcript" {
		t.Fatalf("expected fan-in declaration to be preserved, got %+v", fanIn)
	}
}

func TestConditionalDeclaresBranchTargets(t *testing.T) {
	conditional := Conditional{
		Condition: "has_artifacts",
		Then:      Step{Name: "process"},
		Else:      &Step{Name: "skip"},
	}

	if conditional.Condition != "has_artifacts" {
		t.Fatalf("expected conditional expression to be preserved, got %+v", conditional)
	}
	if conditional.Then.Name != "process" || conditional.Else == nil || conditional.Else.Name != "skip" {
		t.Fatalf("expected branch steps to be preserved, got %+v", conditional)
	}
}

func TestRetryDeclaresAttemptPolicy(t *testing.T) {
	retry := Retry{
		Attempts: 3,
		Step: Step{
			Name: "transcribe",
			Tool: "speech_to_text",
		},
	}

	if retry.Attempts != 3 || retry.Step.Tool != "speech_to_text" {
		t.Fatalf("expected retry policy to be preserved, got %+v", retry)
	}
}

func TestCheckpointDeclaresResumableBoundary(t *testing.T) {
	checkpoint := Checkpoint{
		Name: "after-review",
		Step: Step{Name: "human-review"},
	}

	if checkpoint.Name != "after-review" || checkpoint.Step.Name != "human-review" {
		t.Fatalf("expected checkpoint boundary to be preserved, got %+v", checkpoint)
	}
}
