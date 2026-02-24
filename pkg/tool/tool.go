package tool

import "context"

// Tool represents an executable capability exposed to the agent runtime.
type Tool interface {
	// Name returns the unique identifier of the tool.
	Name() string

	// Description gives a short human readable summary.
	Description() string

	// Schema describes the tool parameters. Nil means the tool does not expect input.
	Schema() *JSONSchema

	// Execute runs the tool with validated parameters.
	Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
}
