# Agentkit Upstream Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Port the recent upstream `agentsdk-go` fixes and ACP additions into `agentkit` without overwriting `agentkit`-specific module and CLI customizations.

**Architecture:** Work on an isolated `agentkit` sync branch, cherry-pick upstream commits one at a time, adapt module/import paths from `github.com/godeps/agentsdk-go` to `github.com/godeps/agentkit`, and validate each step with focused tests before moving to the next upstream change. Restore the user's stashed local edits only after the sync branch is stable.

**Tech Stack:** Go, git cherry-pick, go test

---

### Task 1: Port OpenAI argument encoding fix

**Files:**
- Modify: `pkg/model/openai.go`
- Modify: `pkg/model/openai_test.go`
- Create or modify: `pkg/model/openai_e2e_test.go`
- Modify: `pkg/middleware/trace_additional_test.go`

**Step 1: Write or import the upstream failing tests**

Add the upstream regression test coverage for OpenAI tool call arguments JSON encoding.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/model -run 'OpenAI|ToolCall'`

**Step 3: Write minimal implementation**

Port the upstream encoding fix into `pkg/model/openai.go` and adapt any `agentkit`-specific differences.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/model -run 'OpenAI|ToolCall'`

**Step 5: Commit**

Commit the isolated sync change after verification.

### Task 2: Port message trimmer accounting fix

**Files:**
- Modify: `pkg/message/trimmer.go`
- Modify: `pkg/message/trimmer_test.go`

**Step 1: Write or import the upstream failing test**

Add the upstream regression coverage for counting reasoning and tool result content.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/message -run Trimmer`

**Step 3: Write minimal implementation**

Port the upstream trimmer fix with no extra behavior changes.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/message -run Trimmer`

**Step 5: Commit**

Commit the isolated sync change after verification.

### Task 3: Port bash stream timeout fix

**Files:**
- Modify: `pkg/tool/builtin/bash_stream.go`
- Modify: `pkg/tool/builtin/bash_stream_test.go`

**Step 1: Write or import the upstream failing tests**

Add the upstream timeout and cancellation regression coverage for `StreamExecute`.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run BashStream`

**Step 3: Write minimal implementation**

Port the upstream timeout handling fix and keep existing `agentkit` behavior intact.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run BashStream`

**Step 5: Commit**

Commit the isolated sync change after verification.

### Task 4: Evaluate ACP adapter port

**Files:**
- Modify: `cmd/cli/main.go`
- Create or modify: `cmd/cli/main_test.go`
- Create or modify: `docs/acp-integration.md`
- Create or modify: `pkg/acp/*.go`
- Create or modify: `pkg/api/runtime_helpers_*.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Write or import the upstream tests**

Bring over the ACP-focused tests and any runtime helper tests first.

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli ./pkg/acp ./pkg/api`

**Step 3: Write minimal implementation**

Port ACP files and CLI wiring, rewriting imports to `github.com/godeps/agentkit/...` and preserving any `agentkit`-specific CLI surface already present.

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/cli ./pkg/acp ./pkg/api`

**Step 5: Commit**

Commit the isolated sync change after verification.

### Task 5: Restore local work and validate

**Files:**
- Modify: stash restore results if conflicts appear

**Step 1: Restore the saved stash carefully**

Run: `git stash apply stash^{/pre-sync-agentkit-local-changes-2026-03-09}`

**Step 2: Resolve restore conflicts if any**

Keep upstream sync changes and reapply user-local edits intentionally.

**Step 3: Run full verification**

Run: `go test ./...`

**Step 4: Commit or leave restored changes unstaged**

Only commit the sync work. Leave restored user-local edits uncommitted unless explicitly requested.
