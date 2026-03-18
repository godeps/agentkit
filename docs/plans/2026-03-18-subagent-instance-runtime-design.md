# Subagent Instance Runtime Design

**Date:** 2026-03-18

**Goal:** Upgrade `agentkit` subagents from synchronous prompt-profile dispatch to real runtime-managed subagent instances while preserving the current synchronous `Task` tool behavior.

## Summary

`agentkit` currently loads subagent markdown files into runtime registrations, but the loaded handler only returns the markdown body as output. This makes `Task` behave like a synchronous prompt-profile expansion rather than a real delegated subagent execution. The goal of this design is to keep the external `Task` interface stable while introducing an internal subagent instance runtime with IDs, statuses, lifecycle events, and isolated execution.

The first version intentionally stops short of a full Codex-style public async collaboration surface. Internally, however, the runtime will gain `spawn`, `wait`, and `get` semantics so the system can evolve toward explicit async tools later without another architectural reset.

## Current Problems

- `pkg/runtime/subagents/loader.go` builds handlers that return markdown bodies directly instead of executing a child agent run.
- `pkg/runtime/subagents/manager.go` mixes profile registration, target selection, and synchronous execution.
- `pkg/tool/builtin/task.go` describes subprocess-style delegation, but `pkg/api/agent.go` currently performs a synchronous in-process dispatch.
- Existing subagent events are too coarse for runtime instance lifecycle tracking.
- There is no stable runtime representation of subagent state, IDs, or completion/error metadata.

## Design

### 1. Split Profiles From Instances

Keep the existing subagent definition and loading model as the profile layer:

- Profile metadata remains responsible for name, description, tool restrictions, model hints, and matcher behavior.
- The markdown body becomes subagent instructions, not direct output.

Add a new instance layer:

- Each subagent execution gets a generated instance ID.
- Each instance tracks status, timestamps, result, and error text.
- Each instance is associated with a parent session and logical child session.

### 2. Narrow Manager Responsibility

`pkg/runtime/subagents.Manager` should become a profile registry plus target selector:

- register profile definitions and handlers
- list registered profiles
- select a target profile from an explicit target or matchers

It should no longer be the primary execution surface.

### 3. Add Executor + Store

Introduce a runtime executor in `pkg/runtime/subagents`:

- `Spawn(ctx, SpawnRequest) (SpawnHandle, error)`
- `Wait(ctx, WaitRequest) (WaitResult, error)`
- `Get(ctx, id) (Instance, error)`

Introduce an internal store abstraction:

- in-memory implementation for first release
- thread-safe lookup/update by instance ID
- ability to list instances by session when needed later

Execution flow:

1. Validate dispatch source, instruction, and target.
2. Select the subagent profile.
3. Create an instance with `queued` status and persist it.
4. Launch background execution.
5. Transition instance to `running`.
6. Execute the subagent through a runtime-provided runner.
7. Persist `completed`, `failed`, or `cancelled` final state.

### 4. Runtime Runner Contract

The executor should depend on a runner interface implemented in `pkg/api` so the subagent package stays runtime-focused instead of importing the whole API surface.

The runner will:

- combine profile instructions with the delegated task prompt
- inherit a request-scoped execution snapshot
- apply tool whitelist intersections
- apply model selection and request metadata
- perform the child agent run and convert its output into `subagents.Result`

### 5. Preserve Synchronous Task Semantics

The `Task` tool keeps its public schema and synchronous result behavior.

Internally it changes from:

- `Task -> executeSubagent -> Manager.Dispatch`

to:

- `Task -> Executor.Spawn -> Executor.Wait -> convert final result`

This preserves compatibility for current users and prompt expectations while making the underlying mechanism real.

### 6. Lifecycle Events

Add finer lifecycle signals around runtime instances. First version should at least expose:

- queued
- running
- finished

Payloads should include:

- instance ID
- subagent type
- parent session ID
- child session ID
- final status
- final error text when present

Existing `SubagentStart` and `SubagentStop` hooks can be preserved as compatibility signals or mapped onto the richer lifecycle model.

### 7. Safety Limits

First version must also enforce:

- subagent depth limits
- per-session concurrent subagent limits
- parent/child tool whitelist intersections
- no unrestricted recursive subagent spawning

These safeguards are part of the runtime design, not optional polish.

## Package-Level Changes

- `pkg/runtime/subagents`
  - keep loader and manager
  - add types for instance status and runtime requests/results
  - add in-memory instance store
  - add executor
- `pkg/api`
  - construct and own subagent profiles, store, and executor
  - implement the subagent runner callback
  - route `Task` through spawn/wait
- `pkg/tool/builtin/task.go`
  - keep schema stable
  - correct behavioral description to match synchronous wrapping over real instance execution
- `pkg/core/events`
  - add or extend subagent lifecycle payloads

## Migration Strategy

### Stage 1

- add new subagent runtime types, store, and executor
- keep existing manager dispatch path intact

### Stage 2

- add runtime-backed subagent execution path in `pkg/api`
- update `Task` to call spawn/wait internally

### Stage 3

- change markdown body semantics from direct output to subagent instructions
- update tests and docs accordingly

### Stage 4

- deprecate direct synchronous `Manager.Dispatch` as the primary integration point

## Testing Strategy

- loader tests proving profile instructions are preserved but not directly returned as final output
- executor tests covering queued/running/completed and failed transitions
- wait tests covering already-finished, in-flight, and timeout behavior
- runtime integration tests proving `Task` still returns synchronously while instances are created
- model/tool inheritance tests proving child restrictions are correct
- concurrency tests around store updates and per-session limits

## Explicit Non-Goals

- no public `spawn_agent` or `wait_agent` tools in this change
- no interactive `send_input`, `resume`, or `close` collaboration surface yet
- no persistent on-disk instance store in the first version
