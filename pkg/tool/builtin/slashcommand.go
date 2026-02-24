package toolbuiltin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/godeps/agentkit/pkg/runtime/commands"
	"github.com/godeps/agentkit/pkg/tool"
)

const slashCommandDescriptionHeader = `Execute a slash command within the main conversation

**IMPORTANT - Intent Matching:**
Before starting any task, CHECK if the user's request matches one of the slash commands listed below. This tool exists to route user intentions to specialized workflows.

How slash commands work:
When you use this tool or when a user types a slash command, you will see <command-message>{name} is running…</command-message> followed by the expanded prompt. For example, if .claude/commands/foo.md contains "Print today's date", then /foo expands to that prompt in the next message.

Usage:
- command (required): The slash command to execute, including any arguments
- Example: command: "/review-pr 123"

IMPORTANT: Only use this tool for custom slash commands that appear in the Available Commands list below. Do NOT use for:
- Built-in CLI commands (like /help, /clear, etc.)
- Commands not shown in the list
- Commands you think might exist but aren't listed

Notes:
- When a user requests multiple slash commands, execute each one sequentially and check for <command-message>{name} is running…</command-message> to verify each has been processed
- Do not invoke a command that is already running. For example, if you see <command-message>foo is running…</command-message>, do NOT use this tool with "/foo" - process the expanded prompt in the following message
- Only custom slash commands with descriptions are listed in Available Commands. If a user's command is not listed, ask them to check the slash command file and consult the docs.
`

var slashCommandSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"command": map[string]interface{}{
			"type":        "string",
			"description": "The slash command to execute with its arguments, e.g., \"/review-pr 123\"",
		},
	},
	Required: []string{"command"},
}

// SlashCommandTool routes slash command invocations to the command executor.
type SlashCommandTool struct {
	executor *commands.Executor
	parser   func(string) ([]commands.Invocation, error)
}

// NewSlashCommandTool builds a tool backed by the provided executor.
func NewSlashCommandTool(exec *commands.Executor) *SlashCommandTool {
	return &SlashCommandTool{
		executor: exec,
		parser:   commands.Parse,
	}
}

func (s *SlashCommandTool) Name() string { return "SlashCommand" }

func (s *SlashCommandTool) Description() string {
	var defs []commands.Definition
	if s != nil && s.executor != nil {
		defs = s.executor.List()
	}
	return buildSlashCommandDescription(defs)
}

func (s *SlashCommandTool) Schema() *tool.JSONSchema { return slashCommandSchema }

func buildSlashCommandDescription(defs []commands.Definition) string {
	var b strings.Builder
	b.WriteString(slashCommandDescriptionHeader)
	b.WriteString("\nAvailable Commands:\n")
	if len(defs) == 0 {
		b.WriteString("- (no commands registered)\n")
		return b.String()
	}
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			name = "unnamed"
		}
		description := strings.TrimSpace(def.Description)
		if description == "" {
			description = "No description provided."
		}
		fmt.Fprintf(&b, "- /%s: %s\n", name, description)
	}
	return b.String()
}

func (s *SlashCommandTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if s == nil || s.executor == nil || s.parser == nil {
		return nil, errors.New("slash command executor is not initialised")
	}
	commandText, err := parseSlashCommand(params)
	if err != nil {
		return nil, err
	}
	invocations, err := s.parser(commandText)
	if err != nil {
		if errors.Is(err, commands.ErrNoCommand) {
			return nil, fmt.Errorf("no slash command found in %q", commandText)
		}
		return nil, err
	}
	results, execErr := s.executor.Execute(ctx, invocations)
	output := formatCommandOutput(results)
	data := map[string]interface{}{
		"results": results,
	}
	return &tool.ToolResult{
		Success: execErr == nil,
		Output:  output,
		Data:    data,
	}, execErr
}

func parseSlashCommand(params map[string]interface{}) (string, error) {
	if params == nil {
		return "", errors.New("params is nil")
	}
	raw, ok := params["command"]
	if !ok {
		return "", errors.New("command is required")
	}
	value, err := coerceString(raw)
	if err != nil {
		return "", fmt.Errorf("command must be string: %w", err)
	}
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "/") {
		return "", errors.New("slash commands must start with '/'")
	}
	return value, nil
}

func formatCommandOutput(results []commands.Result) string {
	if len(results) == 0 {
		return "command executed with no output"
	}
	var b strings.Builder
	for _, res := range results {
		name := res.Command
		if name == "" {
			name = "command"
		}
		fmt.Fprintf(&b, "%s:\n", name)
		if res.Error != "" {
			fmt.Fprintf(&b, "  error: %s\n", res.Error)
			continue
		}
		switch out := res.Output.(type) {
		case string:
			if strings.TrimSpace(out) == "" {
				fmt.Fprintf(&b, "  (no output)\n")
			} else {
				fmt.Fprintf(&b, "  %s\n", out)
			}
		case fmt.Stringer:
			text := strings.TrimSpace(out.String())
			if text == "" {
				fmt.Fprintf(&b, "  (no output)\n")
			} else {
				fmt.Fprintf(&b, "  %s\n", text)
			}
		default:
			fmt.Fprintf(&b, "  %+v\n", out)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
