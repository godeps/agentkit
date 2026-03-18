# GVisor Sandbox Design

**Date:** 2026-03-12

**Status:** Proposed

**Goal:** Add a `gvisor` sandbox type that keeps command execution isolated from the host while presenting a single, consistent guest filesystem view to `bash`, `file_read`, `file_write`, `file_edit`, `glob`, and `grep`.

## Problem

`agentkit` already has sandbox-related types, but the current implementation is still host-native:

- `pkg/security` validates paths, commands, and permission rules
- `pkg/sandbox` provides fs / network / resource policy helpers
- `BashTool` still executes `bash -c` directly on the host
- file tools still operate on host paths directly

That means the current sandbox is a policy layer, not an execution environment. If we want gVisor-backed isolation, two things must change together:

1. shell execution must move out of the host process environment
2. file tools and shell commands must see the same filesystem view

If only `bash` is sandboxed and file tools remain host-native, users get path and behavior mismatches immediately.

## Scope

### In scope

- New `gvisor` sandbox type in `SandboxOptions`
- Configurable mounts with per-mount read-only / writable flags
- Default session workspace mount when gVisor is enabled and no mounts are provided
- Unified guest-path model across `bash` and file tools
- Single-binary helper mode: main process `exec`s itself into gVisor helper mode
- Source dependency on `gvisor.dev/gvisor/...`

### Out of scope

- Firecracker / microVM support
- Long-lived interactive shell sessions
- Container-in-container / Docker compatibility
- Full VM-grade filesystem virtualization
- Distributed session storage or remote sandbox orchestration

## Success Criteria

- Enabling gVisor does not require shipping a separate helper binary.
- The runtime uses one executable in two modes: normal mode and hidden helper mode.
- `bash`, `file_read`, `file_write`, `file_edit`, `glob`, and `grep` all operate on the same guest-visible path space.
- Mounts are configurable and enforce read-only vs writable behavior.
- When mounts are omitted, the runtime creates `workspace/<session_id>/` under the project root and mounts it writable at `/workspace`.
- Existing host-mode behavior remains the default and remains backward compatible.

## Approaches

### Option 1: External `runsc` binary

Compile or require `runsc`, then have `agentkit` launch it as an external runtime process.

**Pros**

- Closest to gVisor's runtime model
- Good process isolation
- Lower coupling to gVisor internal packages

**Cons**

- Requires an extra runtime artifact or system dependency
- Conflicts with the requirement to rely on imported source and a single binary

### Option 2: Single-binary helper mode using imported gVisor code

The main `agentkit` process imports `gvisor.dev/gvisor/...` and, when gVisor mode is requested, `exec`s the current executable with a hidden helper flag. The helper path initializes gVisor runtime logic and executes the sandboxed command.

**Pros**

- Meets the single-binary requirement
- Keeps a process boundary between main runtime and sandbox runtime
- Avoids a separate helper artifact
- Supports source-level dependency on gVisor

**Cons**

- More coupling to gVisor internals than external `runsc`
- Helper startup path must be kept minimal and stable
- Upgrading gVisor may require more adaptation work

### Option 3: In-process gVisor runtime

Import gVisor packages and run runtime logic directly inside the main `agentkit` process.

**Pros**

- No extra process
- Simplest runtime topology on paper

**Cons**

- Loses the process boundary between agent runtime and sandbox runtime
- Higher fault-propagation risk
- Harder to reason about lifecycle, global state, and error isolation
- Poorer engineering fit for `agentkit`

## Approach Comparison

| Option | Single binary | Process boundary | Coupling to gVisor internals | Operational complexity | Recommended |
|---|---|---:|---:|---:|---|
| `1` External `runsc` | No | Yes | Low | Medium | No |
| `2` Self-exec helper | Yes | Yes | Medium | Medium | Yes |
| `3` In-process runtime | Yes | No | Highest | Low initially, high later | No |

## Recommendation

Use **Option 2: self-exec helper mode**.

Reasoning:

- It satisfies the stated constraints: single binary, source dependency on gVisor, and no extra helper artifact.
- It preserves a process boundary between the main runtime and sandbox runtime.
- It lets `agentkit` keep control of session state, permissions, path mapping, and tool orchestration while delegating isolated command execution to the helper path.

## Proposed Design

### 1. Configuration Model

Extend `pkg/api/options.go`:

```go
type SandboxOptions struct {
    Type          string
    Root          string
    AllowedPaths  []string
    NetworkAllow  []string
    ResourceLimit sandbox.ResourceLimits

    GVisor *GVisorOptions
}

type GVisorOptions struct {
    Enabled                    bool
    DefaultGuestCwd            string
    AutoCreateSessionWorkspace bool
    SessionWorkspaceBase       string
    HelperModeFlag             string
    Mounts                     []MountSpec
}

type MountSpec struct {
    HostPath        string
    GuestPath       string
    ReadOnly        bool
    CreateIfMissing bool
}
```

Rules:

- gVisor mode is enabled when `Sandbox.Type == "gvisor"` or `Sandbox.GVisor.Enabled == true`
- if `Mounts` is empty, create `<projectRoot>/workspace/<session_id>/` and mount it writable at `/workspace`
- `GuestPath` must be absolute
- guest mount paths must not overlap
- host paths are normalized before use

### 2. Execution Environment Abstraction

Add a new execution-layer abstraction separate from the current policy-layer sandbox:

