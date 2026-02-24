package prompts

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/godeps/agentkit/pkg/runtime/commands"
	"github.com/godeps/agentkit/pkg/runtime/skills"
	"github.com/godeps/agentkit/pkg/runtime/subagents"
)

func TestParse_EmptyFS(t *testing.T) {
	fsys := fstest.MapFS{}
	builtins := Parse(fsys)

	if len(builtins.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(builtins.Skills))
	}
	if len(builtins.Commands) != 0 {
		t.Errorf("expected 0 commands, got %d", len(builtins.Commands))
	}
	if len(builtins.Subagents) != 0 {
		t.Errorf("expected 0 subagents, got %d", len(builtins.Subagents))
	}
	if len(builtins.Hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(builtins.Hooks))
	}
	if len(builtins.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_Skills(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/test-skill/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: test-skill
description: A test skill
allowed-tools: bash, grep
---
This is the skill body.
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(builtins.Skills))
	}

	skill := builtins.Skills[0]
	if skill.Definition.Name != "test-skill" {
		t.Errorf("expected skill name 'test-skill', got %q", skill.Definition.Name)
	}
	if skill.Definition.Description != "A test skill" {
		t.Errorf("expected description 'A test skill', got %q", skill.Definition.Description)
	}
	if skill.Definition.Metadata["allowed-tools"] != "bash,grep" {
		t.Errorf("expected allowed-tools 'bash,grep', got %q", skill.Definition.Metadata["allowed-tools"])
	}
}

func TestParse_Commands(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/test-cmd.md": &fstest.MapFile{
			Data: []byte(`---
name: test-cmd
description: A test command
allowed-tools: bash
---
Run this command with $ARGUMENTS
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(builtins.Commands))
	}

	cmd := builtins.Commands[0]
	if cmd.Definition.Name != "test-cmd" {
		t.Errorf("expected command name 'test-cmd', got %q", cmd.Definition.Name)
	}
	if cmd.Definition.Description != "A test command" {
		t.Errorf("expected description 'A test command', got %q", cmd.Definition.Description)
	}
}

func TestParse_Subagents(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/test-agent.md": &fstest.MapFile{
			Data: []byte(`---
name: test-agent
description: A test subagent
tools: bash, grep
model: sonnet
---
This is the agent prompt.
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(builtins.Subagents))
	}

	agent := builtins.Subagents[0]
	if agent.Definition.Name != "test-agent" {
		t.Errorf("expected subagent name 'test-agent', got %q", agent.Definition.Name)
	}
	if agent.Definition.Description != "A test subagent" {
		t.Errorf("expected description 'A test subagent', got %q", agent.Definition.Description)
	}
	if agent.Definition.DefaultModel != "sonnet" {
		t.Errorf("expected model 'sonnet', got %q", agent.Definition.DefaultModel)
	}
}

func TestParse_Hooks(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{
	"PreToolUse": [
		{"command": "echo pre-tool", "name": "test-hook"}
	],
	"PostToolUse": [
		{"command": "echo post-tool", "timeout": "5s"}
	]
}`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(builtins.Hooks))
	}
}

func TestParse_HooksFromShellScripts(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/pre-tool-use.sh": &fstest.MapFile{
			Data: []byte(`#!/bin/bash
echo "pre-tool-use"
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(builtins.Hooks))
	}
	if builtins.Hooks[0].Name != "pre-tool-use" {
		t.Errorf("expected hook name 'pre-tool-use', got %q", builtins.Hooks[0].Name)
	}
}

func TestParseWithOptions_CustomDirs(t *testing.T) {
	fsys := fstest.MapFS{
		"custom/skills/my-skill/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: my-skill
description: Custom skill
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{
		SkillsDir: "custom/skills",
	})

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(builtins.Skills))
	}
	if builtins.Skills[0].Definition.Name != "my-skill" {
		t.Errorf("expected skill name 'my-skill', got %q", builtins.Skills[0].Definition.Name)
	}
}

func TestParse_ValidationErrors(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/invalid/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: INVALID_NAME
description: Invalid skill
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
	if len(builtins.Skills) != 0 {
		t.Errorf("expected 0 skills due to validation error, got %d", len(builtins.Skills))
	}
}

