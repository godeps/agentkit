# Agentkit Structured Output Design

**Date:** 2026-03-09

**Scope:** Add first-class structured output support to Agentkit by propagating `response_format` / JSON Schema through the runtime and both OpenAI transport implementations.

## Goals

- Let callers configure structured output through `api.Options`.
- Carry structured output requirements through `model.Request`.
- Support both OpenAI transport paths:
  - chat completions
  - responses API
- Keep tool calling compatible with structured output on OpenAI-compatible providers.
- Document clear first-version support boundaries.

## Non-Goals

- Do not implement Anthropic-specific fallback orchestration in this pass.
- Do not add a general provider-capability negotiation framework in this pass.
- Do not redesign the agent loop around a second formatting pass.

## Current State

### Missing model-level schema carrier

`pkg/model/interface.go` exposes `Request`, but it has no `ResponseFormat` field. That means the runtime cannot express structured output requirements in a provider-agnostic way.

### Missing runtime-level configuration

`pkg/api/options.go` exposes model selection, tool controls, compact settings, and other runtime knobs, but there is no `OutputSchema` or equivalent field. Callers cannot configure structured output through the main SDK entrypoint.

### OpenAI transport split

The repository has two OpenAI paths:

- `pkg/model/openai.go` for chat completions
- `pkg/model/openai_responses.go` for the Responses API

If structured output is added to only one path, runtime behavior becomes inconsistent depending on `UseResponses`.

## Evaluated Approaches

### Option A: Chat completions only

- Add `ResponseFormat` only to `pkg/model/openai.go`.
- Pros: smallest diff.
- Cons: wrong abstraction boundary, incomplete support, inconsistent runtime behavior.

Rejected.

### Option B: Unified request model + both OpenAI paths

- Add provider-agnostic `ResponseFormat` types to `pkg/model/interface.go`.
- Add `Options.OutputSchema` in `pkg/api/options.go`.
- Inject the schema into every model request in `pkg/api/agent.go`.
- Map the schema to both OpenAI request builders.
- Leave unsupported providers unchanged.

Pros:

- One runtime abstraction
- Complete for current OpenAI implementation surface
- Small enough for a first version

Chosen.

### Option C: Full provider capability abstraction + automatic fallback

- Add capability negotiation and provider-specific downgrade paths.
- Potentially run a second formatting request when native support is absent.

Pros:

- Most flexible long-term model

Cons:

- Too much surface area for the first version
- Harder to verify
- Couples model capability discovery and orchestration prematurely

Deferred.

## Chosen Design

### 1. Model-layer request shape

Add to `pkg/model/interface.go`:

- `type ResponseFormat struct`
- `type OutputJSONSchema struct`
- `Request.ResponseFormat *ResponseFormat`

Supported `ResponseFormat.Type` values:

- `text`
- `json_object`
- `json_schema`

Validation rules:

- `json_schema` requires non-nil `JSONSchema`
- `JSONSchema.Name` must be non-empty after trim
- `JSONSchema.Schema` must be non-nil

The `model` package will define the request shape, but provider-specific validation remains inside provider builders.

### 2. Runtime configuration surface

Add to `pkg/api/options.go`:

- `OutputSchema *model.ResponseFormat`

This field becomes the caller-facing configuration point. It should be copied through frozen options and preserved the same way other pointer configuration fields are handled.

### 3. Agent request injection

Update request construction in `pkg/api/agent.go` so every model request receives:

- conversation messages
- tool definitions
- system prompt
- prompt cache flag
- `ResponseFormat: m.outputSchema`

This keeps the agent loop unchanged. The model remains free to emit tool calls; when it emits text, OpenAI-compatible providers will constrain that text according to the schema.

### 4. OpenAI chat completions mapping

Update `pkg/model/openai.go`:

- map `Request.ResponseFormat` into `openai.ChatCompletionNewParams.ResponseFormat`
- marshal JSON Schema payload when `Type == "json_schema"`
- return a descriptive error if schema marshaling fails or required fields are missing

### 5. OpenAI Responses API mapping

Update `pkg/model/openai_responses.go` with equivalent behavior:

- map `Request.ResponseFormat` into `responses.ResponseNewParams`
- support `json_object` and `json_schema`
- keep behavior aligned with chat completions

This is required for first-version completeness because the repository already supports the Responses API as a first-class transport path.

### 6. Unsupported providers

First-version behavior for non-OpenAI providers:

- do not add fallback orchestration
- do not attempt automatic second-pass formatting
- keep provider behavior unchanged unless explicit support is added later

Documentation will state that structured output currently applies to OpenAI-compatible transports only.

## Error Handling

- Invalid local schema construction should fail before the provider request is sent.
- Schema marshal errors should be returned from the provider request builder.
- Unsupported provider behavior should remain stable and documented rather than silently approximated.

## Testing Strategy

### Model tests

- `pkg/model/openai_test.go`
  - `json_object` maps to the correct OpenAI request shape
  - `json_schema` maps name / description / schema / strict correctly
  - invalid schema yields an error

- `pkg/model/openai_responses_test.go`
  - same coverage for Responses API mapping

### API/runtime tests

- `pkg/api/*_test.go`
  - `Options.OutputSchema` is injected into generated `model.Request`
  - injected request preserves tools and other existing request fields

### Regression expectations

- Existing OpenAI requests without schema remain unchanged.
- Existing non-OpenAI providers remain unchanged.
- Tool calling behavior remains intact.

## Documentation Changes

Update API docs to describe:

- `model.ResponseFormat`
- `model.OutputJSONSchema`
- `api.Options.OutputSchema`
- support scope: OpenAI-compatible chat completions and Responses API

## Future Extensions

Possible future work after this first version:

- provider capability signaling
- Anthropic two-pass fallback
- helper constructors for common schema patterns
- stricter SDK-side validation of JSON Schema subsets
