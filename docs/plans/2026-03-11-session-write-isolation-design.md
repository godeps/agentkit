# Session Write Isolation Design

**Date:** 2026-03-11

**Status:** Proposed

**Goal:** Prevent accidental cross-session overwrites for repository paths while keeping generated artifacts strongly isolated by default.

## Problem

`agentkit` already isolates session execution and session history:

- same-session concurrent `Run` / `RunStream` is blocked by `sessionGate`
- message history is partitioned by `SessionID`
- bash and tool output directories are partitioned by `SessionID`

But repository file writes are not session-safe today:

- `Write` overwrites the target path directly
- `Edit` reads and writes the same path directly
- two different sessions can write the same repository file path and the last writer wins
- generated artifacts may still be written into shared project paths if the model chooses them

This creates two distinct risks:

1. **Repository file collision**
Two sessions modify the same code path and silently overwrite one another.

2. **Artifact path collision**
Two sessions generate files like `output/result.json` or `output/image.png` and collide even though those files are session-local in intent.

## Scope

### In scope

- Cross-session write isolation for `Write` and `Edit`
- Default session-local output roots for generated artifacts
- Explicit escape hatch for shared-write mode
- Conflict detection semantics
- Backward-compatible migration path

### Out of scope

- Full per-session repository virtualization
- Git merge workflows
- Task store isolation
- Multi-host distributed locking

## Success Criteria

- Two sessions cannot silently overwrite the same repository path by default.
- Generated artifacts default to session-scoped paths.
- Intentional shared writes remain possible through an explicit policy.
- Existing non-streaming and streaming runtime APIs remain stable.
- Conflicts surface as deterministic runtime/tool errors, not partial corruption.

## Approaches

### Option 0: Per-session isolated workspace

Give each session its own working directory and run all file tools, shell commands, and generated outputs inside that isolated workspace.

Possible implementations:

- full filesystem copy under `.agentkit/workspaces/<session_id>/`
- Git-backed worktree under `.agentkit/worktrees/<session_id>/` when `ProjectRoot` is a repository

**Pros**

- Strongest isolation model
- Repository writes, artifacts, temp files, and shell side effects are naturally partitioned
- Eliminates most cross-session file collision classes instead of merely detecting them
- Conceptually simple for hosts: one session, one filesystem view

**Cons**

- Much larger architectural change
- Requires rethinking `ProjectRoot`, sandbox root, config loading, history root, output root, and lifecycle cleanup
- Git/non-Git behavior diverges
- Results must be merged or copied back into the shared workspace
- Significantly higher disk, startup, and operational cost
- More of a host/runtime execution-model redesign than a write-safety feature

### Option 1: Session-scoped output roots only

Make generated files default to `.agentkit/sessions/<session_id>/output/...`, but leave repository file writes unchanged.

**Pros**

- Lowest implementation cost
- Solves most artifact collision cases
- Minimal behavior change for code-editing flows

**Cons**

- Does not solve true repository write collisions
- `Write` / `Edit` still silently race across sessions
- Safety improvement is incomplete

### Option 2: Path lock + session output roots

Keep real repository writes possible, but guard each canonical path with a cross-session lock. In parallel, route generated artifacts to session-local output roots by default.

**Pros**

- Directly addresses repository-path collisions
- Preserves current filesystem model
- Smallest complete fix
- Fits `agentkit`'s runtime-kernel shape

**Cons**

- Protects concurrent overlap, but not stale read-modify-write by itself
- Requires careful lock acquisition/release in tool execution paths

### Option 3: Path lock + session output roots + optimistic version check

Use Option 2, and additionally require edits to prove the target file has not changed since last read using `mtime`, `size`, or content hash metadata.

**Pros**

- Strongest local correctness story
- Prevents both simultaneous collision and stale overwrite
- Best fit for coding-agent workloads

**Cons**

- Higher implementation complexity
- Requires read/write contract changes
- Needs migration strategy for existing tool call schema

## Approach Comparison

| Option | Isolation strength | Compatibility | Implementation cost | Operational cost | Solves repo collision | Solves artifact collision | Recommended use |
|---|---|---|---|---|---|---|---|
| `0` Per-session workspace | Highest | Lowest | Highest | Highest | Yes | Yes | High-isolation execution mode |
| `1` Session output roots only | Low | Highest | Low | Low | No | Mostly | Minimal stopgap |
| `2` Path lock + session output roots | Medium | High | Medium | Low | Yes, concurrent only | Yes | Good first complete fix |
| `3` Path lock + session output roots + optimistic proof | High | Medium-High | Medium-High | Low | Yes | Yes | Best shared-workspace default |

