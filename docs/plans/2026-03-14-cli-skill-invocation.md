# CLI Skill Invocation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `/skill-name` and `$skill-name` prompt syntax that forces runtime skill execution in both interactive and non-interactive CLI flows.

**Architecture:** Keep CLI built-in commands in `pkg/clikit` unchanged for known commands, but treat unknown slash-prefixed input as normal prompt text. Parse prompt skill markers in the API prepare path with awareness of registered slash commands and skills, then populate `Request.ForceSkills` and strip markers before command/skill/model execution.

**Tech Stack:** Go, `pkg/api`, `pkg/clikit`, runtime skills registry, slash command executor, Go tests.

---

### Task 1: Lock parsing behavior with failing tests

**Files:**
- Modify: `pkg/api/request_helpers_additional_test.go`
- Modify: `pkg/clikit/repl_test.go`

**Step 1: Write the failing tests**

- Add parser coverage for:
  - `$ai-sdk $yt-dlp build this`
  - `/tag` remaining a slash command when the command exists
  - `/ai-sdk help` becoming a forced skill when the command does not exist
  - duplicate skill markers deduped in first-seen order
  - missing forced skill reported as an error
- Add REPL coverage showing `/help` is still handled locally while `/unknown hi` falls through to `RunStream(...)`.

**Step 2: Run tests to verify they fail**

Run:
```bash
go test ./pkg/api ./pkg/clikit -run 'TestExtractPromptSkillInvocations|TestInteractiveShellUnknownSlashInputFallsThrough' -count=1
```

Expected: FAIL because prompt skill parsing and REPL fallback are not implemented yet.

### Task 2: Implement unified prompt skill parsing

**Files:**
- Modify: `pkg/api/request_helpers.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/options.go`

**Step 1: Write minimal implementation**

- Add helper(s) in `pkg/api/request_helpers.go` to:
  - detect exact `$skill-name` and `/skill-name` tokens
  - preserve registered slash commands
  - collect forced skill names in stable order with dedupe
  - return cleaned prompt text plus missing skill names
- In `pkg/api/agent.go`, call the helper during `prepare(...)` before command execution.
- Merge parsed names into `Request.ForceSkills`.
- Reject missing forced skills with a clear `api: unknown skill ...` error.
- Allow requests that contain only forced skills to proceed into skill execution even if the cleaned prompt becomes empty.
- In `pkg/api/options.go`, clone `ForceSkills` during normalization.

**Step 2: Run tests to verify they pass**

Run:
```bash
go test ./pkg/api -run 'TestExtractPromptSkillInvocations|TestRequestHelperUtilities' -count=1
```

Expected: PASS.

### Task 3: Update interactive shell slash handling

**Files:**
- Modify: `pkg/clikit/repl.go`
- Modify: `pkg/clikit/repl_test.go`

**Step 1: Write minimal implementation**

- Change REPL command handling so only known built-in commands are intercepted.
- Unknown slash-prefixed input should continue through the normal streamed prompt path.

**Step 2: Run tests to verify they pass**

Run:
```bash
go test ./pkg/clikit -run 'TestHandleCommandListsSkills|TestInteractiveShellUnknownSlashInputFallsThrough' -count=1
```

Expected: PASS.

### Task 4: Verify end-to-end behavior

**Files:**
- Modify: `cmd/cli/main_test.go` if wiring coverage is needed

**Step 1: Run targeted verification**

Run:
```bash
go test ./pkg/api ./pkg/clikit ./cmd/cli/... -count=1
```

Run:
```bash
printf '/unknown hi\n/quit\n' | go run ./cmd/cli --repl
```

Run:
```bash
go run ./cmd/cli --prompt '$ai-sdk hello'
```

Expected: tests pass; REPL no longer prints `unknown command` for `/unknown hi`; forced skill syntax is accepted by the runtime.

### Task 5: Commit

```bash
git add docs/plans/2026-03-14-cli-skill-invocation.md pkg/api/request_helpers.go pkg/api/request_helpers_additional_test.go pkg/api/agent.go pkg/api/options.go pkg/clikit/repl.go pkg/clikit/repl_test.go
git commit -m "feat: support prompt skill invocation syntax"
```
