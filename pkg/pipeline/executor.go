package pipeline

import (
	"context"
	"fmt"

	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/runtime/cache"
	"github.com/godeps/agentkit/pkg/tool"
)

// Input carries the runtime inputs used for pipeline execution.
type Input struct {
	Artifacts   []artifact.ArtifactRef
	Collections map[string][]artifact.ArtifactRef
	Items       []Result
}

// Result captures the output of executing a pipeline step.
type Result struct {
	Output     string
	Summary    string
	Artifacts  []artifact.ArtifactRef
	Structured any
	Preview    *tool.Preview
	Items      []Result
	Lineage    artifact.LineageGraph
}

// Executor runs lightweight multimodal pipeline steps against injected runtime surfaces.
type Executor struct {
	RunTool  func(context.Context, Step, []artifact.ArtifactRef) (*tool.ToolResult, error)
	RunSkill func(context.Context, Step, []artifact.ArtifactRef) (*tool.ToolResult, error)
	Cache    cache.Store
}

// Execute runs a declared pipeline step and returns its aggregated result.
func (e Executor) Execute(ctx context.Context, step Step, input Input) (Result, error) {
	return e.execute(ctx, step, input)
}

func (e Executor) execute(ctx context.Context, step Step, input Input) (Result, error) {
	switch {
	case step.Batch != nil:
		return e.executeBatch(ctx, *step.Batch, input)
	case step.FanOut != nil:
		return e.executeFanOut(ctx, *step.FanOut, input)
	case step.FanIn != nil:
		return e.executeFanIn(*step.FanIn, input), nil
	case step.Retry != nil:
		return e.executeRetry(ctx, *step.Retry, input)
	case step.Checkpoint != nil:
		return e.execute(ctx, step.Checkpoint.Step, input)
	default:
		return e.executeLeaf(ctx, step, input)
	}
}

func (e Executor) executeBatch(ctx context.Context, batch Batch, input Input) (Result, error) {
	current := cloneInput(input)
	var final Result
	for _, child := range batch.Steps {
		result, err := e.execute(ctx, child, current)
		if err != nil {
			return Result{}, err
		}
		final = mergeResults(final, result)
		current.Items = cloneResults(result.Items)
		if len(result.Artifacts) > 0 {
			current.Artifacts = append([]artifact.ArtifactRef(nil), result.Artifacts...)
		}
	}
	return final, nil
}

func (e Executor) executeFanOut(ctx context.Context, fanOut FanOut, input Input) (Result, error) {
	refs := input.Collections[fanOut.Collection]
	if len(refs) == 0 && fanOut.Collection == "" {
		refs = input.Artifacts
	}
	out := Result{}
	for _, ref := range refs {
		child, err := e.execute(ctx, fanOut.Step, Input{
			Artifacts:   []artifact.ArtifactRef{ref},
			Collections: cloneCollections(input.Collections),
		})
		if err != nil {
			return Result{}, err
		}
		out.Items = append(out.Items, child)
		out.Artifacts = append(out.Artifacts, child.Artifacts...)
		out.Lineage.Edges = append(out.Lineage.Edges, child.Lineage.Edges...)
	}
	return out, nil
}

func (e Executor) executeFanIn(fanIn FanIn, input Input) Result {
	values := make([]string, 0, len(input.Items))
	for _, item := range input.Items {
		values = append(values, item.Output)
	}
	return Result{
		Structured: map[string]any{
			fanIn.Into: values,
		},
		Items: cloneResults(input.Items),
	}
}

func (e Executor) executeRetry(ctx context.Context, retry Retry, input Input) (Result, error) {
	attempts := retry.Attempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		result, err := e.execute(ctx, retry.Step, input)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return Result{}, lastErr
}

