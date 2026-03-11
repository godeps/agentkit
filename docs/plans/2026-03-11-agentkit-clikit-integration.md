# Agentkit Clikit Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Internalize the reusable parts of `../../alicloud-skills/apps/pkg/clikit` into `agentkit` without creating a reverse module dependency on `alicloud-skills`.

**Architecture:** Extract `clikit` into an `agentkit`-owned CLI support package, remove product-specific branding and direct `os.Stdout` assumptions, then add a thin adapter around `pkg/api.Runtime` so the existing CLI can reuse the richer REPL, waterfall, and config rendering flows. Do not import `alicloud-skills/internal/agent`; reimplement only the minimal runtime-facing pieces needed by the shared CLI package.

**Tech Stack:** Go 1.24, `flag`, `pkg/api`, `pkg/middleware`, `pkg/runtime/skills`, `github.com/chzyer/readline`, `github.com/google/uuid`, Go tests

---

### Task 1: Freeze Scope And Target Package Boundary

**Files:**
- Create: `docs/plans/2026-03-11-agentkit-clikit-integration-design.md`
- Modify: `docs/architecture.md`

**Step 1: Write the design note**

Document these decisions:

```md
- `agentkit` must not import `github.com/cinience/alicloud-skills/...`
- shared package name should be `pkg/clikit` or `pkg/cliui`
- scope for first pass: REPL, waterfall tracing, effective-config printing
- out of scope: Alibaba branding, DashScope defaults, autonomy policy, skill prompt enrichment
```

**Step 2: Record architectural rationale**

Add a short section to `docs/architecture.md` describing:

```md
The CLI support layer is owned by `agentkit` and may be consumed by downstream products.
Downstream applications may wrap `pkg/api.Runtime` with their own policy and prompt behavior.
```

**Step 3: Review for drift**

Run: `rg -n "clikit|cli support|downstream" docs/architecture.md docs/plans/2026-03-11-agentkit-clikit-integration-design.md`

Expected: the new package boundary and non-goals are explicitly described.

**Step 4: Commit**

```bash
git add docs/architecture.md docs/plans/2026-03-11-agentkit-clikit-integration-design.md
git commit -m "docs: define agentkit cli support boundary"
```

### Task 2: Vendor The Reusable CLI Package Into Agentkit

**Files:**
- Create: `pkg/clikit/types.go`
- Create: `pkg/clikit/config.go`
- Create: `pkg/clikit/render.go`
- Create: `pkg/clikit/run_stream.go`
- Create: `pkg/clikit/repl.go`
- Create: `pkg/clikit/waterfall.go`
- Create: `pkg/clikit/artifact.go`
- Create: `pkg/clikit/output_validation.go`
- Create: `pkg/clikit/output_validation_test.go`
- Create: `pkg/clikit/repl_test.go`
- Create: `pkg/clikit/stream_test.go`
- Create: `pkg/clikit/architecture_test.go`

**Step 1: Write the failing package-level tests**

Seed tests for the non-product-specific behavior:

```go
func TestNormalizeWaterfallMode(t *testing.T) {}
func TestHandleCommandListsSkills(t *testing.T) {}
func TestDetectArtifactInfoExtractsPath(t *testing.T) {}
func TestChooseValidationPathsUsesLLMPathsFirst(t *testing.T) {}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/clikit -run 'TestNormalizeWaterfallMode|TestHandleCommandListsSkills|TestDetectArtifactInfoExtractsPath|TestChooseValidationPathsUsesLLMPathsFirst'`

Expected: FAIL because `pkg/clikit` does not exist yet.

**Step 3: Copy the reusable implementation with neutral defaults**

Port code from `../../alicloud-skills/apps/pkg/clikit`, but make these changes during import:

```go
- rename banner text to `Agentkit CLI`
- change `PrintBanner` to accept `io.Writer`
- change `RunStream` and `RunREPL` to take output/error writers instead of writing directly to process stdio
- keep `api.StreamEvent`-based interfaces
- keep waterfall and artifact helpers package-private unless needed elsewhere
```

**Step 4: Run package tests**

