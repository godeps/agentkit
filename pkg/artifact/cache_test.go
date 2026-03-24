package artifact

import "testing"

func TestCacheKeyDeterministicFromToolParamsAndArtifacts(t *testing.T) {
	refs := []ArtifactRef{
		NewGeneratedRef("art_b", ArtifactKindDocument),
		NewLocalFileRef("/tmp/input.txt", ArtifactKindText),
	}

	first := NewCacheKey("summarize", map[string]any{
		"temperature": 0,
		"options": map[string]any{
			"format": "markdown",
			"max":    3,
		},
	}, refs)
	second := NewCacheKey("summarize", map[string]any{
		"options": map[string]any{
			"max":    3,
			"format": "markdown",
		},
		"temperature": 0,
	}, refs)

	if first != second {
		t.Fatalf("expected deterministic cache key, got %q and %q", first, second)
	}
}

func TestCacheKeyChangesWhenArtifactInputsChange(t *testing.T) {
	baseParams := map[string]any{"prompt": "describe"}
	first := NewCacheKey("caption", baseParams, []ArtifactRef{
		NewGeneratedRef("art_1", ArtifactKindImage),
	})
	second := NewCacheKey("caption", baseParams, []ArtifactRef{
		NewGeneratedRef("art_2", ArtifactKindImage),
	})

	if first == second {
		t.Fatalf("expected cache key to change when artifact refs change, got %q", first)
	}
}