```go
type ExecutionEnvironment interface {
    PrepareSession(ctx context.Context, session SessionContext) (*PreparedSession, error)
    RunCommand(ctx context.Context, ps *PreparedSession, req CommandRequest) (*CommandResult, error)

    ReadFile(ctx context.Context, ps *PreparedSession, guestPath string) ([]byte, error)
    WriteFile(ctx context.Context, ps *PreparedSession, guestPath string, data []byte) error
    EditFile(ctx context.Context, ps *PreparedSession, req EditRequest) error
    Glob(ctx context.Context, ps *PreparedSession, pattern string) ([]string, error)
    Grep(ctx context.Context, ps *PreparedSession, req GrepRequest) ([]GrepMatch, error)

    CloseSession(ctx context.Context, ps *PreparedSession) error
}
```

Implementations:

- `HostEnvironment`
- `GVisorEnvironment`

The existing `pkg/sandbox` package remains the policy layer. The new environment layer is responsible for actual execution behavior.

### 3. Unified Guest Path Model

All user-visible paths in gVisor mode are guest paths.

Example:

- `/workspace`
- `/workspace/src`
- `/workspace/out`

Host paths are internal implementation details resolved through a `PathMapper`.

```go
type PathMapper interface {
    GuestToHost(guestPath string) (string, MountSpec, error)
    HostToGuest(hostPath string) (string, MountSpec, error)
    VisibleRoots() []string
}
```

Rules:

- `bash` `workdir` is interpreted as a guest path
- `file_read` returns content from a guest path
- `file_write` / `file_edit` may only target writable guest mounts
- `glob` / `grep` enumerate guest-visible paths and return guest paths

### 4. Main Process vs Helper Process

Use a single executable in two modes:

- normal agent mode
- hidden gVisor helper mode

Flow:

1. main process selects `GVisorEnvironment`
2. session mounts and guest cwd are prepared in the main process
3. main process `exec`s the current executable with a hidden helper flag
4. helper process initializes imported gVisor runtime logic
5. helper process runs the isolated command
6. helper returns structured results over stdio

This preserves a process boundary without introducing a second binary.

### 5. Helper Protocol

Use a small stdio JSON protocol for the first implementation.

Request:

```go
type HelperRequest struct {
    Version   string
    SessionID string
    Command   string
    GuestCwd  string
    TimeoutMs int64
    Env       map[string]string
    Mounts    []MountSpec
    Limits    sandbox.ResourceLimits
}
```

Response:

```go
type HelperResponse struct {
    Success    bool
    ExitCode   int
    Stdout     string
    Stderr     string
    DurationMs int64
    Error      string
}
```

The helper owns command execution. The main process owns tool orchestration and output persistence.

### 6. Tool Integration

The following built-ins must move to the environment abstraction:

- `bash`
- `file_read`
- `file_write`
- `file_edit`
- `glob`
- `grep`

This is required so that all tool behavior is defined by the same guest mount table and path mapping rules.

### 7. Default Mount Behavior

If gVisor is enabled and no mounts are provided:

- create `<projectRoot>/workspace/<session_id>/`
- mount it writable as `/workspace`
- use `/workspace` as the default guest cwd

This yields a predictable default workspace without needing extra user configuration.

## Module Layout

Recommended new packages:

- `pkg/sandbox/env`
- `pkg/sandbox/hostenv`
- `pkg/sandbox/gvisorenv`
- `pkg/sandbox/pathmap`
- `pkg/sandbox/gvisorhelper`

Existing files that need integration work:

- `pkg/api/options.go`
- `pkg/api/agent.go`
- `pkg/api/sandbox_bridge.go`
- `pkg/tool/builtin/bash.go`
- `pkg/tool/builtin/read.go`
- `pkg/tool/builtin/write.go`
- `pkg/tool/builtin/edit.go`
- `pkg/tool/builtin/glob.go`
- grep implementation files under `pkg/tool/builtin`
- `cmd/cli/main.go` or equivalent main entry point

## Error Handling

Errors should be clearly classified:

- invalid gVisor config
- invalid mount table
- guest path not mounted
- guest path is read-only
- helper startup failure
- helper protocol decode failure
- sandbox command failure
- sandbox command timeout

These should surface as deterministic tool/runtime errors rather than ambiguous shell failures.

## Testing Strategy

### Unit tests

- config normalization
- default session workspace generation
- mount validation and overlap rejection
- guest-to-host and host-to-guest path mapping
- read-only mount enforcement
- helper request / response encode-decode

### Integration tests

- gVisor mode `bash` executes via helper
- `file_write` writes only to writable mounts
- `file_read` reads a file created by sandboxed `bash`
- `glob` / `grep` return guest paths
- host mode regression remains unchanged

## Risks

- gVisor internal package changes may require helper adaptation
- helper startup cost may be visible for short commands
- mount mapping bugs would cause confusing path behavior
- insufficient read-only enforcement would weaken the safety model

## Open Questions

- Which exact gVisor packages provide the most maintainable helper integration path?
- How much of `NetworkAllow` and `ResourceLimit` can be enforced cleanly in the first helper version?
- Should guest-path-only behavior be enabled immediately in host mode, or phased in after gVisor mode stabilizes?

## Rollout Plan

1. Add config model and execution environment abstraction.
2. Move file tools onto guest-path-aware environment methods.
3. Add self-exec helper mode and wire `bash` through `GVisorEnvironment`.
4. Add network/resource support and harden cleanup, diagnostics, and tests.
