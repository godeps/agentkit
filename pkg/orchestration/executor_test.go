package orchestration

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestExecutorSequenceInvokesRuntimeInOrder(t *testing.T) {
	t.Parallel()

	invoker := newRecordingInvoker(map[string][]invokeResult{
		"first":  {{text: "alpha"}},
		"second": {{text: "beta"}},
	})

	exec := NewExecutor(invoker.Invoke)
	plan := Sequence(
		Node{Kind: KindTask, Name: "first"},
		Node{Kind: KindTask, Name: "second"},
	)

	got, err := exec.Execute(context.Background(), plan, Input{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !reflect.DeepEqual(invoker.calls(), []string{"first", "second"}) {
		t.Fatalf("unexpected invocation order: %v", invoker.calls())
	}
	if got == nil || got.Text != "alpha\nbeta" {
		t.Fatalf("unexpected result envelope: %+v", got)
	}
}

func TestExecutorParallelPreservesOutputOrdering(t *testing.T) {
	t.Parallel()

	invoker := newRecordingInvoker(map[string][]invokeResult{
		"slow": {{text: "alpha", delay: 20 * time.Millisecond}},
		"fast": {{text: "beta"}},
	})

	exec := NewExecutor(invoker.Invoke)
	plan := Parallel(
		FanOut("slow", Node{Kind: KindTask, Name: "slow"}),
		FanOut("fast", Node{Kind: KindTask, Name: "fast"}),
	)

	got, err := exec.Execute(context.Background(), plan, Input{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got == nil || got.Text != "alpha\nbeta" {
		t.Fatalf("expected declaration order to be preserved, got %+v", got)
	}
	if got.Branches["slow"].Text != "alpha" || got.Branches["fast"].Text != "beta" {
		t.Fatalf("unexpected branch aggregation: %+v", got.Branches)
	}
}

func TestExecutorConditionalChoosesOneBranch(t *testing.T) {
	t.Parallel()

	invoker := newRecordingInvoker(map[string][]invokeResult{
		"tools": {{text: "tool path"}},
		"chat":  {{text: "chat path"}},
	})

	exec := NewExecutor(invoker.Invoke)
	plan := Conditional(
		When(`input.route == "tools"`, Node{Kind: KindTask, Name: "tools"}),
		Otherwise(Node{Kind: KindTask, Name: "chat"}),
	)

	got, err := exec.Execute(context.Background(), plan, Input{Metadata: map[string]any{"route": "tools"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got == nil || got.Text != "tool path" {
		t.Fatalf("unexpected conditional result: %+v", got)
	}
	if !reflect.DeepEqual(invoker.calls(), []string{"tools"}) {
		t.Fatalf("expected only the selected branch to run, got %v", invoker.calls())
	}
}

func TestExecutorRetryRetriesOnlyWrappedNode(t *testing.T) {
	t.Parallel()

	invoker := newRecordingInvoker(map[string][]invokeResult{
		"first":   {{text: "alpha"}},
		"retryme": {{err: errors.New("transient")}, {text: "beta"}},
		"third":   {{text: "gamma"}},
	})

	exec := NewExecutor(invoker.Invoke)
	plan := Sequence(
		Node{Kind: KindTask, Name: "first"},
		Node{Kind: KindTask, Name: "retryme"}.WithRetry(RetrySpec{MaxAttempts: 2}),
		Node{Kind: KindTask, Name: "third"},
	)

	got, err := exec.Execute(context.Background(), plan, Input{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got == nil || got.Text != "alpha\nbeta\ngamma" {
		t.Fatalf("unexpected retry result: %+v", got)
	}
	if !reflect.DeepEqual(invoker.calls(), []string{"first", "retryme", "retryme", "third"}) {
		t.Fatalf("unexpected retry scope: %v", invoker.calls())
	}
}

type invokeResult struct {
	text  string
	err   error
	delay time.Duration
}

type recordingInvoker struct {
	mu      sync.Mutex
	results map[string][]invokeResult
	order   []string
	counts  map[string]int
}

func newRecordingInvoker(results map[string][]invokeResult) *recordingInvoker {
	return &recordingInvoker{
		results: results,
		counts:  map[string]int{},
	}
}

func (r *recordingInvoker) Invoke(_ context.Context, node Node, _ Input) (*ResultEnvelope, error) {
	r.mu.Lock()
	r.order = append(r.order, node.Name)
	seq := r.counts[node.Name]
	r.counts[node.Name] = seq + 1
	result := r.results[node.Name][seq]
	r.mu.Unlock()

	if result.delay > 0 {
		time.Sleep(result.delay)
	}
	if result.err != nil {
		return nil, result.err
	}
	return &ResultEnvelope{Text: result.text}, nil
}

func (r *recordingInvoker) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}
