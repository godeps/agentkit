package checkpoint

import (
	"context"
	"testing"
)

func TestMemoryStoreRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	record := Record{
		ID:      "cp-1",
		Session: "sess-1",
		Kind:    KindPlan,
		State: map[string]any{
			"step": "beta",
		},
	}

	if err := store.Save(context.Background(), record); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.Load(context.Background(), "cp-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ID != "cp-1" || got.Kind != KindPlan || got.Session != "sess-1" {
		t.Fatalf("unexpected record: %+v", got)
	}
	state, ok := got.State.(map[string]any)
	if !ok || state["step"] != "beta" {
		t.Fatalf("unexpected state: %#v", got.State)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	if err := store.Save(context.Background(), Record{ID: "cp-2", Kind: KindApproval}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.Delete(context.Background(), "cp-2"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Load(context.Background(), "cp-2"); err == nil {
		t.Fatal("expected load after delete to fail")
	}
}
