package tool

import (
	"encoding/json"
	"testing"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/model"
)

func TestToolResultCarriesMultimodalFields(t *testing.T) {
	result := ToolResult{
		Success: true,
		Output:  "text output",
		Summary: "short preview",
		Structured: map[string]any{
			"labels": []string{"invoice", "paid"},
		},
		Artifacts: []artifact.ArtifactRef{
			artifact.NewGeneratedRef("art_123", artifact.ArtifactKindDocument),
		},
		ContentBlocks: []model.ContentBlock{{
			Type:      model.ContentBlockDocument,
			MediaType: "application/pdf",
			URL:       "https://example.com/invoice.pdf",
		}},
		Preview: &Preview{
			Title:     "Invoice",
			Summary:   "Paid invoice",
			MediaType: "application/pdf",
		},
	}

	if result.Output != "text output" {
		t.Fatalf("expected text output to be preserved, got %q", result.Output)
	}
	if result.Summary != "short preview" {
		t.Fatalf("expected summary to be preserved, got %q", result.Summary)
	}
	if got := result.Structured.(map[string]any)["labels"].([]string); len(got) != 2 {
		t.Fatalf("expected structured payload to be preserved, got %+v", result.Structured)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].ArtifactID != "art_123" {
		t.Fatalf("expected artifacts to be preserved, got %+v", result.Artifacts)
	}
	if len(result.ContentBlocks) != 1 || result.ContentBlocks[0].Type != model.ContentBlockDocument {
		t.Fatalf("expected content blocks to be preserved, got %+v", result.ContentBlocks)
	}
	if result.Preview == nil || result.Preview.Title != "Invoice" {
		t.Fatalf("expected preview metadata to be preserved, got %+v", result.Preview)
	}
}

func TestToolResultJSONIncludesMultimodalFields(t *testing.T) {
	result := ToolResult{
		Output:     "ok",
		Summary:    "preview",
		Structured: map[string]any{"status": "done"},
		Artifacts:  []artifact.ArtifactRef{artifact.NewLocalFileRef("/tmp/out.json", artifact.ArtifactKindJSON)},
		Preview: &Preview{
			Title:   "Done",
			Summary: "Structured output ready",
		},
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal tool result: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}

	for _, key := range []string{"output", "summary", "structured", "artifacts", "preview"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("expected JSON payload to include %q, got %+v", key, decoded)
		}
	}
}
