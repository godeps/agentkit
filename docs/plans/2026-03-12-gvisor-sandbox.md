# GVisor Sandbox Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `gvisor` sandbox type with configurable mounts, a default per-session writable workspace, a single-binary self-exec helper mode, and a unified guest-path model across shell and file tools.

**Architecture:** Introduce a new `ExecutionEnvironment` abstraction beneath the runtime, keep the current sandbox manager as the policy layer, then implement `HostEnvironment` and `GVisorEnvironment`. Move `bash` and file tools to environment-driven behavior, and have gVisor command execution happen in a helper mode of the same `agentkit` binary using imported `gvisor.dev/gvisor/...` code.

**Tech Stack:** Go 1.24, `pkg/api`, `pkg/sandbox`, `pkg/tool/builtin`, `cmd/cli`, Go tests, `gvisor.dev/gvisor/...`

---

## Chosen Direction

This plan implements the approved design:

- single `agentkit` binary
- source dependency on gVisor
- main process `exec`s itself into hidden helper mode
- helper process runs gVisor runtime logic
- `bash`, `file_read`, `file_write`, `file_edit`, `glob`, and `grep` share one guest filesystem view

## Task 1: Extend Sandbox Configuration Types

**Files:**
- Modify: `pkg/api/options.go`
- Test: `pkg/api/options_test.go`
- Test: `pkg/api/options_additional_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestSandboxOptionsFreezeCopiesGVisorConfig(t *testing.T) {}
func TestSandboxOptionsDefaultsGVisorWorkspaceBase(t *testing.T) {}
func TestSandboxOptionsRejectsInvalidGuestMounts(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestSandboxOptionsFreezeCopiesGVisorConfig|TestSandboxOptionsDefaultsGVisorWorkspaceBase|TestSandboxOptionsRejectsInvalidGuestMounts'`

Expected: FAIL because gVisor config types and validation do not exist.

**Step 3: Write minimal implementation**

In `pkg/api/options.go`:

- add `SandboxOptions.Type`
- add `SandboxOptions.GVisor`
- add `GVisorOptions`
- add `MountSpec`
- add normalization / freeze logic for gVisor fields

Rules to implement:

