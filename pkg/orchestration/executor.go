package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Input carries request-scoped values that influence orchestration decisions.
type Input struct {
	Metadata map[string]any
}

// Invoker executes a task node against the underlying runtime.
type Invoker func(context.Context, Node, Input) (*ResultEnvelope, error)

// Executor walks a declarative orchestration plan using an injected leaf runner.
type Executor struct {
	invoke Invoker
}

// NewExecutor constructs a plan executor.
func NewExecutor(invoke Invoker) *Executor {
	return &Executor{invoke: invoke}
}

// Execute evaluates a plan and returns the aggregated result envelope.
func (e *Executor) Execute(ctx context.Context, plan Node, input Input) (*ResultEnvelope, error) {
	if e == nil || e.invoke == nil {
		return nil, errors.New("orchestration: executor requires an invoker")
	}
	return e.executeNode(ctx, cloneNode(plan), input)
}

func (e *Executor) executeNode(ctx context.Context, node Node, input Input) (*ResultEnvelope, error) {
	switch node.Kind {
	case KindSequence:
		return e.executeSequence(ctx, node, input)
	case KindParallel:
		return e.executeParallel(ctx, node, input)
	case KindConditional:
		return e.executeConditional(ctx, node, input)
	case "", KindTask:
		return e.executeTask(ctx, node, input)
	default:
		return nil, fmt.Errorf("orchestration: unsupported node kind %q", node.Kind)
	}
}

func (e *Executor) executeSequence(ctx context.Context, node Node, input Input) (*ResultEnvelope, error) {
	out := &ResultEnvelope{}
	var parts []string
	for _, child := range node.Nodes {
		res, err := e.executeNode(ctx, child, input)
		if err != nil {
			return nil, err
		}
		out = mergeEnvelope(out, res)
		if text := strings.TrimSpace(res.Text); text != "" {
			parts = append(parts, text)
		}
	}
	out.Text = strings.Join(parts, "\n")
	return out, nil
}

func (e *Executor) executeParallel(ctx context.Context, node Node, input Input) (*ResultEnvelope, error) {
	type branchResult struct {
		idx    int
		name   string
		result *ResultEnvelope
		err    error
	}

	results := make([]branchResult, len(node.Branches))
	var wg sync.WaitGroup
	for idx, branch := range node.Branches {
		wg.Add(1)
		go func(idx int, branch Branch) {
			defer wg.Done()
			res, err := e.executeNode(ctx, branch.Node, input)
			results[idx] = branchResult{idx: idx, name: branch.Name, result: res, err: err}
		}(idx, branch)
	}
	wg.Wait()

	out := &ResultEnvelope{Branches: map[string]ResultEnvelope{}}
	var parts []string
	for _, item := range results {
		if item.err != nil {
			return nil, item.err
		}
		if item.result == nil {
			continue
		}
		out.Branches[item.name] = *cloneEnvelope(item.result)
		if text := strings.TrimSpace(item.result.Text); text != "" {
			parts = append(parts, text)
		}
	}
	out.Text = strings.Join(parts, "\n")
	return out, nil
}

func (e *Executor) executeConditional(ctx context.Context, node Node, input Input) (*ResultEnvelope, error) {
	for _, branch := range node.Branches {
		if matchesCondition(branch.When, input.Metadata) {
			return e.executeNode(ctx, branch.Node, input)
		}
	}
	if node.Default != nil {
		return e.executeNode(ctx, *node.Default, input)
	}
	return &ResultEnvelope{}, nil
}

func (e *Executor) executeTask(ctx context.Context, node Node, input Input) (*ResultEnvelope, error) {
	attempts := 1
	if node.Retry != nil && node.Retry.MaxAttempts > 1 {
		attempts = node.Retry.MaxAttempts
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		res, err := e.invoke(ctx, node, input)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func matchesCondition(expr string, input map[string]any) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	left, right, found := strings.Cut(expr, "==")
	if !found {
		return false
	}
	left = strings.TrimSpace(left)
	if !strings.HasPrefix(left, "input.") {
		return false
	}
	key := strings.TrimSpace(strings.TrimPrefix(left, "input."))
	if key == "" {
		return false
	}
	value, ok := input[key]
	if !ok {
		return false
	}
	expected := strings.TrimSpace(right)
	if parsed, err := strconv.Unquote(expected); err == nil {
		return fmt.Sprint(value) == parsed
	}
	switch strings.ToLower(expected) {
	case "true", "false":
		return fmt.Sprint(value) == strings.ToLower(expected)
	default:
		return fmt.Sprint(value) == expected
	}
}

func mergeEnvelope(dst, src *ResultEnvelope) *ResultEnvelope {
	if dst == nil {
		return cloneEnvelope(src)
	}
	if src == nil {
		return dst
	}
	if strings.TrimSpace(src.Text) != "" {
		dst.Text = src.Text
	}
	if len(src.Structured) > 0 {
		if dst.Structured == nil {
			dst.Structured = map[string]any{}
		}
		for k, v := range src.Structured {
			dst.Structured[k] = v
		}
	}
	if len(src.Branches) > 0 {
		if dst.Branches == nil {
			dst.Branches = map[string]ResultEnvelope{}
		}
		for k, v := range src.Branches {
			dst.Branches[k] = v
		}
	}
	if len(src.Metadata) > 0 {
		if dst.Metadata == nil {
			dst.Metadata = map[string]any{}
		}
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}
	return dst
}
