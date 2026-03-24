package api

import (
	"context"

	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/orchestration"
)

type ModelRequestPolicy interface {
	ApplyModelRequest(context.Context, ModelRequestPolicyInput) (model.Request, error)
}

type ModelRequestPolicyFunc func(context.Context, ModelRequestPolicyInput) (model.Request, error)

func (fn ModelRequestPolicyFunc) ApplyModelRequest(ctx context.Context, input ModelRequestPolicyInput) (model.Request, error) {
	return fn(ctx, input)
}

type ToolSelectionPolicy interface {
	SelectTools(context.Context, ToolSelectionPolicyInput) ([]model.ToolDefinition, error)
}

type ToolSelectionPolicyFunc func(context.Context, ToolSelectionPolicyInput) ([]model.ToolDefinition, error)

func (fn ToolSelectionPolicyFunc) SelectTools(ctx context.Context, input ToolSelectionPolicyInput) ([]model.ToolDefinition, error) {
	return fn(ctx, input)
}

type IterationPolicy interface {
	DecideIteration(context.Context, IterationPolicyInput) (IterationDecision, error)
}

type IterationPolicyFunc func(context.Context, IterationPolicyInput) (IterationDecision, error)

func (fn IterationPolicyFunc) DecideIteration(ctx context.Context, input IterationPolicyInput) (IterationDecision, error) {
	return fn(ctx, input)
}

type OutputPolicy interface {
	ApplyOutput(context.Context, OutputPolicyInput) (*orchestration.ResultEnvelope, error)
}

type OutputPolicyFunc func(context.Context, OutputPolicyInput) (*orchestration.ResultEnvelope, error)

func (fn OutputPolicyFunc) ApplyOutput(ctx context.Context, input OutputPolicyInput) (*orchestration.ResultEnvelope, error) {
	return fn(ctx, input)
}

type ModelRequestPolicyInput struct {
	Request   model.Request
	Iteration int
	SessionID string
	RequestID string
	Metadata  map[string]any
}

type ToolSelectionPolicyInput struct {
	Tools     []model.ToolDefinition
	Iteration int
	SessionID string
	RequestID string
	Metadata  map[string]any
}

type IterationPolicyInput struct {
	Iteration int
	SessionID string
	RequestID string
	Metadata  map[string]any
}

type IterationDecision struct {
	Continue   bool
	StopReason string
	Output     string
}

type OutputPolicyInput struct {
	Envelope   *orchestration.ResultEnvelope
	Usage      model.Usage
	StopReason string
	SessionID  string
	RequestID  string
	Metadata   map[string]any
}
