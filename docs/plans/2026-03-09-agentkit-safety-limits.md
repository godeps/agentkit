# Agentkit Safety Limits Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove silent truncation and hidden-output pitfalls in Agentkit while making compacting limits configurable.

**Architecture:** Keep the current runtime shape intact and patch the unsafe defaults in place. Prefer additive configuration and behavior-local changes over broad tool-constructor refactors so the regression surface stays manageable.

**Tech Stack:** Go, standard library testing, existing Agentkit tool/runtime packages.

---

### Task 1: Fix `file_read` silent truncation

**Files:**
- Modify: `pkg/tool/builtin/read.go`
- Test: `pkg/tool/builtin/read_test.go`

**Step 1: Write the failing test**

- Add a test that reads a file containing one line far longer than 2000 bytes.
- Assert the returned output contains the original line content until the new total-output limit is reached, not the current `...(truncated)` per-line suffix.
- Assert metadata clearly marks output truncation.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run Read`

Expected: FAIL because the current implementation still truncates per line.

**Step 3: Write minimal implementation**

- Replace per-line truncation with total formatted-output budget enforcement.
- Add explicit cutoff notice at the end of the output.
- Add metadata fields for output-budget truncation.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run Read`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/tool/builtin/read.go pkg/tool/builtin/read_test.go
git commit -m "fix: remove silent file_read line truncation"
```

### Task 2: Let `CustomTools` override built-ins

**Files:**
- Modify: `pkg/api/agent.go`
- Test: `pkg/api/helpers_test.go`

**Step 1: Write the failing test**

- Add a test registering a custom tool whose canonical name matches a built-in.
- Assert the custom implementation is the one registered and the built-in is not.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run RegisterTools`

Expected: FAIL because duplicate names are currently skipped after the built-in wins.

**Step 3: Write minimal implementation**

- Track whether a tool originated from built-ins or `CustomTools`.
- On canonical-name collision, prefer the custom tool.
- Keep `Options.Tools` precedence unchanged.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run RegisterTools`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/agent.go pkg/api/helpers_test.go
git commit -m "fix: allow custom tools to override built-ins"
```

### Task 3: Wire `settings.ToolOutput` into the executor

**Files:**
- Modify: `pkg/api/agent.go`
- Modify: `pkg/tool/persister.go`
- Test: `pkg/tool/persister_test.go`
- Test: `pkg/api/helpers_test.go`

**Step 1: Write the failing tests**

- Add a test proving `settings.ToolOutput.defaultThresholdBytes` changes persistence behavior.
- Add a test proving per-tool thresholds override the default threshold.
- Add a test proving persisted output returns a visible summary instead of only a bare path marker.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/tool ./pkg/api -run 'Persister|ToolOutput|RegisterTools'`

Expected: FAIL because the executor still uses the default persister and the persisted message is path-only.

**Step 3: Write minimal implementation**

- Create the executor persister from settings when present.
- Keep default behavior when settings are absent.
- Expand persisted output text to include summary bytes and persisted path.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/tool ./pkg/api -run 'Persister|ToolOutput|RegisterTools'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/agent.go pkg/tool/persister.go pkg/tool/persister_test.go pkg/api/helpers_test.go
git commit -m "fix: apply tool output persistence settings"
```

### Task 4: Make AutoCompact limits configurable

**Files:**
- Modify: `pkg/api/compact.go`
- Modify: `docs/api-reference.md`
- Test: `pkg/api/compact_test.go`

**Step 1: Write the failing tests**

- Add a test that sets an explicit compact context limit and verifies compaction decisions use it.
- Add a test that sets `SummaryMaxTokens` and verifies the summary request uses the configured value.

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/api -run Compact`

Expected: FAIL because the code still uses hardcoded fallback values.

**Step 3: Write minimal implementation**

- Extend `CompactConfig` with optional fields for context limit and summary token budget.
- Use configured values when provided; preserve existing defaults otherwise.
- Update API docs to reflect the new fields.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/api -run Compact`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/compact.go pkg/api/compact_test.go docs/api-reference.md
git commit -m "feat: configure compact context and summary budgets"
```

### Task 5: Run final regression verification

**Files:**
- Review: `pkg/tool/builtin/read.go`
- Review: `pkg/api/agent.go`
- Review: `pkg/tool/persister.go`
- Review: `pkg/api/compact.go`

**Step 1: Run targeted package tests**

Run: `go test ./pkg/tool/builtin ./pkg/tool ./pkg/api`

Expected: PASS.

**Step 2: Run a broader regression check**

Run: `go test ./...`

Expected: PASS, or a clearly documented list of unrelated pre-existing failures.

**Step 3: Review the diff**

Run: `git diff --stat HEAD~4..HEAD`

Expected: only the planned files and doc updates are touched.

**Step 4: Commit any final doc or naming cleanup**

```bash
git add -A
git commit -m "chore: finalize safety limits rollout"
```