func TestParse_DuplicateSkills(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/duplicate/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: duplicate
description: First skill
---
Body A
`),
		},
		".claude/skills/duplicate2/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: duplicate
description: Second skill
---
Body B
`),
		},
	}

	builtins := Parse(fsys)

	// duplicate2 has name mismatch with directory, so we get that error
	// plus the duplicate name error
	if len(builtins.Errors) < 1 {
		t.Fatalf("expected at least 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_CommandWithoutFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/simple.md": &fstest.MapFile{
			Data: []byte(`Just a simple command body without frontmatter.`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(builtins.Commands))
	}
	if builtins.Commands[0].Definition.Name != "simple" {
		t.Errorf("expected command name 'simple' from filename, got %q", builtins.Commands[0].Definition.Name)
	}
}

func TestParse_SkillMissingFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/bad/SKILL.md": &fstest.MapFile{
			Data: []byte(`No frontmatter here.`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error for missing frontmatter, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
	if len(builtins.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(builtins.Skills))
	}
}

func TestParse_SkillWithAllMetadata(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/full-skill/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: full-skill
description: A fully configured skill
license: MIT
compatibility: claude-3
metadata:
  author: test
  version: "1.0"
allowed-tools:
  - bash
  - grep
  - glob
---
Full skill body content.
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(builtins.Skills))
	}

	skill := builtins.Skills[0]
	if skill.Definition.Metadata["license"] != "MIT" {
		t.Errorf("expected license 'MIT', got %q", skill.Definition.Metadata["license"])
	}
	if skill.Definition.Metadata["compatibility"] != "claude-3" {
		t.Errorf("expected compatibility 'claude-3', got %q", skill.Definition.Metadata["compatibility"])
	}
	if skill.Definition.Metadata["author"] != "test" {
		t.Errorf("expected author 'test', got %q", skill.Definition.Metadata["author"])
	}
}

func TestParse_CommandWithAllMetadata(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/full-cmd.md": &fstest.MapFile{
			Data: []byte(`---
name: full-cmd
description: A fully configured command
allowed-tools: bash, grep
argument-hint: <file-path>
model: sonnet
disable-model-invocation: true
---
Process file: $ARGUMENTS
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(builtins.Commands))
	}
}

func TestParse_SubagentWithAllMetadata(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/full-agent.md": &fstest.MapFile{
			Data: []byte(`---
name: full-agent
description: A fully configured subagent
tools: bash, grep, glob
model: opus
permissionMode: bypassPermissions
skills: code-review, testing
---
Full agent prompt content.
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(builtins.Subagents))
	}

	agent := builtins.Subagents[0]
	if agent.Definition.DefaultModel != "opus" {
		t.Errorf("expected model 'opus', got %q", agent.Definition.DefaultModel)
	}
}

func TestParse_SubagentInvalidModel(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/bad-model.md": &fstest.MapFile{
			Data: []byte(`---
name: bad-model
description: Agent with invalid model
model: invalid-model
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error for invalid model, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_HooksWithMatcher(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{
	"PreToolUse": [
		{"command": "echo matched", "matcher": "bash.*", "name": "bash-matcher"}
	]
}`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(builtins.Hooks))
	}
}

func TestParse_HooksInvalidJSON(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{invalid json`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error for invalid JSON, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_HooksMissingCommand(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{
	"PreToolUse": [
		{"name": "no-command"}
	]
}`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error for missing command, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_HooksInvalidTimeout(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{
	"PreToolUse": [
		{"command": "echo test", "timeout": "invalid"}
	]
}`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error for invalid timeout, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_HooksInvalidMatcher(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{
	"PreToolUse": [
		{"command": "echo test", "matcher": "[invalid"}
	]
}`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error for invalid matcher, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SkillHandlerExecution(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/exec-skill/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: exec-skill
description: Test handler execution
allowed-tools: bash
---
Handler body content.
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(builtins.Skills))
	}

	// Execute the handler
	result, err := builtins.Skills[0].Handler.Execute(context.Background(), skills.ActivationContext{})
	if err != nil {
		t.Fatalf("handler execution failed: %v", err)
	}

	output, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("expected output to be map[string]any, got %T", result.Output)
	}
	body, ok := output["body"].(string)
	if !ok || body != "Handler body content.\n" {
		t.Errorf("expected body 'Handler body content.\\n', got %q", output["body"])
	}
}

