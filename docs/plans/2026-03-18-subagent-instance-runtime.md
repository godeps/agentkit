# Subagent Instance Runtime Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Introduce real subagent instance lifecycle management behind the existing synchronous `Task` tool.

**Architecture:** Keep subagent definitions as profiles, add an in-memory instance store plus executor, and route `Task` through `spawn + wait` while `pkg/api` provides the actual child-agent runner. Preserve the current external `Task` shape and add lifecycle observability and safety guards as part of the runtime.

**Tech Stack:** Go, `pkg/api`, `pkg/runtime/subagents`, `pkg/tool/builtin`, `pkg/core/events`, Go testing

---

### Task 1: Add failing tests for subagent instance primitives

**Files:**
- Modify: `pkg/runtime/subagents/manager_test.go`
- Create: `pkg/runtime/subagents/executor_test.go`
- Create: `pkg/runtime/subagents/store_test.go`

**Step 1: Write the failing tests**

- Add tests for:
  - instance store create/get/update
  - executor `Spawn` creating a queued instance and eventually completing it
  - executor `Wait` timing out or returning immediately for a finished instance
  - missing/unknown target behavior still returning selection errors

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/runtime/subagents -run 'TestMemoryStore|TestExecutor'`

Expected: FAIL with missing types or missing executor/store behavior.

**Step 3: Write minimal implementation**

- Add `types.go`, `store.go`, and `executor.go` under `pkg/runtime/subagents`
- Keep implementation in-memory and minimal

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/runtime/subagents -run 'TestMemoryStore|TestExecutor'`

Expected: PASS

### Task 2: Refactor manager/loader to separate profiles from execution

**Files:**
- Modify: `pkg/runtime/subagents/manager.go`
- Modify: `pkg/runtime/subagents/loader.go`
- Modify: `pkg/runtime/subagents/loader_test.go`

**Step 1: Write the failing tests**

- Add tests proving:
  - loader preserves profile instructions in metadata or profile state
  - direct manager selection still works
  - task-facing output is no longer just raw markdown body

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/runtime/subagents -run 'TestLoad|TestManager'`

Expected: FAIL because instructions are still returned directly and manager still owns dispatch.

**Step 3: Write minimal implementation**

- Move manager responsibility to registration and selection
- Keep a compatibility dispatch wrapper only if needed by existing tests
- Preserve body as instructions instead of direct result text

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/runtime/subagents -run 'TestLoad|TestManager'`

Expected: PASS

### Task 3: Add failing runtime integration tests for Task -> spawn/wait

**Files:**
- Modify: `pkg/api/subagents_additional_test.go`
- Modify: `pkg/api/agent_test.go`
- Modify: `pkg/api/runtime_helpers_merge_additional_test.go`

**Step 1: Write the failing tests**

- Add tests proving:
  - `Task` still returns synchronously
  - `Task` creates a subagent instance ID internally
  - runtime child execution respects tool/model restrictions
  - failures are reflected in final tool result and instance state

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestTask|TestSubagent'`

Expected: FAIL because runtime still dispatches directly without instance tracking.

**Step 3: Write minimal implementation**

- Extend `Runtime` to own subagent profiles, store, and executor
- Add internal runner adapter and spawn/wait helpers

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestTask|TestSubagent'`

Expected: PASS

### Task 4: Update Task behavior and lifecycle events

**Files:**
- Modify: `pkg/tool/builtin/task.go`
- Modify: `pkg/core/events/types.go`
- Modify: `pkg/api/options.go`
- Modify: `pkg/api/agent.go`

**Step 1: Write the failing tests**

- Add tests for:
  - task description/docs reflecting synchronous wait over instance execution
  - lifecycle event payloads include instance ID and final status
  - hook adapter records subagent lifecycle transitions

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin ./pkg/core/events ./pkg/api -run 'TestTask|TestSubagent'`

Expected: FAIL because lifecycle payloads and task text do not match the new runtime.

**Step 3: Write minimal implementation**

- Update task description text to remove subprocess overclaim
- Add event payload fields/types needed by the runtime
- Emit lifecycle events from spawn/start/finish transitions

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin ./pkg/core/events ./pkg/api -run 'TestTask|TestSubagent'`

Expected: PASS

### Task 5: Full verification and integration pass

**Files:**
- Verify: `pkg/runtime/subagents/*`
- Verify: `pkg/api/*`
- Verify: `pkg/tool/builtin/*`
- Verify: `pkg/core/events/*`

**Step 1: Run focused package verification**

Run: `go test ./pkg/runtime/subagents ./pkg/api ./pkg/tool/builtin ./pkg/core/events`

Expected: PASS

**Step 2: Run race or broader integration verification**

Run: `go test ./test/integration/...`

Expected: PASS, or any unrelated failure is identified explicitly before commit.

**Step 3: Commit**

```bash
git add pkg/runtime/subagents pkg/api pkg/tool/builtin pkg/core/events docs/plans/2026-03-18-subagent-instance-runtime-design.md docs/plans/2026-03-18-subagent-instance-runtime.md
git commit -m "feat: add subagent instance runtime"
```