func (e Executor) executeLeaf(ctx context.Context, step Step, input Input) (Result, error) {
	refs := input.Artifacts
	if len(step.Input) > 0 {
		refs = step.Input
	}
	cacheKey := e.cacheKey(step, refs)
	if e.Cache != nil && cacheKey != "" {
		if cached, ok, err := e.Cache.Load(ctx, cacheKey); err != nil {
			return Result{}, err
		} else if ok {
			return toolResultToPipelineResult(cached, refs, step.Name), nil
		}
	}

	var (
		res *tool.ToolResult
		err error
	)
	switch {
	case step.Tool != "":
		if e.RunTool == nil {
			return Result{}, fmt.Errorf("pipeline: tool runner not configured for step %q", step.Name)
		}
		res, err = e.RunTool(ctx, step, refs)
	case step.Skill != "":
		if e.RunSkill == nil {
			return Result{}, fmt.Errorf("pipeline: skill runner not configured for step %q", step.Name)
		}
		res, err = e.RunSkill(ctx, step, refs)
	default:
		return Result{}, fmt.Errorf("pipeline: step %q has no executable target", step.Name)
	}
	if err != nil {
		return Result{}, err
	}
	if res == nil {
		return Result{}, nil
	}

	result := Result{
		Output:     res.Output,
		Summary:    res.Summary,
		Artifacts:  append([]artifact.ArtifactRef(nil), res.Artifacts...),
		Structured: res.Structured,
		Preview:    res.Preview,
	}
	for _, src := range refs {
		for _, derived := range result.Artifacts {
			result.Lineage.AddEdge(src, derived, step.Name)
		}
	}
	if e.Cache != nil && cacheKey != "" {
		if err := e.Cache.Save(ctx, cacheKey, pipelineResultToToolResult(result)); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

func (e Executor) cacheKey(step Step, refs []artifact.ArtifactRef) artifact.CacheKey {
	target := step.Tool
	if target == "" {
		target = step.Skill
	}
	if target == "" {
		return ""
	}
	return artifact.NewCacheKey(target, step.With, refs)
}

func toolResultToPipelineResult(res *tool.ToolResult, refs []artifact.ArtifactRef, stepName string) Result {
	if res == nil {
		return Result{}
	}
	result := Result{
		Output:     res.Output,
		Summary:    res.Summary,
		Artifacts:  append([]artifact.ArtifactRef(nil), res.Artifacts...),
		Structured: res.Structured,
		Preview:    res.Preview,
	}
	for _, src := range refs {
		for _, derived := range result.Artifacts {
			result.Lineage.AddEdge(src, derived, stepName)
		}
	}
	return result
}

func pipelineResultToToolResult(result Result) *tool.ToolResult {
	return &tool.ToolResult{
		Output:     result.Output,
		Summary:    result.Summary,
		Artifacts:  append([]artifact.ArtifactRef(nil), result.Artifacts...),
		Structured: result.Structured,
		Preview:    result.Preview,
	}
}

func mergeResults(base, next Result) Result {
	if next.Output != "" {
		base.Output = next.Output
	}
	if next.Summary != "" {
		base.Summary = next.Summary
	}
	if next.Structured != nil {
		base.Structured = next.Structured
	}
	if next.Preview != nil {
		base.Preview = next.Preview
	}
	if len(next.Artifacts) > 0 {
		base.Artifacts = append([]artifact.ArtifactRef(nil), next.Artifacts...)
	}
	if len(next.Items) > 0 {
		base.Items = cloneResults(next.Items)
	}
	base.Lineage.Edges = append(base.Lineage.Edges, next.Lineage.Edges...)
	return base
}

func cloneInput(in Input) Input {
	return Input{
		Artifacts:   append([]artifact.ArtifactRef(nil), in.Artifacts...),
		Collections: cloneCollections(in.Collections),
		Items:       cloneResults(in.Items),
	}
}

func cloneCollections(in map[string][]artifact.ArtifactRef) map[string][]artifact.ArtifactRef {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]artifact.ArtifactRef, len(in))
	for key, refs := range in {
		out[key] = append([]artifact.ArtifactRef(nil), refs...)
	}
	return out
}

func cloneResults(in []Result) []Result {
	if len(in) == 0 {
		return nil
	}
	out := make([]Result, len(in))
	copy(out, in)
	return out
}
