# agentkit vs codex: runtime and skills analysis

## Purpose

This document records two things:

1. The current high-level optimization advice for `agentkit` after comparing it with `other/codex`.
2. A focused comparison of the skills pipeline in both projects, including concrete optimization space for `agentkit`.

The goal is not to copy `codex` wholesale. The goal is to identify where `agentkit` can improve its runtime kernel and its skill system without importing product complexity that does not fit the current repository.

## Scope

- `agentkit` code under `pkg/api`, `pkg/runtime/skills`, `pkg/tool/builtin`, `cmd/cli`
- reference implementation under `other/codex/codex-rs/core/src/skills`, `other/codex/codex-rs/core/src/tools/handlers`, `other/codex/codex-rs/tui`

## Overall assessment

`agentkit` already has broad runtime capability coverage: tools, hooks, sandbox, MCP, commands, skills, subagents, tasks, CLI, ACP, and tests. The main weakness is not missing features. The main weakness is concentration of orchestration complexity in `pkg/api`.

Compared with `other/codex`, `agentkit` is stronger as a compact embeddable Go runtime, but weaker in three areas:

- boundary discipline between core runtime and product surfaces
- declarative skill metadata richness and policy integration
- skill UX loop across discovery, activation, permissioning, and user interaction

## Part I: overall optimization advice

### 1. Split `pkg/api` into thinner layers

`Runtime` currently owns almost every major subsystem directly: settings, sandbox, registry, executor, hooks, histories, commands, skills, subagents, tasks, compactor, tracer, and lifecycle state in one struct.

Evidence:

- `Runtime` field concentration in [pkg/api/agent.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/api/agent.go#L58)
- constructor assembly concentration in [pkg/api/agent.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/api/agent.go#L97)

This should be refactored into three layers:

- `api` as public facade
- bootstrap/composition layer for wiring config and runtime subsystems
- runtime core for execution lifecycle

Why this matters:

- easier embedding in CLI, ACP, HTTP, CI wrappers
- smaller public API blast radius
- simpler lifecycle reasoning and testing

### 2. Break down `api.Options`

`api.Options` currently mixes entrypoint metadata, config loading, model selection, built-in tool selection, skills, hooks, sandbox, approvals, compacting, and OTEL in one type.

Evidence:

- [pkg/api/options.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/api/options.go#L158)

Recommended split:

- `RuntimeOptions`
- `PolicyOptions`
- `ExtensionOptions`
- `ObservabilityOptions`
- entrypoint-specific mapping structs for CLI and ACP

The current single struct is convenient early on, but it encourages cross-layer coupling and hidden precedence rules.

### 3. Move extension loading rules out of `api`

`pkg/api/runtime_helpers.go` currently implements loading and merge behavior for commands, skills, and subagents.

Evidence:

- command and skill loader assembly in [pkg/api/runtime_helpers.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/api/runtime_helpers.go#L194)

That logic belongs primarily in the owning packages:

- `pkg/runtime/commands`
- `pkg/runtime/skills`
- `pkg/runtime/subagents`

`api` should consume a stable contract, not define merge semantics for every extension type.

### 4. Isolate product entrypoints from runtime kernel

The CLI currently understands runtime setup, sandbox backend selection, govm preflight, ACP serving, REPL routing, and prompt resolution in one flow.

Evidence:

- [cmd/cli/main.go](/home/vipas/workspace/saker-ai/godeps/agentkit/cmd/cli/main.go#L1)

This should move toward:

- `cmd/cli` only parses args and delegates
- `pkg/app/cli` or `pkg/bootstrap/cli` handles product wiring
- `pkg/api` remains runtime-facing

This is the same direction `other/codex` follows with thin entry binaries and deeper module boundaries.

### 5. Introduce a persistent run/event ledger

`agentkit` has history persistence, but not a strong persistent execution ledger. `other/codex` already treats state storage as a first-class concept and documents a SQLite-backed state DB.

Evidence:

- `other/codex` config documentation mentions a state DB in [other/codex/docs/config.md](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/docs/config.md#L27)

Recommended direction:

- persist request lifecycle events
- persist skill/tool/subagent dispatch records
- persist approval and denial decisions
- make replay/debug/audit feasible without log scraping

### 6. Normalize sandbox and approval into a single request policy snapshot

Today the policy surface is spread across sandbox options, permission handler, approval queue, whitelist TTL, and runtime-request behavior.

Evidence:

- policy-related fields in [pkg/api/options.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/api/options.go#L239)

Recommended direction:

- build an immutable per-request policy snapshot before execution
- pass the same snapshot to tool execution, sandbox enforcement, approval checks, and tracing
- emit denials and approvals against that normalized object

This will reduce precedence ambiguity and make observability more coherent.

## Part II: skills flow comparison

## agentkit current skills flow

At a high level, the `agentkit` skills path is:

1. Load `SKILL.md` files from configured roots.
2. Convert frontmatter into `skills.Definition` plus a lazy handler.
3. Register into `skills.Registry`.
4. Build an activation context from the request.
5. Auto-match skills and append any manually forced skills.
6. Execute matched skills and prepend their output into the prompt.
7. Expose a `Skill` tool for explicit invocation by the model.

### Loading

`LoadFromFS()` scans roots, parses frontmatter, validates names, deduplicates by skill name, and creates lazy handlers.

Evidence:

- loader options and root handling in [pkg/runtime/skills/loader.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/runtime/skills/loader.go#L57)
- registration assembly in [pkg/runtime/skills/loader.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/runtime/skills/loader.go#L161)

### Registry and matching

`skills.Registry` stores definitions and handlers, supports manual execution, and resolves auto-activation by `Priority`, `MutexKey`, and matcher score.

Evidence:

- registry definition in [pkg/runtime/skills/registry.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/runtime/skills/registry.go#L118)
- match logic in [pkg/runtime/skills/registry.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/runtime/skills/registry.go#L182)

Matchers are intentionally simple:

- keyword matching on prompt text
- tag matching
- trait matching

Evidence:

- [pkg/runtime/skills/matcher.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/runtime/skills/matcher.go#L10)

### Runtime execution

The runtime resolves auto-matches, merges `ForceSkills`, executes each skill, collects metadata, and prepends resulting output into the prompt before model invocation.

Evidence:

- [pkg/api/agent.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/api/agent.go#L802)

This is straightforward and usable, but it means skill outputs are mostly prompt-prefix transformers plus metadata mutators.

### Explicit invocation by tool

The built-in `Skill` tool publishes available skills to the model and executes a named skill manually.

Evidence:

- tool description and schema in [pkg/tool/builtin/skill.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/tool/builtin/skill.go#L14)
- tool execution path in [pkg/tool/builtin/skill.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/tool/builtin/skill.go#L121)

This gives the model a uniform way to load skills inside a conversation, but the surrounding runtime semantics are still relatively light.

## codex current skills flow

At a high level, `other/codex` uses a richer skill pipeline:

1. Build skill roots from config layers, repo roots, plugin roots, user roots, and system roots.
2. Load skills into a `SkillLoadOutcome` that includes errors, disabled paths, scope, metadata, dependencies, permissions, and indexes for implicit invocation.
3. Resolve explicit mentions from structured user input and text mentions such as `$skill-name` or linked skill paths.
4. Inject full skill instructions into the conversation stream.
5. Track telemetry and analytics for skill invocation.
6. Gate behavior with mode-aware user interaction tools such as `request_user_input`.

### Layered loading and scope

`SkillsManager` caches outcomes per cwd and builds roots from config layers, plugins, repo-local `.agents/skills`, user roots, and system skill cache.

Evidence:

- manager and per-cwd cache in [other/codex/codex-rs/core/src/skills/manager.rs](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/codex-rs/core/src/skills/manager.rs#L30)
- root selection and scope layering in [other/codex/codex-rs/core/src/skills/loader.rs](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/codex-rs/core/src/skills/loader.rs#L215)

This is notably richer than `agentkit`'s current root model.

### Rich metadata model

The codex loader parses not only basic frontmatter but also:

- interface metadata
- dependencies
- policy
- permission profile
- managed network overrides

Evidence:

- skill metadata structures in [other/codex/codex-rs/core/src/skills/loader.rs](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/codex-rs/core/src/skills/loader.rs#L40)
- resulting metadata model in [other/codex/codex-rs/core/src/skills/model.rs](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/codex-rs/core/src/skills/model.rs#L8)

That means a skill is not just prompt text. It is also a policy- and UI-aware asset.

### Mention resolution and injection

Codex resolves skill mentions from both structured inputs and textual `$skill-name` mentions, then injects the full skill instructions as structured response items.

Evidence:

- injection flow in [other/codex/codex-rs/core/src/skills/injection.rs](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/codex-rs/core/src/skills/injection.rs#L24)
- explicit mention resolution in [other/codex/codex-rs/core/src/skills/injection.rs](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/codex-rs/core/src/skills/injection.rs#L100)

This is more sophisticated than `agentkit`'s current `ForceSkills` plus `Skill` tool model.

### Interaction-aware tooling

Codex also integrates skill behavior with collaboration modes and user interaction tools. `request_user_input` is mode-aware and validates availability before execution.

Evidence:

- [other/codex/codex-rs/core/src/tools/handlers/request_user_input.rs](/home/vipas/workspace/saker-ai/godeps/agentkit/other/codex/codex-rs/core/src/tools/handlers/request_user_input.rs#L13)

This matters because many useful skills need a structured way to pause, ask, and resume safely.

## Structural differences

### What `agentkit` does well

- simpler model
- easier to embed
- easier to reason about end-to-end
- lower implementation overhead
- good testability around loader and registry internals

### What `agentkit` is missing relative to `codex`

- layered skill scope and config precedence
- explicit enabled/disabled state per skill path
- policy-aware implicit invocation controls
- dependencies declared as first-class metadata
- permission profile bound to the skill
- structured mention parsing and path-based resolution
- telemetry around skill injection and invocation
- a stronger interactive loop for multi-step skills

## Optimization space for `agentkit`

The answer is yes: there is meaningful optimization space, but it should be pursued selectively.

### P0: worthwhile now

#### A. Add a `SkillLoadOutcome` type instead of returning only registrations

Current loader returns `[]SkillRegistration` plus `[]error`.

Recommended new outcome:

- `Skills []SkillRegistration`
- `Errors []error`
- `DisabledPaths []string`
- `Origins/Scopes`
- `Indexes` for path/name lookup

Why:

- makes precedence and diagnostics explicit
- enables future policy features without changing every caller again

#### B. Add skill scope and origin metadata

Current metadata is relatively thin and mostly string-based.

Recommended additions:

- scope: repo, user, system, injected, embedded
- path/source normalization
- enabled state
- optional install origin

This is one of the biggest practical gaps with `codex`.

#### C. Add path-based explicit skill selection

Right now manual invocation is name-based. That is enough until duplicate names or layered overrides appear.

Recommended:

- preserve name-based invocation for compatibility
- add path or canonical identifier support internally
- resolve ambiguities before execution

This aligns well with future multi-root loading.

#### D. Separate skill activation from prompt mutation

Today a skill mainly returns output that gets prepended to the prompt.

Evidence:

- prompt prefix assembly in [pkg/api/agent.go](/home/vipas/workspace/saker-ai/godeps/agentkit/pkg/api/agent.go#L812)

Recommended direction:

- activation phase decides which skills apply
- injection phase materializes prompt additions or structured runtime directives
- execution phase handles side effects or metadata

This will make the system easier to evolve into richer behaviors without overloading `Result.Output`.

### P1: worthwhile once runtime boundaries are cleaner

#### E. Enrich skill metadata with dependencies and policy

Recommended metadata fields:

- `dependencies.tools`
- `policy.allow_implicit_invocation`
- optional `permission_profile`
- optional `network_overrides`

This does not require copying the full codex schema. A small compatible subset is enough.

#### F. Add skill telemetry

Recommended counters/events:

- skill loaded
- skill matched
- skill forced
- skill executed
- skill failed
- skill injected into prompt

This is cheap and useful for both debugging and product tuning.

#### G. Add a runtime-level `request_user_input` story

If `agentkit` wants skills to become a real workflow substrate rather than static prompt fragments, it needs a structured way to pause and ask the user short questions.

This can start small:

- runtime interface for user prompts
- CLI fallback implementation
- ACP-compatible implementation
- mode gating later

### P2: do only if the product direction requires it

#### H. Full mention parsing for `$skill-name` and linked skill paths

This is useful, but only once `agentkit` has:

- path identity
- layered scopes
- better ambiguity handling

Otherwise it adds complexity before the foundations are ready.

#### I. Plugin-integrated skill roots

`codex` loads skills from plugin roots. `agentkit` should do this only if plugin packaging becomes a real product need.

#### J. UI metadata for skill galleries

Codex supports richer interface metadata. `agentkit` does not need this unless it is building a richer desktop/TUI skill browser.

## Recommended roadmap for `agentkit` skills

### Phase 1

- introduce `SkillLoadOutcome`
- add scope/origin metadata
- refactor skill resolution to carry canonical identity
- keep public behavior unchanged

### Phase 2

- split skill activation from prompt injection
- add telemetry hooks
- add policy metadata with a minimal schema

### Phase 3

- add interactive skill support through a runtime question API
- decide whether explicit mention parsing is worth the added complexity

## Bottom line

`agentkit` absolutely has optimization space in the skill system, but the right target is not “make it as feature-rich as codex.” The right target is:

- keep `agentkit` small and embeddable
- make skill loading and identity model more explicit
- make skill activation more structured
- add enough policy and telemetry to support serious runtime use

The main architectural lesson from `codex` is not the specific UI. It is that skills become much more valuable once they are treated as first-class runtime assets with scope, policy, identity, and interaction semantics.