## Recommendation

Use **Option 3** as the default design for the shared-workspace runtime, and keep **Option 0** as a future high-isolation mode.

Reasoning:

- `agentkit` today is organized around one `ProjectRoot`-centric runtime with shared registries, sandbox root, and history/task lifecycles.
- Option 0 is valid, and stronger, but it is closer to a session-execution-architecture redesign than to a targeted write-isolation fix.
- Option 3 delivers most of the practical safety benefit while preserving current runtime shape and downstream compatibility.

Use **Option 3** in phased rollout:

1. **Phase 1**
Introduce session output roots and cross-session path locks.

2. **Phase 2**
Add optimistic file-state validation for `Edit` and eventually `Write`.

This is the best tradeoff between safety and compatibility for the current runtime model. Option 1 is too weak. Option 2 is good but still leaves stale overwrite windows. Option 0 should remain a separately designed "isolated workspace mode" for users who prefer stronger isolation over lower complexity and lower cost.

## Proposed Design

### Shared-Workspace Default vs Isolated-Workspace Future

This proposal intentionally chooses a **shared workspace with stronger write safety**, not a per-session workspace architecture.

That means:

- all sessions still point at the same `ProjectRoot`
- repository writes are protected by path lock and optional stale-state validation
- generated artifacts default to session-local roots

What this proposal does **not** do:

- create per-session repo copies
- create Git worktrees
- merge per-session changes back into a primary workspace

Those capabilities are still desirable, but they belong in a separate design track because they change the runtime execution model, not just file write safety.

### 1. Write Isolation Model

Classify writes into two categories:

#### A. Session-local artifact writes

Default target root:

```text
.agentkit/sessions/<session_id>/output/
```

Behavior:

- any generated artifact path that is not an explicit user-requested repository path is resolved under the session output root
- examples:
  - `output/result.json`
  - `output/image.png`
  - `output/report/index.html`

These paths are never shared across sessions by default.

#### B. Repository writes

Canonical target:

- absolute, cleaned path under `ProjectRoot`

Behavior:

- path must acquire a runtime path lock before write
- lock scope is per-runtime, keyed by canonical absolute path
- default policy rejects or waits when another session holds the same path

### 2. Runtime Path Gate

Add a new runtime helper similar to `sessionGate`:

```go
type pathGate struct {
    gates sync.Map // map[string]chan struct{}
}
```

Key:

- canonical absolute filesystem path

Operations:

- `Acquire(ctx, path, sessionID)`
- `Release(path)`

Properties:

- session-agnostic lock key
- protects file writes across all sessions in the same runtime
- lock held for the shortest possible duration around filesystem mutation

### 3. Write Policy Modes

Introduce explicit write isolation modes:

```go
type WriteIsolationMode string

const (
    WriteIsolationStrict WriteIsolationMode = "strict"
    WriteIsolationShared WriteIsolationMode = "shared"
    WriteIsolationLegacy WriteIsolationMode = "legacy"
)
```

#### `strict` (recommended default)

- repository paths are path-locked
- session output root is applied by default
- shared collisions return deterministic conflict errors

#### `shared`

- repository writes still path-locked
- conflict can wait for lock if caller opts in
- explicit host-controlled mode for coordinated workflows

#### `legacy`

- preserves current behavior
- no path-gate enforcement
- for compatibility only, not recommended

## File Operation Semantics

### Write tool

#### Current behavior

- accepts absolute path
- writes content directly
- overwrites existing file

#### Proposed behavior

1. Resolve requested path type:
   - repository path
   - session artifact path

2. If artifact-like and not explicitly pinned by user:
   - rewrite under session output root

3. If repository path:
   - acquire `pathGate`
   - validate policy
   - write atomically
   - release `pathGate`

4. Return metadata including:

```json
{
  "path": "...",
  "resolved_path": "...",
  "scope": "session_output|repository",
  "session_id": "...",
  "bytes": 123
}
```

### Edit tool

#### Current behavior

- reads current content
- applies replacement
- writes file back directly

#### Proposed behavior

Phase 1:

- acquire `pathGate` before final write
- write atomically under lock

Phase 2:

- require file-state proof from prior read
- reject if current file state differs from expected

Returned conflict example:

