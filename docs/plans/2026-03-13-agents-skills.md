# Support `~/.agents/skills` Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the runtime discover skills from `~/.agents/skills` by default when no explicit skills directories are configured.

**Architecture:** Keep the change inside filesystem skill discovery. `pkg/runtime/skills/loader.go` will extend the default root resolution logic so it returns both the config-root skills directory and `~/.agents/skills`, while preserving current explicit-directory override behavior. Tests will exercise the root resolution through `LoadFromFS`.

**Tech Stack:** Go, standard library `os`/`filepath`, existing runtime skills loader tests.

---

### Task 1: Add a failing loader test for the new default root

**Files:**
- Modify: `pkg/runtime/skills/loader_additional_test.go`
- Test: `pkg/runtime/skills/loader_additional_test.go`

**Step 1: Write the failing test**

Add a test that:
- creates a temp project root with `config/skills/delta/SKILL.md`
- creates a temp home directory with `.agents/skills/omega/SKILL.md`
- temporarily sets `HOME` to that temp home
- calls `LoadFromFS(LoaderOptions{ProjectRoot: root, ConfigRoot: "config"})`
- expects both `delta` and `omega`

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/runtime/skills -run TestLoadFromFSWithConfigRootIncludesAgentsSkills -count=1`
Expected: FAIL because only the config-root skill is discovered before implementation.

**Step 3: Write minimal implementation**

Update `resolveSkillRoots()` in `pkg/runtime/skills/loader.go` so that when `opts.Directories` is empty it adds:
- `<resolved config root>/skills`
- `~/.agents/skills` when `os.UserHomeDir()` succeeds

Keep explicit directories unchanged.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/runtime/skills -run TestLoadFromFSWithConfigRootIncludesAgentsSkills -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/runtime/skills/loader.go pkg/runtime/skills/loader_additional_test.go docs/plans/2026-03-13-agents-skills-design.md docs/plans/2026-03-13-agents-skills.md
git commit -m "feat: discover ~/.agents skills by default"
```

### Task 2: Verify override semantics stay intact

**Files:**
- Modify: `pkg/runtime/skills/loader_additional_test.go`
- Test: `pkg/runtime/skills/loader_additional_test.go`

**Step 1: Write the failing test**

Add a test that:
- creates a temp home directory with `.agents/skills/home-skill/SKILL.md`
- creates an explicit custom skills dir with `custom-skill/SKILL.md`
- sets `HOME` to the temp home
- calls `LoadFromFS(LoaderOptions{ProjectRoot: root, Directories: []string{customDir}})`
- expects only `custom-skill`

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/runtime/skills -run TestLoadFromFSExplicitDirectoriesDoNotIncludeAgentsSkills -count=1`
Expected: FAIL only if the implementation accidentally appends `~/.agents/skills` even with explicit directories.

**Step 3: Write minimal implementation**

No code change should be required if Task 1 preserves explicit-directory semantics.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/runtime/skills -run 'TestLoadFromFSWithConfigRootIncludesAgentsSkills|TestLoadFromFSExplicitDirectoriesDoNotIncludeAgentsSkills' -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/runtime/skills/loader_additional_test.go
git commit -m "test: cover ~/.agents skills discovery semantics"
```

### Task 3: Run targeted and package verification

**Files:**
- Test: `pkg/runtime/skills/loader_additional_test.go`

**Step 1: Run focused package tests**

Run: `go test ./pkg/runtime/skills -count=1`
Expected: PASS

**Step 2: Run CLI tests as regression coverage**

Run: `go test ./cmd/cli/... -count=1`
Expected: PASS

**Step 3: Record results**

Summarize that default skills discovery now includes:
- `<config-root>/skills`
- `~/.agents/skills`

and that explicit `--skills-dir` still overrides defaults.
