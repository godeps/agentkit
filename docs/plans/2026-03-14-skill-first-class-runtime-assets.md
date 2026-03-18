# Skill First-Class Runtime Assets Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade `agentkit` skills from prompt-oriented helper artifacts into explicit runtime assets with identity, scope, loading outcomes, activation/injection boundaries, and basic telemetry.

**Architecture:** Keep the current Go runtime compact and embeddable. Do not import Codex's full product surface. Instead, introduce a small `SkillLoadOutcome` model, canonical skill identity, richer metadata, and a clearer skill pipeline split between load, match, inject, and execute. Preserve backward compatibility for the existing `Skill` tool and `ForceSkills` request flow while incrementally tightening internal semantics.

**Tech Stack:** Go, `go test`, existing `pkg/runtime/skills`, `pkg/api`, `pkg/tool/builtin`, Markdown docs

---

### Task 1: Freeze Current Skill Behavior With Focused Tests

**Files:**
- Modify: `pkg/runtime/skills/loader_test.go`
- Modify: `pkg/runtime/skills/registry_test.go`
- Modify: `pkg/tool/builtin/skill_test.go`
- Create: `pkg/api/skills_pipeline_test.go`

**Step 1: Add a loader test for duplicate skill names across roots**

```go
func TestLoadFromFSDuplicateNamesAcrossRoots(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a", "dup")
	dirB := filepath.Join(root, "b", "dup")
	writeSkill(t, filepath.Join(dirA, "SKILL.md"), "dup", "a")
	writeSkill(t, filepath.Join(dirB, "SKILL.md"), "dup", "b")

	regs, errs := LoadFromFS(LoaderOptions{
		ProjectRoot: root,
		Directories: []string{filepath.Join(root, "a"), filepath.Join(root, "b")},
	})

	if len(regs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(regs))
	}
	if len(errs) == 0 {
		t.Fatalf("expected duplicate warning")
	}
}
```

**Step 2: Run the loader test**

Run: `go test ./pkg/runtime/skills -run TestLoadFromFSDuplicateNamesAcrossRoots -v`
Expected: PASS

**Step 3: Add an API pipeline test for auto-match + forced-skill dedupe**

```go
func TestExecuteSkillsDedupesAutoAndForcedMatches(t *testing.T) {
	// Build a runtime with one matching skill and also request it via ForceSkills.
	// Assert only one execution record is emitted.
}
```

**Step 4: Run the API pipeline test**

Run: `go test ./pkg/api -run TestExecuteSkillsDedupesAutoAndForcedMatches -v`
Expected: PASS

**Step 5: Add a `Skill` tool test that manual execution still works by name**

```go
func TestSkillToolExecutesByNameAfterRefactor(t *testing.T) {
	// Register one skill and assert the output path is unchanged.
}
```

**Step 6: Run the skill tool test**

Run: `go test ./pkg/tool/builtin -run TestSkillToolExecutesByNameAfterRefactor -v`
Expected: PASS

**Step 7: Commit**

```bash
git add pkg/runtime/skills/loader_test.go pkg/runtime/skills/registry_test.go pkg/tool/builtin/skill_test.go pkg/api/skills_pipeline_test.go
git commit -m "test: freeze current skill pipeline behavior"
```

### Task 2: Introduce `SkillLoadOutcome`

**Files:**
- Create: `pkg/runtime/skills/outcome.go`
- Modify: `pkg/runtime/skills/loader.go`
- Modify: `pkg/api/runtime_helpers.go`
- Modify: `pkg/runtime/skills/loader_test.go`

**Step 1: Write a failing test for load outcome scope and errors**

```go
func TestLoadOutcomeCarriesRegistrationsAndErrors(t *testing.T) {
	outcome := LoadOutcomeFromFS(LoaderOptions{ProjectRoot: t.TempDir()})
	if outcome == nil {
		t.Fatal("expected outcome")
	}
}
```

**Step 2: Run the failing test**

Run: `go test ./pkg/runtime/skills -run TestLoadOutcomeCarriesRegistrationsAndErrors -v`
Expected: FAIL with undefined `LoadOutcomeFromFS`

