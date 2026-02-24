package subagents

import (
	"context"
	"reflect"
	"testing"
)

func TestContextMetadataAndLookup(t *testing.T) {
	ctx := Context{Metadata: map[string]any{"a": 1}}
	merged := ctx.WithMetadata(map[string]any{"b": 2})
	if merged.Metadata["a"] != 1 || merged.Metadata["b"] != 2 {
		t.Fatalf("metadata merge failed: %+v", merged.Metadata)
	}

	if _, ok := FromContext(context.TODO()); ok {
		t.Fatalf("expected nil context lookup to fail")
	}
	withCtx := WithContext(context.TODO(), Context{ToolWhitelist: []string{"bash"}})
	found, ok := FromContext(withCtx)
	if !ok || !reflect.DeepEqual(found.ToolWhitelist, []string{"bash"}) {
		t.Fatalf("expected context roundtrip, got %+v ok=%v", found, ok)
	}
}

func TestNormalizeToolsDeduplicates(t *testing.T) {
	tools := normalizeTools([]string{"Bash", "bash", "read", " "})
	if !reflect.DeepEqual(tools, []string{"bash", "read"}) {
		t.Fatalf("expected normalized tools, got %v", tools)
	}
	if set := toToolSet([]string{"bash", "bash"}); len(set) != 1 {
		t.Fatalf("expected deduplicated set, got %v", set)
	}
}
