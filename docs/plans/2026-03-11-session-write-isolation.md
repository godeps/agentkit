# Session Write Isolation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add cross-session repository write protection and default session-scoped artifact output paths without breaking existing runtime APIs.

**Architecture:** Introduce a runtime-level `pathGate` alongside the existing `sessionGate`, add configurable write-isolation policy and session output root helpers, then wire `Write` and `Edit` through path classification, lock acquisition, and atomic writes. After that, expose read-side file state metadata and optional optimistic proof validation so stale read-modify-write cycles are rejected instead of silently overwriting newer content.

**Tech Stack:** Go 1.24, `pkg/api`, `pkg/tool/builtin`, `pkg/runtime/tasks`, `pkg/security`, `pkg/message`, Go tests

---

## Chosen Direction

This implementation plan intentionally targets the **shared-workspace hardening** path, not per-session isolated workspaces.

Reason:

- it preserves the current `ProjectRoot`-centric runtime contract
- it avoids pulling Git worktree or repo-copy lifecycle into the first implementation
- it solves the immediate overwrite problem with smaller compatibility risk

Deferred alternative:

- a future `isolated workspace mode` may create one workspace per session and treat the current plan as the lower-cost default mode
- if that mode is pursued later, it should be designed as a separate runtime execution model, not mixed into the first write-isolation rollout

### Task 1: Add Runtime Write Isolation Configuration

**Files:**
- Modify: `pkg/api/options.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/api/runtime_helpers.go`
- Test: `pkg/api/options_test.go`
- Test: `pkg/api/runtime_helpers_additional_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestWriteIsolationOptionsDefaultsToLegacy(t *testing.T) {}
func TestWriteIsolationOptionsFrozenCopiesFields(t *testing.T) {}
func TestSessionOutputRootUsesSessionID(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestWriteIsolationOptionsDefaultsToLegacy|TestWriteIsolationOptionsFrozenCopiesFields|TestSessionOutputRootUsesSessionID'`

Expected: FAIL because write-isolation options and helpers do not exist yet.

**Step 3: Write minimal implementation**

Add to `pkg/api/options.go`:

```go
type WriteIsolationMode string

const (
    WriteIsolationStrict WriteIsolationMode = "strict"
    WriteIsolationShared WriteIsolationMode = "shared"
    WriteIsolationLegacy WriteIsolationMode = "legacy"
)

type WriteIsolationOptions struct {
    Mode              WriteIsolationMode
    SessionOutputRoot string
    LockTimeout       time.Duration
    RequireReadProof  bool
}
```

Extend `Options` with:

```go
WriteIsolation WriteIsolationOptions
```

Add helpers in `pkg/api/runtime_helpers.go`:

```go
func sessionOutputRoot(projectRoot, sessionID, configured string) string
func normalizeWriteIsolationOptions(projectRoot string, in WriteIsolationOptions) WriteIsolationOptions
```