- default helper flag: `--agentkit-gvisor-helper`
- default guest cwd: `/workspace`
- default session workspace base: `<projectRoot>/workspace`
- mount guest paths must be absolute and non-overlapping

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestSandboxOptionsFreezeCopiesGVisorConfig|TestSandboxOptionsDefaultsGVisorWorkspaceBase|TestSandboxOptionsRejectsInvalidGuestMounts'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/options.go pkg/api/options_test.go pkg/api/options_additional_test.go
git commit -m "feat: add gvisor sandbox configuration types"
```

## Task 2: Add Environment Abstraction

**Files:**
- Create: `pkg/sandbox/env/types.go`
- Create: `pkg/sandbox/hostenv/environment.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/sandbox_bridge.go`
- Test: `pkg/api/sandbox_bridge_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestBuildExecutionEnvironmentDefaultsToHost(t *testing.T) {}
func TestBuildExecutionEnvironmentSelectsGVisor(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestBuildExecutionEnvironmentDefaultsToHost|TestBuildExecutionEnvironmentSelectsGVisor'`

Expected: FAIL because the execution environment abstraction does not exist.

**Step 3: Write minimal implementation**

Create `pkg/sandbox/env/types.go` with:

- `ExecutionEnvironment`
- `SessionContext`
- `PreparedSession`
- `CommandRequest`
- `CommandResult`

Create `pkg/sandbox/hostenv/environment.go` with a host-native implementation mirroring current behavior.

In `pkg/api/agent.go` and `pkg/api/sandbox_bridge.go`:

- build an environment in runtime initialization
- attach it to `Runtime`
- keep current sandbox manager for policy checks

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestBuildExecutionEnvironmentDefaultsToHost|TestBuildExecutionEnvironmentSelectsGVisor'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/sandbox/env/types.go pkg/sandbox/hostenv/environment.go pkg/api/agent.go pkg/api/sandbox_bridge.go pkg/api/sandbox_bridge_test.go
git commit -m "feat: add execution environment abstraction"
```

## Task 3: Add Guest Path Mapping

**Files:**
- Create: `pkg/sandbox/pathmap/mapper.go`
- Create: `pkg/sandbox/pathmap/mapper_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestGuestToHostResolvesWritableMount(t *testing.T) {}
func TestGuestToHostRejectsUnmountedPath(t *testing.T) {}
func TestGuestToHostPreservesReadOnlyMetadata(t *testing.T) {}
func TestMountOverlapRejected(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/sandbox/pathmap -run 'TestGuestToHostResolvesWritableMount|TestGuestToHostRejectsUnmountedPath|TestGuestToHostPreservesReadOnlyMetadata|TestMountOverlapRejected'`

Expected: FAIL because the path mapper does not exist.

**Step 3: Write minimal implementation**

Create `pkg/sandbox/pathmap/mapper.go` with:

- mount normalization
- overlap validation
- `GuestToHost`
- `HostToGuest`
- `VisibleRoots`

Guest-path rules:

- only absolute guest paths
- no `..` escape
- longest-prefix mount match
- return mount metadata for permission checks

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/sandbox/pathmap -run 'TestGuestToHostResolvesWritableMount|TestGuestToHostRejectsUnmountedPath|TestGuestToHostPreservesReadOnlyMetadata|TestMountOverlapRejected'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/sandbox/pathmap/mapper.go pkg/sandbox/pathmap/mapper_test.go
git commit -m "feat: add guest path mapper for sandbox mounts"
```

## Task 4: Add gVisor Session Preparation

**Files:**
- Create: `pkg/sandbox/gvisorenv/session.go`
- Create: `pkg/sandbox/gvisorenv/session_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestPrepareSessionUsesConfiguredMounts(t *testing.T) {}
func TestPrepareSessionCreatesDefaultWorkspaceWhenMountsEmpty(t *testing.T) {}
func TestPrepareSessionUsesWorkspaceSessionID(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/sandbox/gvisorenv -run 'TestPrepareSessionUsesConfiguredMounts|TestPrepareSessionCreatesDefaultWorkspaceWhenMountsEmpty|TestPrepareSessionUsesWorkspaceSessionID'`

Expected: FAIL because gVisor session preparation does not exist.

**Step 3: Write minimal implementation**

Create `pkg/sandbox/gvisorenv/session.go` with:

- gVisor session config resolution
- default workspace creation
- mount normalization
- `PreparedSession` construction

Default behavior:

- if mounts are empty, create `<projectRoot>/workspace/<session_id>/`
- mount it writable to `/workspace`
- set guest cwd to `/workspace`

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/sandbox/gvisorenv -run 'TestPrepareSessionUsesConfiguredMounts|TestPrepareSessionCreatesDefaultWorkspaceWhenMountsEmpty|TestPrepareSessionUsesWorkspaceSessionID'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/sandbox/gvisorenv/session.go pkg/sandbox/gvisorenv/session_test.go
git commit -m "feat: add gvisor session preparation"
```

## Task 5: Move File Tools To Environment Methods

**Files:**
- Modify: `pkg/tool/builtin/read.go`
- Modify: `pkg/tool/builtin/write.go`
- Modify: `pkg/tool/builtin/edit.go`
- Modify: `pkg/tool/builtin/glob.go`
- Modify: grep-related files under `pkg/tool/builtin`
- Test: `pkg/tool/builtin/read_test.go`
- Test: `pkg/tool/builtin/write_test.go`
- Test: `pkg/tool/builtin/edit_test.go`
- Test: `pkg/tool/builtin/file_glob_grep_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestWriteRejectsReadOnlyGuestMount(t *testing.T) {}
func TestReadUsesGuestPathResolution(t *testing.T) {}
func TestGlobReturnsGuestPaths(t *testing.T) {}
func TestGrepReturnsGuestPaths(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run 'TestWriteRejectsReadOnlyGuestMount|TestReadUsesGuestPathResolution|TestGlobReturnsGuestPaths|TestGrepReturnsGuestPaths'`

Expected: FAIL because file tools still operate directly on host paths.

**Step 3: Write minimal implementation**

Update the built-ins to depend on `ExecutionEnvironment` instead of direct host-path logic.

Requirements:

- guest path input only in gVisor mode
- writable mounts required for write / edit
- returned paths use guest paths
- host mode can continue to resolve through `HostEnvironment`

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run 'TestWriteRejectsReadOnlyGuestMount|TestReadUsesGuestPathResolution|TestGlobReturnsGuestPaths|TestGrepReturnsGuestPaths'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/tool/builtin/read.go pkg/tool/builtin/write.go pkg/tool/builtin/edit.go pkg/tool/builtin/glob.go pkg/tool/builtin/*.go pkg/tool/builtin/*_test.go
git commit -m "feat: route file tools through execution environments"
```

## Task 6: Add Helper Protocol

**Files:**
- Create: `pkg/sandbox/gvisorhelper/protocol.go`
- Create: `pkg/sandbox/gvisorhelper/protocol_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestHelperRequestRoundTrip(t *testing.T) {}
func TestHelperResponseRoundTrip(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/sandbox/gvisorhelper -run 'TestHelperRequestRoundTrip|TestHelperResponseRoundTrip'`

Expected: FAIL because the helper protocol does not exist.

**Step 3: Write minimal implementation**

Define:

- `HelperRequest`
- `HelperResponse`
- JSON encoding / decoding helpers

Use stdin/stdout JSON framing for the first version.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/sandbox/gvisorhelper -run 'TestHelperRequestRoundTrip|TestHelperResponseRoundTrip'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/sandbox/gvisorhelper/protocol.go pkg/sandbox/gvisorhelper/protocol_test.go
git commit -m "feat: add gvisor helper protocol"
```

## Task 7: Add Self-Exec Helper Mode

**Files:**
- Create: `pkg/sandbox/gvisorhelper/helper.go`
- Modify: `cmd/cli/main.go`
- Test: `cmd/cli/main_test.go`
- Test: `pkg/sandbox/gvisorhelper/helper_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestMainDispatchesToGVisorHelperMode(t *testing.T) {}
func TestHelperModeReadsRequestAndWritesResponse(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli ./pkg/sandbox/gvisorhelper -run 'TestMainDispatchesToGVisorHelperMode|TestHelperModeReadsRequestAndWritesResponse'`

Expected: FAIL because helper mode does not exist.

**Step 3: Write minimal implementation**

Implement helper mode:

- add hidden `--agentkit-gvisor-helper` flag handling in `cmd/cli/main.go`
- branch early into helper path
- keep helper startup minimal
- read request from stdin and emit response to stdout

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/cli ./pkg/sandbox/gvisorhelper -run 'TestMainDispatchesToGVisorHelperMode|TestHelperModeReadsRequestAndWritesResponse'`

Expected: PASS.

**Step 5: Commit**

```bash
git add cmd/cli/main.go cmd/cli/main_test.go pkg/sandbox/gvisorhelper/helper.go pkg/sandbox/gvisorhelper/helper_test.go
git commit -m "feat: add self-exec gvisor helper mode"
```

## Task 8: Add GVisor Command Execution

**Files:**
- Create: `pkg/sandbox/gvisorenv/environment.go`
- Modify: `pkg/tool/builtin/bash.go`
- Test: `pkg/tool/builtin/bash_test.go`
- Test: `pkg/tool/builtin/bash_stream_test.go`
- Test: `pkg/sandbox/gvisorenv/environment_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestGVisorEnvironmentRunCommandExecsHelper(t *testing.T) {}
func TestBashToolUsesExecutionEnvironment(t *testing.T) {}
func TestBashToolGuestWorkdirIsPassedToEnvironment(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin ./pkg/sandbox/gvisorenv -run 'TestGVisorEnvironmentRunCommandExecsHelper|TestBashToolUsesExecutionEnvironment|TestBashToolGuestWorkdirIsPassedToEnvironment'`

Expected: FAIL because bash still uses host `exec.CommandContext`.

**Step 3: Write minimal implementation**

Create `pkg/sandbox/gvisorenv/environment.go` with:

- `PrepareSession`
- `RunCommand`
- self-exec helper invocation
- helper request / response handling

Modify `pkg/tool/builtin/bash.go`:

- stop calling `exec.CommandContext("bash", "-c", ...)`
- call `ExecutionEnvironment.RunCommand`
- keep output spool / timeout integration in the main process

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin ./pkg/sandbox/gvisorenv -run 'TestGVisorEnvironmentRunCommandExecsHelper|TestBashToolUsesExecutionEnvironment|TestBashToolGuestWorkdirIsPassedToEnvironment'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/sandbox/gvisorenv/environment.go pkg/sandbox/gvisorenv/environment_test.go pkg/tool/builtin/bash.go pkg/tool/builtin/bash_test.go pkg/tool/builtin/bash_stream_test.go
git commit -m "feat: execute bash through gvisor environment"
```

## Task 9: Wire Runtime Tool Construction To Environments

**Files:**
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/runtime_helpers.go`
- Test: `pkg/api/runtime_helpers_tools_test.go`
- Test: `pkg/api/agent_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestBuiltinFactoriesInjectExecutionEnvironment(t *testing.T) {}
func TestGVisorSandboxBuildsToolsWithGuestAwareEnvironment(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestBuiltinFactoriesInjectExecutionEnvironment|TestGVisorSandboxBuildsToolsWithGuestAwareEnvironment'`

Expected: FAIL because built-in tools are not environment-driven.

**Step 3: Write minimal implementation**

Update runtime tool factory setup so that:

- tools receive the selected environment
- gVisor mode uses `GVisorEnvironment`
- host mode uses `HostEnvironment`

Keep existing sandbox policy injection intact.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestBuiltinFactoriesInjectExecutionEnvironment|TestGVisorSandboxBuildsToolsWithGuestAwareEnvironment'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/agent.go pkg/api/runtime_helpers.go pkg/api/runtime_helpers_tools_test.go pkg/api/agent_test.go
git commit -m "feat: wire runtime tools to sandbox environments"
```

## Task 10: Add Integration Coverage

**Files:**
- Create: `test/integration/gvisor_sandbox_test.go`
- Optionally modify: `test/integration/clikit_shared_test.go`

**Step 1: Write the failing tests**

Add integration coverage for:

```go
func TestGVisorSandboxDefaultWorkspace(t *testing.T) {}
func TestGVisorSandboxReadOnlyMountBlocksWrite(t *testing.T) {}
func TestGVisorSandboxBashAndFileReadSeeSamePath(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./test/integration -run 'TestGVisorSandboxDefaultWorkspace|TestGVisorSandboxReadOnlyMountBlocksWrite|TestGVisorSandboxBashAndFileReadSeeSamePath'`

Expected: FAIL because the gVisor sandbox path is not implemented.

**Step 3: Write minimal implementation**

Use the new runtime configuration in tests to verify:

- default per-session workspace creation
- mount permissions
- guest-path consistency across shell and file tools

Skip gracefully when the environment cannot support the imported gVisor execution path yet.

**Step 4: Run test to verify it passes**

Run: `go test ./test/integration -run 'TestGVisorSandboxDefaultWorkspace|TestGVisorSandboxReadOnlyMountBlocksWrite|TestGVisorSandboxBashAndFileReadSeeSamePath'`

Expected: PASS or explicit SKIP in unsupported environments.

**Step 5: Commit**

```bash
git add test/integration/gvisor_sandbox_test.go test/integration/clikit_shared_test.go
git commit -m "test: add integration coverage for gvisor sandbox"
```

## Task 11: Final Verification

**Files:**
- No code changes required unless failures are found

**Step 1: Run targeted package tests**

Run:

```bash
go test ./pkg/api ./pkg/sandbox/... ./pkg/tool/builtin ./cmd/cli ./test/integration
```

Expected: PASS, or documented SKIP for environment-specific gVisor coverage.

**Step 2: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

**Step 3: Run race tests if practical**

Run:

```bash
go test -race ./...
```

Expected: PASS, or document any environment-specific exclusions.

**Step 4: Commit**

```bash
git add .
git commit -m "feat: add gvisor sandbox execution environment"
```