```text
edit rejected: /repo/app/main.go changed since last read by session sess-b
```

### Bash / custom tool writes

For shell-based artifact generation:

- default working convention should expose `AGENTKIT_SESSION_OUTPUT_DIR`
- downstream prompts and helpers should encourage writing generated files there
- runtime-owned bash output spooling remains session-partitioned as it is today

This does not fully sandbox arbitrary shell writes, but it gives a strong default for generated outputs without breaking existing shell capabilities.

## Path Classification

Add a classifier for candidate write targets:

```go
type PathScope string

const (
    PathScopeRepository   PathScope = "repository"
    PathScopeSessionOutput PathScope = "session_output"
)
```

Rules:

1. If path is under configured session output root, classify as `session_output`
2. If path is relative and starts with `output/`, rewrite to session output root
3. If path is under project root and outside session output root, classify as `repository`
4. If path is outside project root, existing sandbox and permission logic still governs it

## Optimistic Version Check

### Read-side metadata

`Read` should eventually expose file state metadata:

```json
{
  "path": "/repo/app/main.go",
  "mtime_unix_nano": 123456789,
  "size": 2048,
  "sha256": "..."
}
```

### Edit-side validation

`Edit` can accept optional expected state:

```json
{
  "file_path": "/repo/app/main.go",
  "old_string": "...",
  "new_string": "...",
  "expected_sha256": "..."
}
```

If omitted:

- Phase 1 behavior stays compatible

If provided and mismatch:

- reject write with conflict error

This allows gradual hardening without breaking all existing tool callers immediately.

## Conflict Handling

### Default conflict policy

For repository writes in `strict` mode:

- fail fast on lock contention or stale state mismatch

Example:

```text
write rejected: path /repo/pkg/api/agent.go is currently locked by another session
```

### Optional coordinated policy

In `shared` mode:

- host may allow waiting for path lock
- stale version mismatch still fails

### Why fail fast by default

- avoids deadlocks or long blocking in agent loops
- makes conflicts visible to the model and host
- better suited to multi-session coding workloads

## Atomicity Requirements

All repository writes should move to atomic write semantics:

- write temp file in same directory
- fsync/close temp file
- rename temp file over target

This reduces partial-write corruption and aligns with the existing runtime history persistence pattern.

## API Surface Changes

### Runtime options

Add to `api.Options`:

```go
type WriteIsolationOptions struct {
    Mode              WriteIsolationMode
    SessionOutputRoot string
    LockTimeout       time.Duration
    RequireReadProof  bool
}
```

### Runtime internals

Add to `api.Runtime`:

- `pathGate`
- resolved write isolation config

### Tool metadata

Enrich tool execution context with:

- `session_id`
- `session_output_dir`
- `write_isolation_mode`

## Migration Plan

### Stage 1

- add write isolation config with `legacy` default behind explicit opt-in
- add session output root helpers
- add path gate for `Write` and `Edit`

### Stage 2

- switch runtime default to `strict`
- document rendered conflict errors
- expose session output dir in CLI / downstream adapters

### Stage 3

- add read proof metadata and edit proof validation
- optionally make proof validation default for `Edit`

## Testing Plan

### Unit tests

- path gate same-path contention
- different-path writes proceed independently
- session output path rewriting
- strict/shared/legacy policy behavior
- atomic write preserves final content

### Runtime concurrency tests

- two sessions writing same repo path
- two sessions writing different repo paths
- one session writing repo path while another writes session-local artifact

### Tool tests

- `Write` rewrites `output/foo.json` into session output root
- `Write` preserves explicit repository targets
- `Edit` rejects stale proof mismatch
- `Edit` succeeds when proof matches

### Integration tests

- parallel sessions generating `output/result.json` do not collide
- parallel sessions attempting same repository write conflict deterministically
- bash output directory and generated artifact root remain session-local

## Open Questions

1. Should `Write` infer artifact-vs-repository automatically forever, or should a future schema add an explicit `scope` field?
2. Should custom tools receive a general-purpose path gate helper from runtime, or is `Write`/`Edit` coverage sufficient for first rollout?
3. Should stale proof validation be optional forever, or become mandatory once `Read` reliably returns file state metadata?

## Recommended First Implementation Slice

Implement the smallest complete safety improvement first:

1. session output root helper
2. path classifier
3. runtime `pathGate`
4. `Write` and `Edit` path locking
5. atomic writes
6. conflict tests

Then add read-proof validation as the second slice.
