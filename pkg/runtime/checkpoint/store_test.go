package checkpoint

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/pipeline"
)

func TestCheckpointMemoryStoreRoundTrip(t *testing.T) {
	store := NewMemoryStore()
	entry := Entry{
		SessionID: "sess-1",
		Remaining: &pipeline.Step{Name: "finalize", Tool: "finalizer"},
		Input: pipeline.Input{
			Artifacts: []artifact.ArtifactRef{
				artifact.NewGeneratedRef("art_1", artifact.ArtifactKindText),
			},
		},
		Result: pipeline.Result{Output: "prepared"},
	}

	id, err := store.Save(context.Background(), entry)
	if err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	if id == "" {
		t.Fatal("expected generated checkpoint id")
	}

	loaded, err := store.Load(context.Background(), id)
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if loaded.SessionID != "sess-1" {
		t.Fatalf("expected session id to round-trip, got %+v", loaded)
	}
	if loaded.Remaining == nil || loaded.Remaining.Name != "finalize" {
		t.Fatalf("expected remaining step to round-trip, got %+v", loaded.Remaining)
	}
	if len(loaded.Input.Artifacts) != 1 || loaded.Input.Artifacts[0].ArtifactID != "art_1" {
		t.Fatalf("expected input artifacts to round-trip, got %+v", loaded.Input)
	}
	if loaded.Result.Output != "prepared" {
		t.Fatalf("expected snapshot result to round-trip, got %+v", loaded.Result)
	}
}

func TestCheckpointMemoryStoreDelete(t *testing.T) {
	store := NewMemoryStore()
	id, err := store.Save(context.Background(), Entry{SessionID: "sess-2"})
	if err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	if err := store.Delete(context.Background(), id); err != nil {
		t.Fatalf("delete checkpoint: %v", err)
	}
	if _, err := store.Load(context.Background(), id); err == nil {
		t.Fatal("expected missing checkpoint after delete")
	}
}

func TestCheckpointFileStoreRoundTrip(t *testing.T) {
	store, err := NewFileStore(filepath.Join(t.TempDir(), "checkpoints.json"))
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	entry := Entry{
		SessionID: "sess-file",
		Remaining: &pipeline.Step{Name: "resume", Tool: "runner"},
		Result:    pipeline.Result{Output: "paused"},
	}

	id, err := store.Save(context.Background(), entry)
	if err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	reloaded, err := NewFileStore(store.path)
	if err != nil {
		t.Fatalf("reload file store: %v", err)
	}
	got, err := reloaded.Load(context.Background(), id)
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if got.SessionID != "sess-file" || got.Result.Output != "paused" {
		t.Fatalf("unexpected file-backed checkpoint: %+v", got)
	}
}
