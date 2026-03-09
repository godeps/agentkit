# Agentkit Safety Limits Design

**Date:** 2026-03-09

**Scope:** Fix silent truncation and hidden-output behavior in core tools, enable safe custom tool overrides, and make compaction limits configurable.

## Goals

- Remove silent per-line data loss from `file_read`.
- Let caller-supplied `CustomTools` replace same-name built-ins intentionally.
- Wire `settings.ToolOutput` into runtime output persistence.
- Make persisted large outputs more visible than a bare filesystem path.
- Make AutoCompact fallback context limit and summary token budget configurable.

## Non-Goals

- Do not redesign every tool constructor in this pass.
- Do not remove the existing default line-count limit from `file_read`.
- Do not auto-inject full persisted output back into model context in this pass.

## Current Problems

### `file_read`

- `pkg/tool/builtin/read.go` truncates each line at 2000 characters.
- Truncation is byte-based, so UTF-8 characters can be split.
- The tool reports `truncated`, but the returned content is still incomplete in a way that is easy for the model to miss.

### `CustomTools`

- Built-ins are appended first, `CustomTools` second.
- Duplicate canonical names are skipped during registration.
- This prevents using `CustomTools` as a targeted fix for built-in behavior.

### Large output persistence

- `pkg/tool/persister.go` supports configurable thresholds.
- `pkg/config/settings_types.go` already defines `ToolOutputConfig`.
- `pkg/api/agent.go` still constructs the executor with a default persister and does not apply `settings.ToolOutput`.
- Persisted output currently becomes `[Output saved to: /path]`, which is too weak a signal for model follow-up.

### AutoCompact

- `pkg/api/compact.go` uses a hardcoded fallback context limit of `200000`.
- Summary generation uses a hardcoded `1024` token budget.
- Both values should be adjustable from `CompactConfig`.

## Chosen Approach

### 1. `file_read`: total output budget instead of per-line truncation

- Remove the per-line truncation path from `ReadTool`.
- Add a formatted-output byte budget enforced while building the response body.
- Preserve line numbering and `offset`/`limit` behavior.
- Append an explicit tail notice when output is cut off.
- Add structured metadata for output truncation so callers can reason about the cutoff.

This fixes the highest-risk issue without changing the basic tool contract.

### 2. `CustomTools`: custom wins on name collision

- Keep existing tool discovery flow.
- Change duplicate handling so caller-supplied custom tools replace previously collected built-ins with the same canonical name.
- Preserve `Options.Tools` as the highest-priority explicit override path.

This keeps the API simple and makes the intended extension point actually usable.

### 3. `ToolOutput`: wire settings and improve visible result text

- Build the executor persister from `settings.ToolOutput` when provided.
- Keep the persisted file behavior.
- Replace the output placeholder with a compact summary that includes:
  - leading snippet
  - byte size
  - persisted path

This avoids the heavier architecture change of implicit context reinjection while still making the output visible enough for follow-up tool use.

### 4. `AutoCompact`: expose the two hardcoded limits

- Add `ContextLimit` and `SummaryMaxTokens` fields to `CompactConfig`.
- Keep current defaults when fields are unset.
- Rename the internal fallback constant to neutral wording.

This is a low-risk API extension with clear value.

## Compatibility Notes

- `file_read` output will change for very long single-line files; this is intentional.
- `CustomTools` collision behavior will change from silent built-in precedence to custom precedence.
- Existing settings files remain valid because new compact fields will be optional.
- Persisted tool output remains on disk; only the visible inline message changes.

## Testing Strategy

- Add failing tests before implementation for each behavior change.
- Verify red/green for:
  - long single-line `file_read`
  - custom tool replacing built-in
  - `settings.ToolOutput` thresholds flowing into executor
  - compact config using explicit `ContextLimit` and `SummaryMaxTokens`
- Run targeted package tests first, then a broader regression pass on touched packages.

## Rollout Order

1. `file_read`
2. custom-tool override behavior
3. tool output settings + visible persisted output summary
4. compact configuration

This order fixes the highest user-facing correctness issue first and keeps later changes easier to validate.