**Step 3: Create `outcome.go` with an explicit outcome model**

```go
type SkillScope string

const (
	SkillScopeRepo   SkillScope = "repo"
	SkillScopeUser   SkillScope = "user"
	SkillScopeSystem SkillScope = "system"
	SkillScopeCustom SkillScope = "custom"
)

type LoadOrigin struct {
	Path  string
	Scope SkillScope
}

type SkillLoadOutcome struct {
	Registrations []SkillRegistration
	Errors        []error
	Origins       map[string]LoadOrigin
}
```

**Step 4: Implement `LoadOutcomeFromFS()` in `loader.go`**

```go
func LoadOutcomeFromFS(opts LoaderOptions) *SkillLoadOutcome {
	// Reuse existing scan logic.
	// Populate registrations, errors, and origin metadata.
}
```

**Step 5: Keep `LoadFromFS()` as a compatibility wrapper**

```go
func LoadFromFS(opts LoaderOptions) ([]SkillRegistration, []error) {
	outcome := LoadOutcomeFromFS(opts)
	if outcome == nil {
		return nil, nil
	}
	return outcome.Registrations, outcome.Errors
}
```

**Step 6: Update `pkg/api/runtime_helpers.go` to consume the new outcome internally**

```go
outcome := skills.LoadOutcomeFromFS(...)
fsRegs, errs := outcome.Registrations, outcome.Errors
```

**Step 7: Run focused tests**

Run: `go test ./pkg/runtime/skills ./pkg/api -run 'TestLoadOutcomeCarriesRegistrationsAndErrors|TestLoadFromFS' -v`
Expected: PASS

**Step 8: Commit**

```bash
git add pkg/runtime/skills/outcome.go pkg/runtime/skills/loader.go pkg/runtime/skills/loader_test.go pkg/api/runtime_helpers.go
git commit -m "refactor: add explicit skill load outcome"
```

### Task 3: Add Canonical Skill Identity, Scope, and Origin

**Files:**
- Modify: `pkg/runtime/skills/registry.go`
- Modify: `pkg/runtime/skills/loader.go`
- Modify: `pkg/tool/builtin/skill.go`
- Modify: `pkg/runtime/skills/registry_test.go`
- Modify: `pkg/tool/builtin/skill_test.go`

**Step 1: Write a failing registry test for origin metadata**

```go
func TestRegistryListIncludesSkillOriginMetadata(t *testing.T) {
	// Register a definition with origin metadata and assert it survives List().
}
```

**Step 2: Run the failing test**

Run: `go test ./pkg/runtime/skills -run TestRegistryListIncludesSkillOriginMetadata -v`
Expected: FAIL

**Step 3: Standardize reserved metadata keys**

```go
const (
	MetadataKeySkillPath   = "skill.path"
	MetadataKeySkillScope  = "skill.scope"
	MetadataKeySkillOrigin = "skill.origin"
	MetadataKeySkillID     = "skill.id"
)
```

**Step 4: Update loader metadata assembly**

```go
func buildDefinitionMetadata(file SkillFile) map[string]string {
	return map[string]string{
		MetadataKeySkillPath: file.Path,
		MetadataKeySkillScope: string(resolveSkillScope(file.Path, ...)),
		MetadataKeySkillOrigin: "filesystem",
		MetadataKeySkillID: canonicalSkillID(file),
	}
}
```

**Step 5: Update the `Skill` tool description to show canonical identity fields when present**

```go
// Prefer skill.path and skill.scope when rendering available skills.
```

**Step 6: Keep manual execution name-based but prepare internal path-aware lookup helpers**

```go
func skillID(def skills.Definition) string { ... }
```

**Step 7: Run focused tests**

Run: `go test ./pkg/runtime/skills ./pkg/tool/builtin -run 'TestRegistryListIncludesSkillOriginMetadata|TestSkillTool' -v`
Expected: PASS

**Step 8: Commit**

```bash
git add pkg/runtime/skills/registry.go pkg/runtime/skills/loader.go pkg/tool/builtin/skill.go pkg/runtime/skills/registry_test.go pkg/tool/builtin/skill_test.go
git commit -m "feat: add canonical skill identity metadata"
```

