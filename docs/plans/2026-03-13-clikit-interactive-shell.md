# Clikit Interactive Shell Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a higher-level interactive shell in `pkg/clikit` so `cmd/cli --repl` behaves more like a Claude Code style long-running console with status output, slash commands, and resilient multi-turn interaction.

**Architecture:** Build a small `InteractiveShell` wrapper above the existing `RunREPL` / `RunStream` primitives, then switch the CLI to call that wrapper. Keep the UI text-based, not full-screen, and centralize session state, status rendering, and command handling in `pkg/clikit`.

**Tech Stack:** Go 1.24, `pkg/clikit`, `cmd/cli`, existing runtime adapter and stream renderer, Go tests.

---

### Task 1: Add failing shell behavior tests

**Files:**
- Modify: `pkg/clikit/repl_test.go`

**Step 1: Write the failing test**

Add tests proving:
- interactive shell prints a status header with session/model/repo context
- slash commands still work in the shell loop
- a stream failure prints an error and continues to the next input instead of exiting

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/clikit -run 'TestInteractiveShell'`

Expected: FAIL because no interactive shell abstraction exists yet.

**Step 3: Write minimal implementation**

Create a shell type and route the existing REPL through it.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/clikit -run 'TestInteractiveShell'`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/clikit/repl.go pkg/clikit/repl_test.go
git commit -m "feat: add clikit interactive shell"
```

### Task 2: Wire cmd/cli to the new shell

**Files:**
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/main_test.go`

**Step 1: Write the failing test**

Add a CLI test proving `--repl` invokes the higher-level clikit shell entrypoint instead of the old direct REPL runner.

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli -run TestCLIReplUsesInteractiveShell`

Expected: FAIL because `cmd/cli` still calls the old REPL function.

**Step 3: Write minimal implementation**

Route CLI REPL mode through the new clikit shell function.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/cli -run TestCLIReplUsesInteractiveShell`

Expected: PASS

**Step 5: Commit**

```bash
git add cmd/cli/main.go cmd/cli/main_test.go
git commit -m "feat: wire cli repl through interactive shell"
```

### Task 3: Verify shell and CLI behavior end-to-end

**Files:**
- Modify: `pkg/clikit/repl.go`
- Modify: `pkg/clikit/repl_test.go`
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/main_test.go`

**Step 1: Run targeted tests**

Run:
- `go test ./pkg/clikit`
- `go test ./cmd/cli/...`

Expected: PASS

**Step 2: Run manual CLI REPL check**

Run:

```bash
printf '/help\n/quit\n' | go run ./cmd/cli --repl
```

Expected: banner + status header + help output + clean exit.

**Step 3: Commit**

```bash
git add docs/plans/2026-03-13-clikit-interactive-shell.md
git commit -m "docs: add clikit interactive shell plan"
```
