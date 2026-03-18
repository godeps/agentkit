# Agentkit Runtime Optimization Roadmap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Strengthen `agentkit` around its highest-value differentiator as a Go agent runtime kernel by prioritizing runtime execution unification, sandbox/security hardening, subagent/task semantics, and productized observability and documentation.

**Architecture:** The roadmap treats `agentkit` as a runtime-first system, not a generic AI SDK. Work should converge around one explicit execution chain: request intake, policy and sandbox enforcement, hook and middleware interception, model and tool execution, MCP integration, subagent or task dispatch, and trace or audit emission. Each phase should reduce ambiguity between packages and tighten public API and runtime semantics.

**Tech Stack:** Go, Go test, race detector, OTEL integration, Markdown docs, existing `pkg/api`, `pkg/sandbox`, `pkg/security`, `pkg/runtime/*`, `pkg/core/hooks`, `pkg/mcp`

---

## Priority Summary

### P0

- Unify runtime execution lifecycle and context model
- Productize sandbox and security as a first-class boundary
- Unify subagent and task dispatch semantics
- Rewrite project positioning around `runtime kernel`

### P1

- Expand observability, auditability, and policy logs
- Stabilize public API and configuration boundaries
- Add production embedding examples

### P2

- Add richer policy engine and approval flows
- Add stronger persistence, replay, and extension model
- Extend provider and model integration after runtime core is stable

### Task 1: Unify Runtime Execution Lifecycle

**Files:**
- Modify: `pkg/api/agent.go`
- Modify: `pkg/agent/*`
- Modify: `pkg/core/events/*`
- Modify: `pkg/core/hooks/*`
- Modify: `pkg/middleware/*`
- Modify: `docs/architecture.md`
- Create: `docs/plans/2026-03-11-agentkit-runtime-lifecycle-design.md`
- Test: `test/integration/*`
- Test: `pkg/api/*_test.go`

**Step 1: Document the current execution chain**

Run: `rg -n "Run\\(|RunStream\\(|BeforeAgent|AfterAgent|before_model|after_model|tool_execution|subagent|task" pkg test docs`
Expected: The current lifecycle touchpoints are enumerated.

**Step 2: Write a lifecycle design doc**

Document one canonical chain:

`Request -> Normalization -> Policy/Sandbox -> Hooks/Middleware -> Model -> Tool/MCP -> Subagent/Task -> Trace/Audit -> Response`

Include:

- stage ownership
- data passed between stages
- cancellation and timeout propagation
- error propagation rules
- trace propagation rules

**Step 3: Add failing integration tests for lifecycle ordering**

Add tests that assert:

- hooks run in documented order
- middleware observes consistent state
- tool and model spans inherit the same request context
- subagent or task dispatch receives the correct request-scoped metadata

**Step 4: Implement minimal lifecycle normalization**

Refactor runtime code so the ordering is explicit and stable. Avoid duplicate stage semantics across events, hooks, and middleware.

**Step 5: Run focused tests**

Run: `go test ./pkg/api ./pkg/core/hooks ./pkg/middleware ./test/integration/...`
Expected: PASS

**Step 6: Run race verification**

Run: `go test -race ./pkg/api ./pkg/core/hooks ./pkg/middleware ./test/integration/...`
Expected: PASS

**Step 7: Commit**

```bash
git add pkg/api pkg/agent pkg/core/events pkg/core/hooks pkg/middleware docs/architecture.md docs/plans/2026-03-11-agentkit-runtime-lifecycle-design.md test/integration
git commit -m "refactor: unify runtime lifecycle semantics"
```

### Task 2: Productize Sandbox And Security

**Files:**
- Modify: `pkg/sandbox/*`
- Modify: `pkg/security/*`
- Modify: `pkg/api/options*.go`
- Modify: `docs/security.md`
- Create: `docs/plans/2026-03-11-agentkit-sandbox-policy-design.md`
- Test: `pkg/sandbox/*_test.go`
- Test: `pkg/security/*_test.go`
- Test: `test/integration/*`

**Step 1: Enumerate current policy surfaces**

Run: `rg -n "sandbox|approval|policy|whitelist|filesystem|network|path|command" pkg/sandbox pkg/security pkg/api test`
Expected: Current policy knobs and enforcement sites are listed.

**Step 2: Write a policy precedence design**

Define exact precedence:

- global defaults
- project configuration
- session-level overrides
- request-level overrides

Define exact subjects:

- filesystem
- network
- command execution
- path access
- approval state

**Step 3: Add failing tests for precedence and denial behavior**

Include cases for:

- allow at project level, deny at request level
- deny by default with explicit allow
- expired approvals
- path traversal or invalid path handling
- command invocation under mismatched policy

**Step 4: Implement policy normalization and enforcement**

