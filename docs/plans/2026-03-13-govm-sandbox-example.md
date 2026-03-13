# Govm Sandbox Example Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a standalone `examples/13-govm-sandbox` example that demonstrates govm sandbox mounts, read-only vs read-write permissions, and automatic per-session workspace creation with host-visible outputs.

**Architecture:** Build a dedicated example package with a small `main.go` entrypoint and focused helpers for sandbox configuration, demo execution, and output reporting. Keep the example offline and deterministic by interacting with the govm-backed execution environment directly instead of relying on an external model provider.

**Tech Stack:** Go 1.24, `pkg/sandbox/govmenv`, `pkg/api` sandbox option types, Go tests, markdown docs.

---

### Task 1: Add a failing example test

**Files:**
- Create: `examples/13-govm-sandbox/demo_test.go`

**Step 1: Write the failing test**

Add a test that expects:
- `/inputs/policy.txt` is readable from a readonly mount
- writing `/inputs/blocked.txt` is rejected
- writing `/shared/result.txt` succeeds and is visible on host
- writing `/workspace/session-note.txt` succeeds in auto-created `workspace/<session-id>`

**Step 2: Run test to verify it fails**

Run: `go test ./examples/13-govm-sandbox`

Expected: FAIL because the example package and helper functions do not exist yet.

**Step 3: Write minimal implementation**

Create the example package and enough helper code to make the test pass with a real govm environment.

**Step 4: Run test to verify it passes**

Run: `go test ./examples/13-govm-sandbox`

Expected: PASS

**Step 5: Commit**

```bash
git add examples/13-govm-sandbox docs/plans/2026-03-13-govm-sandbox-example.md
git commit -m "feat: add govm sandbox example"
```

### Task 2: Add runnable example entrypoint and report output

**Files:**
- Create: `examples/13-govm-sandbox/main.go`
- Create: `examples/13-govm-sandbox/sandbox.go`
- Create: `examples/13-govm-sandbox/demo.go`
- Create: `examples/13-govm-sandbox/report.go`
- Create: `examples/13-govm-sandbox/testdata/readonly/policy.txt`
- Create: `examples/13-govm-sandbox/testdata/shared/.gitkeep`

**Step 1: Write the failing test**

Extend the example test to assert the report contains all expected step statuses and host path summaries.

**Step 2: Run test to verify it fails**

Run: `go test ./examples/13-govm-sandbox -run TestRunDemo`

Expected: FAIL because report formatting is not complete yet.

**Step 3: Write minimal implementation**

Implement the fixed demo flow and concise report formatter.

**Step 4: Run test to verify it passes**

Run: `go test ./examples/13-govm-sandbox -run TestRunDemo`

Expected: PASS

**Step 5: Commit**

```bash
git add examples/13-govm-sandbox
git commit -m "feat: add runnable govm sandbox demo"
```

### Task 3: Wire docs and verify end-to-end

**Files:**
- Modify: `examples/README.md`
- Create: `examples/13-govm-sandbox/README.md`

**Step 1: Write the failing test**

Add or extend a doc-oriented smoke check if needed, otherwise use the existing example test as the behavioral gate and verify the README command manually.

**Step 2: Run verification**

Run:
- `go test ./examples/13-govm-sandbox`
- `go run ./examples/13-govm-sandbox`

Expected: tests pass; example prints readonly/readwrite/workspace results and host output paths.

**Step 3: Write minimal documentation**

Document prerequisites, command, and expected host output files.

**Step 4: Re-run verification**

Run:
- `go test ./examples/13-govm-sandbox`
- `go run ./examples/13-govm-sandbox`

Expected: same successful result.

**Step 5: Commit**

```bash
git add examples/README.md examples/13-govm-sandbox/README.md
git commit -m "docs: document govm sandbox example"
```
