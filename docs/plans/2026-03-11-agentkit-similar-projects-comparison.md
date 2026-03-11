# Go Agent Projects Comparison Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fetch the latest source snapshots of similar open-source Go agent projects into `other/` and produce a local, code-based comparison document against `agentkit`.

**Architecture:** Clone the selected repositories as shallow checkouts under `other/`, inspect their module metadata and source layout, extract evidence for core runtime capabilities, and summarize the findings in a Markdown document under `docs/`.

**Tech Stack:** Git, Go modules, ripgrep, shell inspection, Markdown

---

### Task 1: Fetch the comparison repositories

**Files:**
- Create: `other/langchaingo`
- Create: `other/eino`
- Create: `other/AgenticGoKit`
- Create: `other/agent-sdk-go`
- Create: `other/go-agent`
- Create: `other/go-agent-framework`

**Step 1: Verify target directory state**

Run: `ls -la other`
Expected: Existing contents are visible and there is no ambiguity about clone targets.

**Step 2: Clone the repositories**

Run: `git clone --depth 1 <repo-url> other/<name>`
Expected: Each repository is checked out at its latest default-branch commit.

### Task 2: Inspect code-level capabilities

**Files:**
- Read: `README.md`
- Read: `go.mod`
- Read: source files under each cloned repository

**Step 1: Capture module metadata and top-level structure**

Run: `find other/<name> -maxdepth 2 -type f \\( -name 'go.mod' -o -name 'README*' \\)`
Expected: Module and documentation entry points are identified.

**Step 2: Inspect code markers for runtime capabilities**

Run: `rg -n "mcp|sandbox|hook|subagent|workflow|memory|tool|trace|otel|json_schema|structured" other/<name>`
Expected: Evidence for implemented features is collected from source.

### Task 3: Write the local comparison document

**Files:**
- Create: `docs/2026-03-11-go-agent-projects-comparison.md`

**Step 1: Draft the comparison matrix**

Include: project positioning, module path, architecture style, tool/runtime model, MCP support, sandbox/security posture, multi-agent support, workflow/task model, observability, structured output, code maturity signals, and recommendation notes.

**Step 2: Cite code evidence**

Include: file paths for each key claim so the comparison stays anchored in source.

### Task 4: Verify outputs

**Files:**
- Verify: `other/*`
- Verify: `docs/2026-03-11-go-agent-projects-comparison.md`

**Step 1: Confirm cloned repositories exist**

Run: `find other -maxdepth 1 -mindepth 1 -type d | sort`
Expected: The expected repository directories are present.

**Step 2: Confirm the comparison document exists**

Run: `test -f docs/2026-03-11-go-agent-projects-comparison.md && echo ok`
Expected: `ok`
