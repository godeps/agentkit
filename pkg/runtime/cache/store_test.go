package cache

import (
	"context"
	"testing"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/tool"
)

func TestCacheMemoryStoreHitAndMiss(t *testing.T) {
	store := NewMemoryStore()
	key := artifact.NewCacheKey("caption", map[string]any{"prompt": "describe"}, []artifact.ArtifactRef{
		artifact.NewGeneratedRef("art_1", artifact.ArtifactKindImage),
	})
	want := &tool.ToolResult{Output: "cached"}

	if _, ok, err := store.Load(context.Background(), key); err != nil || ok {
		t.Fatalf("expected initial cache miss, got ok=%v err=%v", ok, err)
	}
	if err := store.Save(context.Background(), key, want); err != nil {
		t.Fatalf("save cache entry: %v", err)
	}
	got, ok, err := store.Load(context.Background(), key)
	if err != nil || !ok {
		t.Fatalf("expected cache hit, got ok=%v err=%v", ok, err)
	}
	if got == nil || got.Output != "cached" {
		t.Fatalf("expected cached result to round-trip, got %+v", got)
	}
}