Run: `go test ./pkg/clikit`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/clikit
git commit -m "feat: add reusable cli support package"
```

### Task 3: Add An Agentkit Runtime Adapter For Clikit

**Files:**
- Create: `pkg/clikit/runtime_adapter.go`
- Create: `pkg/clikit/runtime_adapter_test.go`
- Modify: `pkg/api/runtime_helpers.go`
- Modify: `pkg/runtime/skills/registry.go`

**Step 1: Write the failing adapter tests**

Cover the adapter contract:

```go
func TestRuntimeAdapterExposesModelNameAndRepoRoot(t *testing.T) {}
func TestRuntimeAdapterReturnsDiscoveredSkills(t *testing.T) {}
func TestRuntimeAdapterTracksModelTurnsAcrossStreamRuns(t *testing.T) {}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/clikit -run 'TestRuntimeAdapterExposesModelNameAndRepoRoot|TestRuntimeAdapterReturnsDiscoveredSkills|TestRuntimeAdapterTracksModelTurnsAcrossStreamRuns'`

Expected: FAIL because the adapter and turn recorder do not exist.

**Step 3: Implement the minimal adapter**

Implement a wrapper around `*api.Runtime` plus the resolved runtime options:

```go
type RuntimeAdapter struct {
    runtime      *api.Runtime
    projectRoot  string
    configRoot   string
    modelName    string
    skillsDirs   []string
    recursive    bool
    turnRecorder *turnRecorder
}
```

Requirements:

```go
- `RunStream()` delegates to `runtime.RunStream()`
- middleware records token usage and preview per session
- `Skills()` uses agentkit skill discovery, not downstream code
- `RepoRoot()`, `SettingsRoot()`, `SkillsDirs()`, `SkillsRecursive()`, `ModelName()` are deterministic
```

**Step 4: Run targeted tests**

Run: `go test ./pkg/clikit -run 'TestRuntimeAdapter'`

Expected: PASS.

**Step 5: Commit**

```bash
git add pkg/clikit/runtime_adapter.go pkg/clikit/runtime_adapter_test.go pkg/api/runtime_helpers.go pkg/runtime/skills/registry.go
git commit -m "feat: add runtime adapter for cli support"
```

### Task 4: Wire The Existing CLI To The Shared Package

**Files:**
- Modify: `cmd/cli/main.go`
- Modify: `cmd/cli/main_test.go`

**Step 1: Write the failing CLI tests**

Add tests for the new behavior:

```go
func TestRunPrintsSharedEffectiveConfig(t *testing.T) {}
func TestRunStreamUsesClikitRendererWhenEnabled(t *testing.T) {}
func TestCLIReplUsesSharedBannerAndCommandLoop(t *testing.T) {}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./cmd/cli -run 'TestRunPrintsSharedEffectiveConfig|TestRunStreamUsesClikitRendererWhenEnabled|TestCLIReplUsesSharedBannerAndCommandLoop'`

Expected: FAIL because `cmd/cli` still uses the old JSON stream and plain text response path.

**Step 3: Implement the CLI migration**

Update the CLI to:

```go
- construct the `pkg/clikit` runtime adapter after `api.New()`
- route `--print-effective-config` through shared printer helpers
- add an explicit REPL subcommand or mode using `clikit.RunREPL`
- preserve existing ACP mode and non-stream one-shot behavior unless intentionally replaced
- keep backward compatibility for existing flags unless a flag rename is deliberate and documented
```

**Step 4: Run CLI tests**

Run: `go test ./cmd/cli`

Expected: PASS.

**Step 5: Commit**

```bash
git add cmd/cli/main.go cmd/cli/main_test.go
git commit -m "feat: wire cli through shared clikit package"
```

### Task 5: Validate End-To-End Behavior

**Files:**
- Modify: `test/integration/claude_code_compat_test.go`
- Create: `test/integration/cli_repl_shared_test.go`
- Create: `test/integration/cli_stream_render_test.go`

**Step 1: Write the failing integration tests**

Cover:

```go
func TestSharedCLIStreamRendersToolProgress(t *testing.T) {}
func TestSharedCLIReplSupportsSlashCommands(t *testing.T) {}
func TestSharedCLIConfigOutputShowsSkillsDirs(t *testing.T) {}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./test/integration -run 'TestSharedCLI(StreamRendersToolProgress|ReplSupportsSlashCommands|ConfigOutputShowsSkillsDirs)'`

Expected: FAIL until the CLI is wired and output is stabilized.

**Step 3: Implement any missing stabilization**

Typical fixes:

```go
- normalize ANSI behavior in tests
- inject deterministic session IDs
- avoid direct `os.Stdout` dependencies in shared package
```

**Step 4: Run integration tests**

Run: `go test ./test/integration -run 'TestSharedCLI(StreamRendersToolProgress|ReplSupportsSlashCommands|ConfigOutputShowsSkillsDirs)'`

Expected: PASS.

**Step 5: Commit**

```bash
git add test/integration
git commit -m "test: cover shared cli integration flows"
```

### Task 6: Full Verification And Migration Notes

**Files:**
- Modify: `README.md`
- Modify: `README_zh.md`
- Modify: `CHANGELOG.md`

**Step 1: Update user-facing docs**

Document:

```md
- shared CLI rendering behavior
- REPL availability and slash commands
- any changed flags or output format expectations
- downstream guidance for consumers that want to reuse `pkg/clikit`
```

**Step 2: Run full verification**

Run: `go test ./...`

Expected: PASS.

**Step 3: Smoke test the binary**

Run: `go run ./cmd/cli --help`

Expected: help output includes the intended stream/REPL/config entry points and exits with status 0.

**Step 4: Commit**

```bash
git add README.md README_zh.md CHANGELOG.md
git commit -m "docs: document shared cli support package"
```

### Notes For The Implementer

- Do not import `../../alicloud-skills/...` from `agentkit`; treat that repo only as a reference source during migration.
- Keep the first migration small. Reuse the `clikit` package only where it improves `agentkit` directly.
- If the current `cmd/cli` contract is intentionally stable, keep legacy output mode behind a flag and introduce the new renderer opt-in first.
- Prefer injecting writers and deterministic dependencies everywhere. The downstream package currently writes to global stdio; that should not survive the port.
- If token usage cannot be recorded cleanly from existing middleware hooks, add the smallest internal helper in `pkg/api` rather than duplicating runtime internals in `cmd/cli`.
