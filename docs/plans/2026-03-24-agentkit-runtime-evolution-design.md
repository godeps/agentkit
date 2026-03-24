# AgentKit Runtime Evolution Design

## Task 1 Scope

Phase 1 starts by introducing a declarative orchestration layer that can be attached to `api.Request` without changing how the runtime executes requests today. The first slice is intentionally non-executable: it defines the data model for plan composition and the minimal result envelope that later tasks will adapt to runtime output.

## Orchestration Model

- `orchestration.Node` is a serializable plan node with a `Kind`, optional `Name`, child `Nodes`, named `Branches`, optional `Default` branch, and optional `RetrySpec`.
- `Sequence(...)` models ordered composition.
- `Parallel(...)` models named fan-out and declares that fan-in aggregates branch results into a shared `ResultEnvelope.Branches` map.
- `Conditional(...)` models ordered predicate branches plus a default fallback path.
- `RetrySpec` remains declarative in this task and only records retry intent.

## API Surface

- `api.Request.Plan` carries an optional orchestration plan.
- `api.Result.Envelope` carries the minimal structured output shape shared with orchestration.
- Existing `Output`, `Usage`, `ToolCalls`, and `StopReason` fields remain unchanged for backward compatibility.

## Execution Boundary

Task 1 does not execute orchestration nodes. The runtime still runs the existing single-loop agent path, while `api.Result.Envelope` mirrors the final text output so later tasks can adapt both plain runtime responses and orchestration branch results to one common shape.