Refactor config and enforcement code so every sandbox decision is derived from the same merged policy object.

**Step 5: Add audit-friendly denial outputs**

Make denials machine-readable enough for tracing, logs, and callers to understand why access was rejected.

**Step 6: Run focused tests**

Run: `go test ./pkg/sandbox ./pkg/security ./test/integration/...`
Expected: PASS

**Step 7: Run race verification**

Run: `go test -race ./pkg/sandbox ./pkg/security ./test/integration/...`
Expected: PASS

**Step 8: Commit**

```bash
git add pkg/sandbox pkg/security pkg/api docs/security.md docs/plans/2026-03-11-agentkit-sandbox-policy-design.md test/integration
git commit -m "feat: unify sandbox and security policy enforcement"
```

### Task 3: Unify Subagent And Task Dispatch Semantics

**Files:**
- Modify: `pkg/runtime/subagents/*`
- Modify: `pkg/runtime/tasks/*`
- Modify: `pkg/tool/*`
- Modify: `pkg/api/*`
- Create: `docs/plans/2026-03-11-agentkit-subagent-task-dispatch-design.md`
- Test: `pkg/runtime/subagents/*_test.go`
- Test: `pkg/runtime/tasks/*_test.go`
- Test: `test/runtime/subagents/*`

**Step 1: Document current dispatch modes**

Run: `rg -n "subagent|dispatch|task|dependency|TargetSubagent|taskTool" pkg/runtime pkg/api test`
Expected: All subagent and task entrypoints are enumerated.

**Step 2: Write a boundary design**

Specify:

- when a subagent call is synchronous vs task-backed
- what metadata is inherited
- what state is isolated
- how cancellation bubbles
- how failures surface to the caller
- how task dependencies interact with subagent work

**Step 3: Add failing tests for dispatch consistency**

Add tests that assert:

- subagent dispatch inherits request metadata correctly
- task-backed work reports deterministic state transitions
- cancellation interrupts both task and subagent execution
- error surfaces are stable and documented

**Step 4: Implement minimal semantic unification**

Refactor shared code so subagent and task dispatch use aligned state, identifiers, and event emission.

**Step 5: Run focused tests**

Run: `go test ./pkg/runtime/subagents ./pkg/runtime/tasks ./test/runtime/subagents/...`
Expected: PASS

**Step 6: Run race verification**

Run: `go test -race ./pkg/runtime/subagents ./pkg/runtime/tasks ./test/runtime/subagents/...`
Expected: PASS

**Step 7: Commit**

```bash
git add pkg/runtime/subagents pkg/runtime/tasks pkg/tool pkg/api docs/plans/2026-03-11-agentkit-subagent-task-dispatch-design.md test/runtime/subagents
git commit -m "refactor: align subagent and task dispatch semantics"
```

### Task 4: Rewrite Positioning And Public Docs

**Files:**
- Modify: `README.md`
- Modify: `README_zh.md`
- Modify: `docs/architecture.md`
- Modify: `docs/getting-started.md`
- Create: `docs/2026-03-11-agentkit-positioning-notes.md`

**Step 1: Extract the new positioning statement**

Define the shortest accurate statement:

`agentkit is a Go agent runtime kernel for Claude Code-style execution environments.`

**Step 2: Rewrite top-level docs around runtime-first concepts**

Prioritize:

- runtime kernel
- sandbox and security
- hooks and middleware
- MCP runtime integration
- subagents and tasks

De-emphasize generic feature-list marketing.

**Step 3: Add one architecture diagram that matches runtime flow**

Ensure README and `docs/architecture.md` use the same terminology as the lifecycle design doc.

**Step 4: Verify docs consistency**

Run: `rg -n "Agent SDK|runtime kernel|sandbox|hooks|MCP|subagents|tasks" README.md README_zh.md docs/architecture.md docs/getting-started.md`
Expected: Updated terminology appears consistently.

**Step 5: Commit**

```bash
git add README.md README_zh.md docs/architecture.md docs/getting-started.md docs/2026-03-11-agentkit-positioning-notes.md
git commit -m "docs: reposition agentkit as a runtime kernel"
```

### Task 5: Expand Observability And Auditability

**Files:**
- Modify: `pkg/api/otel*.go`
- Modify: `pkg/core/events/*`
- Modify: `pkg/mcp/*`
- Modify: `pkg/tool/*`
- Modify: `pkg/runtime/subagents/*`
- Modify: `pkg/runtime/tasks/*`
- Create: `docs/plans/2026-03-11-agentkit-observability-roadmap.md`
- Modify: `docs/trace-system.md`
- Test: `pkg/api/*_test.go`
- Test: `test/integration/*`

**Step 1: Define the minimum trace and audit envelope**

Track:

