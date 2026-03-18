# Skill First-Class Runtime Assets Design

## Goal

Make `agentkit` skills first-class runtime assets without turning the runtime into a clone of `other/codex`.

The design should preserve the current strengths of `agentkit`:

- compact embedding surface
- simple `SKILL.md`-based authoring
- backward-compatible `Skill` tool and `ForceSkills`

At the same time, it should remove the main current limitation: skills are treated mostly as prompt-oriented helpers instead of explicit runtime objects.

## Problem

Today a skill in `agentkit` is effectively:

- loaded from `SKILL.md`
- converted into `skills.Definition`
- matched against request context
- executed
- its output is prepended to the prompt

That works, but it leaves the runtime blind to several things:

- canonical identity
- source and scope
- disabled/overridden state
- explicit load outcome
- the difference between activation and injection
- structured metadata beyond a loose string map

This becomes fragile once the project has:

- more than one skill source
- future user/system/project layering
- stronger policy or permission semantics
- more skills and more collisions

## Non-Goals

- do not add a full Codex-style plugin ecosystem
- do not add TUI-specific skill UI metadata
- do not redesign the public `SKILL.md` format in a breaking way
- do not replace the current `Skill` tool invocation path

## Current Flow

1. `pkg/runtime/skills/loader.go` scans roots and returns registrations.
2. `pkg/runtime/skills/registry.go` stores definitions and handlers.
3. `pkg/api/agent.go` matches skills and executes them before model invocation.
4. `pkg/tool/builtin/skill.go` exposes manual skill execution to the model.

This means the core runtime contract for skills is spread across loader, registry, runtime, and tool layers, but there is no explicit asset model tying them together.

## Target Model

The target model is:

1. **Load**: skill discovery returns a `SkillLoadOutcome`, not only registrations.
2. **Identify**: every skill has stable canonical metadata, including path, scope, origin, and id.
3. **Activate**: skill matching decides which skills apply to the request.
4. **Inject**: a separate injection step materializes prompt prefix and metadata updates.
5. **Execute**: execution records results against stable skill identity.
6. **Observe**: telemetry can attribute match/force/execute/fail to a skill id.

## Proposed Runtime Types

### `SkillLoadOutcome`

This should become the main loader return type.

Proposed shape:

```go
type SkillScope string

const (
	SkillScopeRepo   SkillScope = "repo"
	SkillScopeUser   SkillScope = "user"
	SkillScopeSystem SkillScope = "system"
	SkillScopeCustom SkillScope = "custom"
)

type LoadOrigin struct {
	Path   string
	Scope  SkillScope
	Origin string
}

type SkillLoadOutcome struct {
	Registrations []SkillRegistration
	Errors        []error
	Origins       map[string]LoadOrigin
}
```

Notes:

- key for `Origins` should be canonical skill id, not only skill name
- `LoadFromFS()` remains as a compatibility wrapper

### Canonical metadata keys

Keep `Definition.Metadata` as the compatibility carrier, but reserve stable keys:

```go
const (
	MetadataKeySkillPath   = "skill.path"
	MetadataKeySkillScope  = "skill.scope"
	MetadataKeySkillOrigin = "skill.origin"
	MetadataKeySkillID     = "skill.id"
)
```

This lets the rest of the runtime rely on a common contract without immediately changing public APIs.

## Activation vs Injection

The current runtime combines these responsibilities too tightly.

Current behavior:

- match a skill
- execute it
- concatenate outputs into a prompt prefix
- merge metadata during the same pass

Target behavior:

- **Activation** decides which skills apply.
- **Execution** produces `skills.Result`.
- **Injection** converts results into prompt and metadata changes.

This allows future extensions:

- non-prompt skill actions
- structured injection
- conditional metadata application
- prompt dedupe/caching

## Compatibility Strategy

Compatibility rules for the first implementation phase:

- `LoadFromFS()` keeps working unchanged for callers
- `Skill` tool still executes by skill name
- `Request.ForceSkills` stays name-based externally
- `skills.Definition.Metadata` remains the integration point for new identity fields
- existing `skills.Result.Output` prompt behavior stays intact

The implementation should improve internal semantics before changing external APIs.

## Minimal Metadata Enrichment

Phase 1 should only add metadata the runtime will actually use:

- `skill.path`
- `skill.scope`
- `skill.origin`
- `skill.id`

Phase 2 can add:

- `policy.allow_implicit_invocation`
- `dependencies.tools`

This keeps YAGNI discipline and avoids importing Codex's full skill schema prematurely.

## Telemetry Contract

Telemetry should stay optional and cheap.

Minimal events:

- `skill_matched`
- `skill_forced`
- `skill_executed`
- `skill_failed`

Minimal payload:

- `skill.id`
- `skill.name`
- event type

This is enough to understand runtime behavior and guide future optimization.

## Risks

### Risk: hidden compatibility regressions

Mitigation:

- freeze current behavior with focused tests before code changes
- keep old loader API as wrapper
- preserve name-based manual invocation

### Risk: metadata bloat without runtime use

Mitigation:

- only add keys the runtime immediately consumes
- do not introduce UI or plugin metadata yet

### Risk: overfitting to Codex

Mitigation:

- borrow only the structural lessons
- avoid product-surface complexity such as plugin UI and deep mode coupling

## Implementation Sequence

1. Freeze current skill behavior with tests.
2. Add `SkillLoadOutcome`.
3. Add canonical identity metadata.
4. Split activation from injection.
5. Add minimal policy/dependency helpers.
6. Add telemetry hooks.

## Success Criteria

This design is successful when:

- the runtime can explain which skill was loaded and from where
- the runtime can distinguish match, execute, and inject phases
- callers still use the old public interfaces without breakage
- future policy and interaction features have an obvious place to attach