### Task 4: Separate Skill Activation From Prompt Injection

**Files:**
- Create: `pkg/runtime/skills/injection.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/runtime/skills/registry.go`
- Create: `pkg/runtime/skills/injection_test.go`
- Modify: `pkg/api/skills_pipeline_test.go`

**Step 1: Write a failing injection unit test**

```go
func TestBuildPromptInjectionCombinesSkillOutputs(t *testing.T) {
	results := []skills.Result{
		{Skill: "a", Output: "first"},
		{Skill: "b", Output: "second"},
	}
	got := BuildPromptInjection(results, nil)
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Fatalf("unexpected injection: %q", got)
	}
}
```

**Step 2: Run the failing test**

Run: `go test ./pkg/runtime/skills -run TestBuildPromptInjectionCombinesSkillOutputs -v`
Expected: FAIL with undefined `BuildPromptInjection`

**Step 3: Create `injection.go`**

```go
type Injection struct {
	PromptPrefix string
	Metadata     map[string]any
}

func BuildPromptInjection(results []Result, base map[string]any) Injection {
	// Merge prompt output and metadata without executing anything.
}
```

**Step 4: Refactor `executeSkills()` to use the new injection helper**

```go
// Match and execute skills.
// Collect results.
// Build prompt injection separately.
// Apply injection to prompt and request metadata.
```

**Step 5: Keep external response behavior unchanged**

```go
// Preserve SkillExecution output and prompt mutation semantics.
```

**Step 6: Run focused tests**

Run: `go test ./pkg/runtime/skills ./pkg/api -run 'TestBuildPromptInjectionCombinesSkillOutputs|TestExecuteSkills' -v`
Expected: PASS

**Step 7: Commit**

```bash
git add pkg/runtime/skills/injection.go pkg/runtime/skills/injection_test.go pkg/api/agent.go pkg/api/skills_pipeline_test.go
git commit -m "refactor: split skill activation from prompt injection"
```

### Task 5: Add Minimal Skill Policy and Dependencies Metadata

**Files:**
- Modify: `pkg/runtime/skills/loader.go`
- Create: `pkg/runtime/skills/metadata.go`
- Create: `pkg/runtime/skills/metadata_test.go`
- Modify: `pkg/runtime/skills/loader_test.go`
- Modify: `docs/plans/2026-03-14-agentkit-vs-codex-skills-analysis.md`

**Step 1: Write a failing metadata parser test**

```go
func TestParseSkillMetadataPolicyAndDependencies(t *testing.T) {
	contents := `---
name: test-skill
description: demo
metadata:
  policy.allow_implicit_invocation: "false"
  dependencies.tools: "bash,glob"
---
body`
	meta, _, err := parseFrontMatter(contents)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Metadata["policy.allow_implicit_invocation"] != "false" {
		t.Fatalf("unexpected metadata: %#v", meta.Metadata)
	}
}
```

**Step 2: Run the failing test**

Run: `go test ./pkg/runtime/skills -run TestParseSkillMetadataPolicyAndDependencies -v`
Expected: FAIL

**Step 3: Add structured metadata helpers**

```go
type SkillPolicy struct {
	AllowImplicitInvocation *bool
}

type SkillDependencies struct {
	Tools []string
}

func PolicyFromDefinition(def Definition) SkillPolicy { ... }
func DependenciesFromDefinition(def Definition) SkillDependencies { ... }
```

**Step 4: Map frontmatter into stable metadata keys without breaking existing schema**

```go
// Reuse `metadata` map for compatibility.
// Normalize `policy.*` and `dependencies.*` keys.
```

**Step 5: Document the minimal metadata contract in the analysis doc**

```markdown
- `skill.scope`
- `skill.path`
- `skill.id`
- `policy.allow_implicit_invocation`
- `dependencies.tools`
```

**Step 6: Run focused tests**

Run: `go test ./pkg/runtime/skills -run 'TestParseSkillMetadataPolicyAndDependencies|TestLoadFromFS' -v`
Expected: PASS

**Step 7: Commit**