Update `Options.withDefaults()` / `frozen()` paths so the config is normalized and copied.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestWriteIsolationOptionsDefaultsToLegacy|TestWriteIsolationOptionsFrozenCopiesFields|TestSessionOutputRootUsesSessionID'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/options.go pkg/api/agent.go pkg/api/runtime_helpers.go pkg/api/options_test.go pkg/api/runtime_helpers_additional_test.go
git commit -m "feat: add runtime write isolation options"
```

### Task 2: Add Runtime Path Gate

**Files:**
- Modify: `pkg/api/runtime_helpers.go`
- Modify: `pkg/api/agent.go`
- Test: `pkg/api/session_gate_test.go`
- Test: `pkg/api/agent_concurrent_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestPathGateSamePathBlocks(t *testing.T) {}
func TestPathGateDifferentPathsIndependent(t *testing.T) {}
func TestPathGateReleaseOnContextCancel(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestPathGateSamePathBlocks|TestPathGateDifferentPathsIndependent|TestPathGateReleaseOnContextCancel'`

Expected: FAIL because `pathGate` does not exist.

**Step 3: Write minimal implementation**

In `pkg/api/runtime_helpers.go`, add:

```go
type pathGate struct {
    gates sync.Map // map[string]chan struct{}
}

func newPathGate() *pathGate
func (g *pathGate) Acquire(ctx context.Context, canonicalPath string) error
func (g *pathGate) Release(canonicalPath string)
```

In `pkg/api/agent.go`, add a `pathGate` field to `Runtime` and initialize it in `New()`.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestPathGateSamePathBlocks|TestPathGateDifferentPathsIndependent|TestPathGateReleaseOnContextCancel'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/runtime_helpers.go pkg/api/agent.go pkg/api/session_gate_test.go pkg/api/agent_concurrent_test.go
git commit -m "feat: add runtime path gate for write isolation"
```

### Task 3: Add Path Classification And Session Output Rewriting

**Files:**
- Create: `pkg/tool/builtin/write_isolation.go`
- Test: `pkg/tool/builtin/write_isolation_test.go`
- Modify: `pkg/api/runtime_helpers.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestClassifyWritePathRepository(t *testing.T) {}
func TestClassifyWritePathSessionOutputRewrite(t *testing.T) {}
func TestClassifyWritePathExplicitRepositoryRemainsUnchanged(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run 'TestClassifyWritePathRepository|TestClassifyWritePathSessionOutputRewrite|TestClassifyWritePathExplicitRepositoryRemainsUnchanged'`

Expected: FAIL because the classifier does not exist.

**Step 3: Write minimal implementation**

Create `pkg/tool/builtin/write_isolation.go` with:

```go
type PathScope string

const (
    PathScopeRepository    PathScope = "repository"
    PathScopeSessionOutput PathScope = "session_output"
)

type WritePathResolution struct {
    OriginalPath string
    ResolvedPath string
    Scope        PathScope
}

func ResolveWritePath(projectRoot, sessionID, sessionOutputRoot, raw string) (WritePathResolution, error)
```

Rules:

- `output/...` relative paths resolve into session output root
- explicit absolute paths under project root stay repository-scoped
- paths already under session output root stay session-scoped

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run 'TestClassifyWritePathRepository|TestClassifyWritePathSessionOutputRewrite|TestClassifyWritePathExplicitRepositoryRemainsUnchanged'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/tool/builtin/write_isolation.go pkg/tool/builtin/write_isolation_test.go pkg/api/runtime_helpers.go
git commit -m "feat: add path classification for session-scoped outputs"
```

### Task 4: Make WriteTool Atomic And Path-Gated

**Files:**
- Modify: `pkg/tool/builtin/file_sandbox.go`
- Modify: `pkg/tool/builtin/write.go`
- Modify: `pkg/api/agent.go`
- Test: `pkg/tool/builtin/write_test.go`
- Test: `pkg/api/runtime_helpers_tools_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestWriteToolRewritesArtifactPathIntoSessionOutputRoot(t *testing.T) {}
func TestWriteToolFailsWhenRepositoryPathLocked(t *testing.T) {}
func TestWriteToolUsesAtomicWrite(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run 'TestWriteToolRewritesArtifactPathIntoSessionOutputRoot|TestWriteToolFailsWhenRepositoryPathLocked|TestWriteToolUsesAtomicWrite'`

Expected: FAIL because `WriteTool` does not know about session output roots or path locks.

**Step 3: Write minimal implementation**

In `pkg/tool/builtin/file_sandbox.go`, add:

```go
func (f *fileSandbox) atomicWriteFile(path string, content string, perm os.FileMode) error
```

In `pkg/tool/builtin/write.go`:

- resolve path via `ResolveWritePath`
- acquire/release runtime path gate for repository-scoped paths
- route session-output paths to session root
- return metadata including `resolved_path`, `scope`, and `session_id`

In `pkg/api/agent.go`, thread session/write-isolation context into tool execution so file tools can see:

```go
session_id
session_output_dir
write_isolation_mode
path_gate
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run 'TestWriteToolRewritesArtifactPathIntoSessionOutputRoot|TestWriteToolFailsWhenRepositoryPathLocked|TestWriteToolUsesAtomicWrite'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/tool/builtin/file_sandbox.go pkg/tool/builtin/write.go pkg/api/agent.go pkg/tool/builtin/write_test.go pkg/api/runtime_helpers_tools_test.go
git commit -m "feat: path-gate write tool and session outputs"
```

### Task 5: Make EditTool Path-Gated

**Files:**
- Modify: `pkg/tool/builtin/edit.go`
- Test: `pkg/tool/builtin/edit_test.go`

**Step 1: Write the failing tests**

Add tests for:

```go
func TestEditToolFailsWhenRepositoryPathLocked(t *testing.T) {}
func TestEditToolWritesAtomically(t *testing.T) {}
func TestEditToolPreservesSessionOutputScope(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run 'TestEditToolFailsWhenRepositoryPathLocked|TestEditToolWritesAtomically|TestEditToolPreservesSessionOutputScope'`

Expected: FAIL because `EditTool` still writes directly.

**Step 3: Write minimal implementation**

Update `pkg/tool/builtin/edit.go` so repository-scoped paths:

- resolve through `ResolveWritePath`
- acquire/release path gate
- write via atomic write helper

Session-output scoped paths should still work and return enriched metadata.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run 'TestEditToolFailsWhenRepositoryPathLocked|TestEditToolWritesAtomically|TestEditToolPreservesSessionOutputScope'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/tool/builtin/edit.go pkg/tool/builtin/edit_test.go
git commit -m "feat: path-gate edit tool writes"
```

### Task 6: Add Read-Side File State Metadata

**Files:**
- Modify: `pkg/tool/builtin/read.go`
- Test: `pkg/tool/builtin/read_test.go`
- Modify: `pkg/model/interface.go` (only if response schema helper types are needed)

**Step 1: Write the failing tests**

Add tests for:

```go
func TestReadToolReturnsFileStateMetadata(t *testing.T) {}
func TestReadToolHashMatchesFileContent(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run 'TestReadToolReturnsFileStateMetadata|TestReadToolHashMatchesFileContent'`

Expected: FAIL because `Read` does not expose file-state proof metadata.

**Step 3: Write minimal implementation**

Extend `Read` output metadata with:

```json
{
  "path": "...",
  "mtime_unix_nano": 123,
  "size": 456,
  "sha256": "..."
}
```

Do not break existing textual output.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run 'TestReadToolReturnsFileStateMetadata|TestReadToolHashMatchesFileContent'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/tool/builtin/read.go pkg/tool/builtin/read_test.go pkg/model/interface.go
git commit -m "feat: expose read-side file state metadata"
```

### Task 7: Add Optional Optimistic Proof Validation For Edit

**Files:**
- Modify: `pkg/tool/builtin/edit.go`
- Modify: `pkg/tool/builtin/edit_test.go`
- Modify: `pkg/tool/schema.go` (only if shared schema helpers are required)

**Step 1: Write the failing tests**

Add tests for:

```go
func TestEditToolRejectsStaleExpectedHash(t *testing.T) {}
func TestEditToolAcceptsMatchingExpectedHash(t *testing.T) {}
func TestEditToolLegacyModeStillWorksWithoutProof(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/tool/builtin -run 'TestEditToolRejectsStaleExpectedHash|TestEditToolAcceptsMatchingExpectedHash|TestEditToolLegacyModeStillWorksWithoutProof'`

Expected: FAIL because expected file-state proof fields are not recognized.

**Step 3: Write minimal implementation**

Extend `Edit` schema and parsing with optional fields:

```go
expected_sha256
expected_size
expected_mtime_unix_nano
```

Validation rules:

- if any expected field is present, compare against current file state before write
- mismatch returns deterministic conflict error
- no proof keeps backward-compatible behavior

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/tool/builtin -run 'TestEditToolRejectsStaleExpectedHash|TestEditToolAcceptsMatchingExpectedHash|TestEditToolLegacyModeStillWorksWithoutProof'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/tool/builtin/edit.go pkg/tool/builtin/edit_test.go pkg/tool/schema.go
git commit -m "feat: add optimistic proof validation for edits"
```

### Task 8: Add Runtime-Level Concurrency And Integration Coverage

**Files:**
- Modify: `pkg/api/agent_concurrent_test.go`
- Create: `test/integration/session_write_isolation_test.go`
- Create: `test/integration/session_output_root_test.go`

**Step 1: Write the failing integration tests**

Cover:

```go
func TestParallelSessionsSameRepositoryPathConflict(t *testing.T) {}
func TestParallelSessionsDifferentRepositoryPathsSucceed(t *testing.T) {}
func TestParallelSessionsArtifactPathsDoNotCollide(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/api -run 'TestParallelSessions'`

Run: `go test -tags=integration ./test/integration -run 'TestParallelSessions(ArtifactPathsDoNotCollide|SameRepositoryPathConflict|DifferentRepositoryPathsSucceed)'`

Expected: FAIL until end-to-end isolation is wired.

**Step 3: Write minimal implementation fixes**

Stabilize any missing runtime plumbing:

- propagate session output root into tool context
- ensure path-gate release on all error paths
- normalize conflict errors for deterministic assertions

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/api -run 'TestParallelSessions'`

Run: `go test -tags=integration ./test/integration -run 'TestParallelSessions(ArtifactPathsDoNotCollide|SameRepositoryPathConflict|DifferentRepositoryPathsSucceed)'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/api/agent_concurrent_test.go test/integration/session_write_isolation_test.go test/integration/session_output_root_test.go
git commit -m "test: cover session write isolation end to end"
```

### Task 9: Document The New Write Isolation Model

**Files:**
- Modify: `README.md`
- Modify: `README_zh.md`
- Modify: `docs/architecture.md`
- Modify: `CHANGELOG.md`

**Step 1: Update docs**

Document:

```md
- write isolation modes: strict/shared/legacy
- session output root behavior
- repository write conflict semantics
- optimistic file-state proof validation for edits
```

**Step 2: Run a docs smoke check**

Run: `rg -n "write isolation|session output|strict|shared|legacy|expected_sha256" README.md README_zh.md docs/architecture.md CHANGELOG.md`

Expected: All user-visible semantics are described in the docs.

**Step 3: Commit**

```bash
git add README.md README_zh.md docs/architecture.md CHANGELOG.md
git commit -m "docs: describe session write isolation"
```

### Task 10: Full Verification

**Files:**
- Verify only

**Step 1: Run focused package tests**

Run: `go test ./pkg/api ./pkg/tool/builtin`

Expected: PASS.

**Step 2: Run integration tests for the new feature**

Run: `go test -tags=integration ./test/integration -run 'TestParallelSessions|TestSessionOutputRoot|TestSharedClikit'`

Expected: PASS, excluding any pre-existing unrelated integration failures.

**Step 3: Run broad repo verification**

Run: `go test ./...`

Expected: PASS, or report only pre-existing unrelated failures with exact test names.

**Step 4: Smoke test CLI stream output**

Run: `go run ./cmd/cli --help`

Expected: exits 0 and still shows `--stream`, `--stream-format`, and `--repl`.

**Step 5: Commit**

```bash
git status --short
```

Expected: clean tree except for unrelated pre-existing files.

### Notes For The Implementer

- Do not try to make the first slice distributed or cross-process. Runtime-local isolation is enough for the first implementation.
- Keep `legacy` mode available at first even if `strict` is the long-term default.
- Avoid schema churn unless it materially improves safety. `Edit` proof fields can stay optional for the first rollout.
- Reuse existing sanitization and session directory helpers instead of inventing a second path-normalization scheme.
- Do not couple this feature to task store isolation; that is a separate design track.
