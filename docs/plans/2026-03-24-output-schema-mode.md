# Output Schema Mode Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an opt-in `OutputSchemaMode` so agent runs can defer structured-output formatting to a post-processing pass without changing the default inline behavior.

**Architecture:** Thread a new mode through `api.Options` and `api.Request`, keep `inline` as the compatibility default, and add a single formatting pass after the agent loop for `post_process`. The first version only formats the final text output and falls back to the original result on formatting failure.

**Tech Stack:** Go, `pkg/api`, existing `model.Model` / `model.ResponseFormat` abstractions, Go test suite.

---

### Task 1: Add failing tests for mode plumbing and post-process behavior

**Files:**
- Modify: `pkg/api/agent_test.go`
- Modify: `pkg/api/options_frozen_coverage_test.go`

**Step 1: Write failing tests**

- Add a compatibility test asserting default `inline` still sends `ResponseFormat` during the agent loop.
- Add a test asserting `post_process` suppresses inline `ResponseFormat` on the loop call and formats the final text in a second model call.
- Add a fast-path test asserting already-valid JSON does not trigger an extra formatting call.
- Add a fallback test asserting formatting-call failure leaves the original output intact.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/api -run 'TestRuntime(OutputSchema|PostProcess)'`

**Step 3: Implement minimal production changes**

No implementation in this task.

**Step 4: Re-run targeted tests after implementation**

Run the same command until green.

### Task 2: Add mode plumbing and post-process formatting

**Files:**
- Modify: `pkg/api/options.go`
- Modify: `pkg/api/agent.go`

**Step 1: Add `OutputSchemaMode`**

- Add a new type plus `inline` and `post_process` constants.
- Add fields to both `Options` and `Request`.
- Normalize/freeze/clone mode values as needed.

**Step 2: Implement effective mode selection**

- Resolve request override vs runtime default.
- Keep empty mode equivalent to `inline`.

**Step 3: Implement post-process formatting pass**

- Do not send `ResponseFormat` during the loop when mode is `post_process`.
- After the loop, if `OutputSchema` exists and final text is not already valid JSON, run one model formatting call with no tools.
- Merge usage from the formatting pass.
- If formatting fails, preserve the original result.

### Task 3: Verify and clean up

**Files:**
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/agent_test.go`
- Modify: `pkg/api/options_frozen_coverage_test.go`

**Step 1: Format**

Run: `gofmt -w pkg/api/agent.go pkg/api/agent_test.go pkg/api/options.go pkg/api/options_frozen_coverage_test.go`

**Step 2: Run targeted verification**

Run: `go test ./pkg/api`

**Step 3: Run full verification**

Run: `go test ./...`
