package subagents

import (
	"context"
	"testing"
)

func TestDispatchContextHelpers(t *testing.T) {
	ctx := WithDispatchSource(context.TODO(), " Task_Tool ")
	if got := dispatchSource(ctx); got != DispatchSourceTaskTool {
		t.Fatalf("expected normalized task source, got %q", got)
	}

	ctx = WithDispatchSource(ctx, "other")
	if got := dispatchSource(ctx); got != "other" {
		t.Fatalf("expected source override, got %q", got)
	}

	bg := context.TODO()
	emptyCtx := WithDispatchSource(bg, " ")
	if emptyCtx != bg {
		t.Fatalf("expected empty source to return original context")
	}
	if got := dispatchSource(context.TODO()); got != "" {
		t.Fatalf("context without source should yield empty source, got %q", got)
	}
}
