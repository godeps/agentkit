# CLI Govm Defaults Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add first-class `govm` sandbox selection to the CLI with built-in default mounts, session workspace creation, and clear preflight errors for unsupported or unavailable environments.

**Architecture:** Extend `cmd/cli/main.go` with a small set of sandbox flags, derive `api.SandboxOptions` in one helper, and validate `govm` availability before runtime startup. Keep defaults opinionated: project mounted at `/workspace/project`, session workspace at `/workspace`, network disabled, and explicit failure instead of silent fallback.

**Tech Stack:** Go 1.24, `cmd/cli`, `pkg/api` sandbox options, `github.com/godeps/govm/pkg/client`, Go tests.

---

### Task 1: Lock CLI sandbox defaults with failing tests

**Files:**
- Modify: `cmd/cli/main_test.go`

**Step 1: Write the failing test**

Add tests covering:
- `--sandbox-backend=govm` builds `api.SandboxOptions` with default `RuntimeHome`, `OfflineImage`, project mount, and auto session workspace
- `--sandbox-project-mount=off` removes the project mount
- invalid `--sandbox-project-mount` values fail before runtime creation

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli -run 'TestRunBuildsGovmSandboxOptions|TestRunGovmProjectMountOff|TestRunRejectsInvalidSandboxProjectMount'`

Expected: FAIL because the CLI does not parse or apply these flags yet.

**Step 3: Write minimal implementation**

Add CLI flag parsing and sandbox option builder helpers.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/cli -run 'TestRunBuildsGovmSandboxOptions|TestRunGovmProjectMountOff|TestRunRejectsInvalidSandboxProjectMount'`

Expected: PASS

**Step 5: Commit**

```bash
git add cmd/cli/main.go cmd/cli/main_test.go
git commit -m "feat: add cli govm sandbox defaults"
```

### Task 2: Add govm preflight validation

**Files:**
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/main_test.go`

**Step 1: Write the failing test**

Add tests covering:
- unsupported platform returns a clear error when `govm` is selected
- native runtime unavailable returns a clear error before agent execution

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli -run 'TestRunRejectsUnsupportedGovmPlatform|TestRunRejectsUnavailableGovmRuntime'`

Expected: FAIL because no govm preflight validation exists yet.

**Step 3: Write minimal implementation**

Add a small preflight helper that checks supported OS/arch and verifies `govm` runtime availability.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/cli -run 'TestRunRejectsUnsupportedGovmPlatform|TestRunRejectsUnavailableGovmRuntime'`

Expected: PASS

**Step 5: Commit**

```bash
git add cmd/cli/main.go cmd/cli/main_test.go
git commit -m "feat: validate cli govm availability"
```

### Task 3: Run full CLI verification

**Files:**
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/main_test.go`

**Step 1: Run test suite**

Run: `go test ./cmd/cli/...`

Expected: PASS

**Step 2: Run help output check**

Run: `go run ./cmd/cli --help`

Expected: new sandbox flags appear in help output.

**Step 3: Run manual govm startup check**

Run: `go run ./cmd/cli --sandbox-backend=govm --prompt "test"`

Expected: either a clear govm preflight failure on this environment or a runtime startup that uses the govm backend.

**Step 4: Commit**

```bash
git add docs/plans/2026-03-13-cli-govm-defaults.md
git commit -m "docs: add cli govm defaults plan"
```