```bash
git add pkg/runtime/skills/loader.go pkg/runtime/skills/metadata.go pkg/runtime/skills/metadata_test.go pkg/runtime/skills/loader_test.go docs/plans/2026-03-14-agentkit-vs-codex-skills-analysis.md
git commit -m "feat: add minimal structured skill metadata"
```

### Task 6: Add Skill Telemetry Hooks

**Files:**
- Create: `pkg/runtime/skills/telemetry.go`
- Modify: `pkg/api/agent.go`
- Modify: `pkg/runtime/skills/registry.go`
- Create: `pkg/runtime/skills/telemetry_test.go`
- Modify: `pkg/api/skills_pipeline_test.go`

**Step 1: Write a failing telemetry test**

```go
func TestSkillTelemetryRecordsMatchAndExecute(t *testing.T) {
	var events []Event
	recorder := func(e Event) { events = append(events, e) }
	// Execute a matching skill and assert both phases are recorded.
}
```

**Step 2: Run the failing test**

Run: `go test ./pkg/runtime/skills -run TestSkillTelemetryRecordsMatchAndExecute -v`
Expected: FAIL

**Step 3: Add a minimal telemetry interface**

```go
type EventType string

const (
	EventSkillMatched  EventType = "skill_matched"
	EventSkillForced   EventType = "skill_forced"
	EventSkillExecuted EventType = "skill_executed"
	EventSkillFailed   EventType = "skill_failed"
)

type Event struct {
	Type    EventType
	SkillID string
	Name    string
}
```

**Step 4: Emit telemetry from match and execute paths**

```go
// Emit in registry match path and runtime execution path.
```

**Step 5: Keep telemetry optional and no-op by default**

```go
type Recorder func(Event)
```

**Step 6: Run focused tests**

Run: `go test ./pkg/runtime/skills ./pkg/api -run 'TestSkillTelemetryRecordsMatchAndExecute|TestExecuteSkills' -v`
Expected: PASS

**Step 7: Commit**

```bash
git add pkg/runtime/skills/telemetry.go pkg/runtime/skills/telemetry_test.go pkg/runtime/skills/registry.go pkg/api/agent.go pkg/api/skills_pipeline_test.go
git commit -m "feat: add skill telemetry hooks"
```

### Task 7: Document the New Skill Runtime Contract

**Files:**
- Modify: `README.md`
- Modify: `README_zh.md`
- Create: `docs/skills-runtime.md`

**Step 1: Write docs for the new contract**

```markdown
## Skill Runtime Model

- Load outcome
- Canonical identity
- Scope and origin
- Activation vs injection
- Structured metadata
- Telemetry
```

**Step 2: Link docs from the top-level README files**

```markdown
For skill runtime behavior, see `docs/skills-runtime.md`.
```

**Step 3: Run documentation sanity checks**

Run: `rg -n "skills-runtime|Skill Runtime Model|canonical identity" README.md README_zh.md docs/skills-runtime.md`
Expected: matching lines present in all three files

**Step 4: Commit**

```bash
git add README.md README_zh.md docs/skills-runtime.md
git commit -m "docs: describe skill runtime contract"
```

### Task 8: Full Verification Sweep

**Files:**
- Test: `pkg/runtime/skills/...`
- Test: `pkg/api/...`
- Test: `pkg/tool/builtin/...`

**Step 1: Run the focused package tests**

Run: `go test ./pkg/runtime/skills ./pkg/api ./pkg/tool/builtin`
Expected: PASS

**Step 2: Run a race check for affected packages**

Run: `go test -race ./pkg/runtime/skills ./pkg/api ./pkg/tool/builtin`
Expected: PASS

**Step 3: Run a broader smoke test**

Run: `go test ./...`
Expected: PASS

**Step 4: Final commit**

```bash
git add pkg/runtime/skills pkg/api pkg/tool/builtin README.md README_zh.md docs/skills-runtime.md docs/plans/2026-03-14-agentkit-vs-codex-skills-analysis.md docs/plans/2026-03-14-skill-first-class-runtime-assets.md
git commit -m "refactor: promote skills to first-class runtime assets"
```
