# Support `~/.agents/skills` Design

## Goal

Add `~/.agents/skills` as an additional default skills discovery root without changing existing explicit `--skills-dir` semantics.

## Decision

When the runtime resolves skills discovery roots:

- If `SkillsDirs` / `LoaderOptions.Directories` is non-empty, keep existing behavior and use only those explicit directories.
- If no explicit directories are provided, discover skills from:
  - `<config-root>/skills`
  - `~/.agents/skills`

## Rationale

This keeps the change small and predictable:

- Existing project-local defaults continue to work.
- Users gain a user-level shared skills directory.
- Explicit CLI overrides remain authoritative and do not unexpectedly pull in extra roots.

## Affected Areas

- `pkg/runtime/skills/loader.go`
- `pkg/runtime/skills/loader_additional_test.go`

## Testing

Add loader tests that verify:

- default discovery includes both config-root and `~/.agents/skills`
- explicit `Directories` still override defaults