- request ID
- session ID
- policy decision IDs
- model call metadata
- tool call metadata
- MCP server and tool metadata
- subagent IDs
- task IDs and dependency edges

**Step 2: Add failing tests for span and event propagation**

Add tests that assert the identifiers above survive across the full runtime chain.

**Step 3: Implement structured audit events**

Emit structured events for:

- policy deny or allow decisions
- approval usage
- tool invocation
- MCP tool invocation
- task state transitions
- subagent dispatch start and finish

**Step 4: Run focused tests**

Run: `go test ./pkg/api ./pkg/core/events ./pkg/mcp ./pkg/tool ./pkg/runtime/subagents ./pkg/runtime/tasks ./test/integration/...`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/api pkg/core/events pkg/mcp pkg/tool pkg/runtime/subagents pkg/runtime/tasks docs/trace-system.md docs/plans/2026-03-11-agentkit-observability-roadmap.md test/integration
git commit -m "feat: expand runtime observability and audit events"
```

### Task 6: Stabilize Public API And Configuration Surface

**Files:**
- Modify: `pkg/api/*`
- Modify: `pkg/config/*`
- Modify: `docs/api-reference.md`
- Modify: `docs/smart-defaults.md`
- Create: `docs/plans/2026-03-11-agentkit-api-stability-design.md`
- Test: `pkg/api/*_test.go`
- Test: `pkg/config/*_test.go`

**Step 1: Inventory public entrypoints**

Run: `rg -n "^type |^func " pkg/api pkg/config pkg/model pkg/tool | head -n 400`
Expected: Public API surface is enumerated for review.

**Step 2: Classify stable vs internal vs experimental**

Mark:

- stable public types
- internal-only behaviors
- experimental toggles or features

**Step 3: Add failing compatibility tests where possible**

Protect:

- `api.Options`
- `api.Request`
- structured output types
- config loading defaults
- runtime constructor behavior

**Step 4: Update docs to match the classified surface**

Ensure public docs stop implying stability where there is none.

**Step 5: Run focused tests**

Run: `go test ./pkg/api ./pkg/config`
Expected: PASS

**Step 6: Commit**

```bash
git add pkg/api pkg/config docs/api-reference.md docs/smart-defaults.md docs/plans/2026-03-11-agentkit-api-stability-design.md
git commit -m "docs: define api stability and config boundaries"
```

### Task 7: Add Production Embedding Examples

**Files:**
- Modify: `examples/02-cli/*`
- Modify: `examples/03-http/*`
- Create: `examples/13-runtime-kernel/*`
- Modify: `examples/README.md`
- Modify: `examples/README_zh.md`

**Step 1: Define three production-focused example narratives**

Include:

- embedded CLI assistant
- HTTP/SSE hosted runtime
- policy-constrained runtime with MCP and tracing enabled

**Step 2: Add or update examples**

Each example should demonstrate:

- runtime construction
- explicit session usage
- policy or sandbox configuration
- hooks or middleware usage
- tool or MCP integration

**Step 3: Verify examples build**

Run: `go test ./examples/...`
Expected: PASS

**Step 4: Commit**

```bash
git add examples
git commit -m "docs: add production embedding examples"
```

### Task 8: P2 Backlog Definition

**Files:**
- Create: `docs/plans/2026-03-11-agentkit-p2-backlog.md`

**Step 1: Record deferred work explicitly**

Include:

- richer policy engine
- approval workflows
- session persistence and replay
- extension or plugin model
- provider breadth after runtime core is stable

**Step 2: Define acceptance criteria for entering implementation**

Each P2 item must specify:

- why P0 or P1 is insufficient
- what production use case requires it
- what API stability risk it introduces

**Step 3: Commit**

```bash
git add docs/plans/2026-03-11-agentkit-p2-backlog.md
git commit -m "docs: capture p2 runtime backlog"
```

## Verification Gate Before Declaring Any Phase Done

For every implementation task above, run the narrowest relevant package tests first, then at least one broad verification pass:

Run: `go test ./...`
Expected: PASS

Run: `go test -race ./...`
Expected: PASS or documented exclusions if too expensive for routine execution

If docs are changed materially, also verify:

Run: `rg -n "runtime kernel|sandbox|hooks|MCP|subagents|tasks" README.md README_zh.md docs`
Expected: Terminology is consistent and intentional.

## Recommended Execution Order

1. Task 1: Unify Runtime Execution Lifecycle
2. Task 2: Productize Sandbox And Security
3. Task 3: Unify Subagent And Task Dispatch Semantics
4. Task 4: Rewrite Positioning And Public Docs
5. Task 5: Expand Observability And Auditability
6. Task 6: Stabilize Public API And Configuration Surface
7. Task 7: Add Production Embedding Examples
8. Task 8: P2 Backlog Definition
