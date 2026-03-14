# govm Streaming Bash Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Teach `agentkit` to use the new `govm` streaming exec API so interactive CLI sessions can stream bash output inside the `govm` sandbox.

**Architecture:** Extend the sandbox execution abstraction with a streaming command interface, implement it in `govmenv`, and update `bash_stream.go` to prefer the streaming environment path for virtualized sandboxes instead of hard-failing. Keep host-mode streaming behavior unchanged.

**Tech Stack:** Go, `agentkit` sandbox execution environment interfaces, `govm v0.1.3`.

---

### Task 1: Add streaming command abstraction

**Files:**
- Modify: `pkg/sandbox/env/types.go`

Add:
- `CommandStreamCallbacks`
- `StreamingExecutionEnvironment`

### Task 2: Add failing streaming bash test

**Files:**
- Modify: `pkg/tool/builtin/bash_stream_test.go`

Add a test proving virtualized sandbox sessions can stream through an environment that implements the new interface.

### Task 3: Implement virtualized streaming path

**Files:**
- Modify: `pkg/tool/builtin/bash_stream.go`
- Modify: `pkg/sandbox/govmenv/environment.go`
- Create: `pkg/sandbox/govmenv/environment_test.go`

Implement:
- `govmenv.RunCommandStream(...)`
- virtualized path in `BashTool.StreamExecute(...)`

### Task 4: Verify with released govm

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

Upgrade to `github.com/godeps/govm v0.1.3` and verify:

- `go test ./pkg/tool/builtin ./pkg/sandbox/govmenv ./pkg/clikit ./cmd/cli/... -count=1`
- `printf '查看下当前系统是什么\n/quit\n' | go run -tags govm_native ./cmd/cli --sandbox-backend=govm --repl`