func TestParse_CommandHandlerExecution(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/exec-cmd.md": &fstest.MapFile{
			Data: []byte(`---
name: exec-cmd
description: Test handler execution
---
Process: $ARGUMENTS
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(builtins.Commands))
	}

	// Execute the handler with arguments
	result, err := builtins.Commands[0].Handler.Handle(context.Background(), commands.Invocation{
		Name: "exec-cmd",
		Args: []string{"file.txt", "arg2"},
	})
	if err != nil {
		t.Fatalf("handler execution failed: %v", err)
	}

	if result.Output != "Process: file.txt arg2\n" {
		t.Errorf("expected 'Process: file.txt arg2\\n', got %q", result.Output)
	}
}

func TestParse_SubagentHandlerExecution(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/exec-agent.md": &fstest.MapFile{
			Data: []byte(`---
name: exec-agent
description: Test handler execution
---
Agent prompt body.
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(builtins.Subagents))
	}

	// Execute the handler
	result, err := builtins.Subagents[0].Handler.Handle(context.Background(), subagents.Context{}, subagents.Request{})
	if err != nil {
		t.Fatalf("handler execution failed: %v", err)
	}

	if result.Output != "Agent prompt body.\n" {
		t.Errorf("expected 'Agent prompt body.\\n', got %q", result.Output)
	}
}

func TestParse_ToolListYAMLSequence(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/yaml-list/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: yaml-list
description: Test YAML sequence parsing
allowed-tools:
  - bash
  - bash
  - grep
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(builtins.Skills))
	}

	// Should deduplicate
	tools := builtins.Skills[0].Definition.Metadata["allowed-tools"]
	if tools != "bash,grep" {
		t.Errorf("expected deduplicated 'bash,grep', got %q", tools)
	}
}

