package orchestration

import (
	"testing"
	"time"
)

func TestSequence(t *testing.T) {
	t.Parallel()

	seq := Sequence(
		Node{Kind: KindTask, Name: "collect"},
		Node{Kind: KindTask, Name: "summarize"},
	)

	if seq.Kind != KindSequence {
		t.Fatalf("expected sequence kind, got %q", seq.Kind)
	}
	if len(seq.Nodes) != 2 {
		t.Fatalf("expected 2 child nodes, got %d", len(seq.Nodes))
	}
	if seq.Nodes[0].Name != "collect" || seq.Nodes[1].Name != "summarize" {
		t.Fatalf("unexpected node order: %+v", seq.Nodes)
	}
}

func TestParallel(t *testing.T) {
	t.Parallel()

	par := Parallel(
		FanOut("draft", Node{Kind: KindTask, Name: "draft"}),
		FanOut("review", Node{Kind: KindTask, Name: "review"}),
	)

	if par.Kind != KindParallel {
		t.Fatalf("expected parallel kind, got %q", par.Kind)
	}
	if len(par.Branches) != 2 {
		t.Fatalf("expected 2 parallel branches, got %d", len(par.Branches))
	}
	if par.Branches[0].Name != "draft" || par.Branches[1].Name != "review" {
		t.Fatalf("unexpected branch names: %+v", par.Branches)
	}
	if par.Result == nil {
		t.Fatal("expected parallel result envelope declaration")
	}
	if par.Result.Branches == nil {
		t.Fatal("expected parallel result envelope to reserve named branch aggregation")
	}
}

func TestConditional(t *testing.T) {
	t.Parallel()

	plan := Conditional(
		When("input.route == \"tools\"", Node{Kind: KindTask, Name: "tool_path"}),
		Otherwise(Node{Kind: KindTask, Name: "chat_path"}),
	)

	if plan.Kind != KindConditional {
		t.Fatalf("expected conditional kind, got %q", plan.Kind)
	}
	if len(plan.Branches) != 1 {
		t.Fatalf("expected 1 conditional branch, got %d", len(plan.Branches))
	}
	if plan.Branches[0].When != "input.route == \"tools\"" {
		t.Fatalf("unexpected condition: %+v", plan.Branches[0])
	}
	if plan.Default == nil || plan.Default.Name != "chat_path" {
		t.Fatalf("unexpected default branch: %+v", plan.Default)
	}
}

func TestRetry(t *testing.T) {
	t.Parallel()

	retried := Sequence(Node{Kind: KindTask, Name: "fetch"}).WithRetry(RetrySpec{
		MaxAttempts: 3,
		Backoff:     2 * time.Second,
	})

	if retried.Retry == nil {
		t.Fatal("expected retry declaration")
	}
	if retried.Retry.MaxAttempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", retried.Retry.MaxAttempts)
	}
	if retried.Retry.Backoff != 2*time.Second {
		t.Fatalf("unexpected backoff: %v", retried.Retry.Backoff)
	}
}