func TestParse_SubagentInheritModel(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/inherit-agent.md": &fstest.MapFile{
			Data: []byte(`---
name: inherit-agent
description: Agent with inherit model
model: inherit
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(builtins.Subagents))
	}

	// inherit should normalize to empty string
	if builtins.Subagents[0].Definition.DefaultModel != "" {
		t.Errorf("expected empty model for inherit, got %q", builtins.Subagents[0].Definition.DefaultModel)
	}
}

func TestParse_SkillsNotDirectory(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills": &fstest.MapFile{
			Data: []byte(`not a directory`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_CommandsNotDirectory(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands": &fstest.MapFile{
			Data: []byte(`not a directory`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentsNotDirectory(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents": &fstest.MapFile{
			Data: []byte(`not a directory`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_HooksNotDirectory(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks": &fstest.MapFile{
			Data: []byte(`not a directory`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentMissingFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/bad.md": &fstest.MapFile{
			Data: []byte(`No frontmatter here.`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentInvalidPermissionMode(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/bad-perm.md": &fstest.MapFile{
			Data: []byte(`---
name: bad-perm
description: Agent with invalid permission mode
permissionMode: invalid
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SkillUnclosedFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/unclosed/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: unclosed
description: Missing closing separator
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_CommandUnclosedFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/unclosed.md": &fstest.MapFile{
			Data: []byte(`---
name: unclosed
description: Missing closing separator
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentUnclosedFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/unclosed.md": &fstest.MapFile{
			Data: []byte(`---
name: unclosed
description: Missing closing separator
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SkillInvalidYAML(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/bad-yaml/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: [invalid yaml
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_ToolListInvalidType(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/bad-tools/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: bad-tools
description: Invalid tool list type
allowed-tools:
  key: value
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_ToolListSequenceWithInvalidEntry(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/bad-entry/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: bad-entry
description: Tool list with invalid entry
allowed-tools:
  - bash
  - [nested, array]
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_HooksAllEventTypes(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{
	"PreToolUse": [{"command": "echo 1"}],
	"PostToolUse": [{"command": "echo 2"}],
	"PostToolUseFailure": [{"command": "echo 3"}],
	"PreCompact": [{"command": "echo 4"}],
	"ContextCompacted": [{"command": "echo 5"}],
	"UserPromptSubmit": [{"command": "echo 6"}],
	"SessionStart": [{"command": "echo 7"}],
	"SessionEnd": [{"command": "echo 8"}],
	"Stop": [{"command": "echo 9"}],
	"SubagentStart": [{"command": "echo 10"}],
	"SubagentStop": [{"command": "echo 11"}],
	"Notification": [{"command": "echo 12"}]
}`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Hooks) != 12 {
		t.Errorf("expected 12 hooks, got %d", len(builtins.Hooks))
	}
}

func TestParse_HooksWithEnv(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/hooks.json": &fstest.MapFile{
			Data: []byte(`{
	"PreToolUse": [
		{"command": "echo $MY_VAR", "env": {"MY_VAR": "test-value"}}
	]
}`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(builtins.Hooks))
	}
	if builtins.Hooks[0].Env["MY_VAR"] != "test-value" {
		t.Errorf("expected env MY_VAR='test-value', got %q", builtins.Hooks[0].Env["MY_VAR"])
	}
}

func TestParse_HooksUnknownEventFromFilename(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/hooks/unknown-event.sh": &fstest.MapFile{
			Data: []byte(`#!/bin/bash
echo "unknown"
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error for unknown event, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SkillValidationDescriptionTooLong(t *testing.T) {
	longDesc := make([]byte, 1025)
	for i := range longDesc {
		longDesc[i] = 'a'
	}

	fsys := fstest.MapFS{
		".claude/skills/long-desc/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: long-desc
description: ` + string(longDesc) + `
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SkillValidationCompatibilityTooLong(t *testing.T) {
	longCompat := make([]byte, 501)
	for i := range longCompat {
		longCompat[i] = 'a'
	}

	fsys := fstest.MapFS{
		".claude/skills/long-compat/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: long-compat
description: Test skill
compatibility: ` + string(longCompat) + `
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SkillValidationMissingDescription(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/no-desc/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: no-desc
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentValidationMissingDescription(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/no-desc.md": &fstest.MapFile{
			Data: []byte(`---
name: no-desc
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentValidationInvalidName(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/INVALID.md": &fstest.MapFile{
			Data: []byte(`---
name: INVALID_NAME
description: Test
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_CommandValidationInvalidName(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/INVALID NAME.md": &fstest.MapFile{
			Data: []byte(`---
name: INVALID NAME
description: Test
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_CommandNoArguments(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/no-args.md": &fstest.MapFile{
			Data: []byte(`---
name: no-args
description: Command without arguments
---
Static content without $ARGUMENTS
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(builtins.Commands))
	}

	// Execute with empty args
	result, err := builtins.Commands[0].Handler.Handle(context.Background(), commands.Invocation{
		Name: "no-args",
		Args: []string{},
	})
	if err != nil {
		t.Fatalf("handler execution failed: %v", err)
	}

	if result.Output != "Static content without $ARGUMENTS\n" {
		t.Errorf("unexpected output: %q", result.Output)
	}
}

func TestParse_ToolListNullValue(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/null-tools/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: null-tools
description: Skill with null tools
allowed-tools: ~
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(builtins.Skills))
	}
}

func TestParse_ToolListEmptyString(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/skills/empty-tools/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: empty-tools
description: Skill with empty tools string
allowed-tools: ""
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
}

func TestParse_SkillValidationMissingName(t *testing.T) {
	// When name is missing, it should fail validation
	// But skill requires name to match directory, so we test with empty name
	fsys := fstest.MapFS{
		".claude/skills/test/SKILL.md": &fstest.MapFile{
			Data: []byte(`---
name: ""
description: Skill with empty name
---
Body
`),
		},
	}

	builtins := ParseWithOptions(fsys, ParseOptions{Validate: true})

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_DuplicateCommands(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/dup.md": &fstest.MapFile{
			Data: []byte(`---
name: dup
description: First
---
Body 1
`),
		},
		".claude/commands/sub/dup.md": &fstest.MapFile{
			Data: []byte(`---
name: dup
description: Second
---
Body 2
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_DuplicateSubagents(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/dup.md": &fstest.MapFile{
			Data: []byte(`---
name: dup
description: First
---
Body 1
`),
		},
		".claude/agents/sub/dup.md": &fstest.MapFile{
			Data: []byte(`---
name: dup
description: Second
---
Body 2
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 duplicate error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentEmptyModel(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/no-model.md": &fstest.MapFile{
			Data: []byte(`---
name: no-model
description: Agent without model
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", builtins.Errors)
	}
	if len(builtins.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(builtins.Subagents))
	}
	if builtins.Subagents[0].Definition.DefaultModel != "" {
		t.Errorf("expected empty model, got %q", builtins.Subagents[0].Definition.DefaultModel)
	}
}

func TestParse_CommandInvalidYAML(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/commands/bad-yaml.md": &fstest.MapFile{
			Data: []byte(`---
name: [invalid
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}

func TestParse_SubagentInvalidYAML(t *testing.T) {
	fsys := fstest.MapFS{
		".claude/agents/bad-yaml.md": &fstest.MapFile{
			Data: []byte(`---
name: [invalid
---
Body
`),
		},
	}

	builtins := Parse(fsys)

	if len(builtins.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(builtins.Errors), builtins.Errors)
	}
}
