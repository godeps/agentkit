package toolbuiltin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/godeps/agentkit/pkg/tool"
)

const todoWriteDescription = `Updates the todo list.

Usage:
- Provide the full list of todos in the desired order.
- Each todo must include content and status.
- status should be one of: pending, in_progress, completed.
`

var todoWriteSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"todos": map[string]interface{}{
			"type":        "array",
			"description": "Full todo list to set",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Todo text",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "pending|in_progress|completed",
					},
					"activeForm": map[string]interface{}{
						"type":        "string",
						"description": "Alternate phrasing used when actively working",
					},
				},
				"required": []string{"content", "status"},
			},
		},
	},
	Required: []string{"todos"},
}

type TodoWriteItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// TodoWriteTool stores a session-local todo list in memory.
// It mirrors the Claude Code style TodoWrite tool surface.
type TodoWriteTool struct {
	mu    sync.Mutex
	items []TodoWriteItem
}

func NewTodoWriteTool() *TodoWriteTool {
	return &TodoWriteTool{}
}

func (t *TodoWriteTool) Name() string { return "TodoWrite" }

func (t *TodoWriteTool) Description() string { return todoWriteDescription }

func (t *TodoWriteTool) Schema() *tool.JSONSchema { return todoWriteSchema }

func (t *TodoWriteTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if t == nil {
		return nil, errors.New("todo write tool is not initialised")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	list, err := parseTodoWriteItems(params)
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	t.items = list
	t.mu.Unlock()

	return &tool.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("updated %d todos", len(list)),
		Data: map[string]interface{}{
			"count": len(list),
		},
	}, nil
}

func (t *TodoWriteTool) Snapshot() []TodoWriteItem {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]TodoWriteItem, len(t.items))
	copy(out, t.items)
	return out
}

func parseTodoWriteItems(params map[string]interface{}) ([]TodoWriteItem, error) {
	if params == nil {
		return nil, errors.New("params is nil")
	}
	raw, ok := params["todos"]
	if !ok {
		return nil, errors.New("todos is required")
	}

	var entries []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		entries = make([]map[string]interface{}, 0, len(v))
		for i, entry := range v {
			m, ok := entry.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("todos[%d] must be object, got %T", i, entry)
			}
			entries = append(entries, m)
		}
	case []map[string]interface{}:
		entries = v
	default:
		return nil, fmt.Errorf("todos must be array, got %T", raw)
	}

	out := make([]TodoWriteItem, 0, len(entries))
	for i, m := range entries {
		content, err := coerceString(m["content"])
		if err != nil {
			return nil, fmt.Errorf("todos[%d].content must be string: %w", i, err)
		}
		status, err := coerceString(m["status"])
		if err != nil {
			return nil, fmt.Errorf("todos[%d].status must be string: %w", i, err)
		}
		active := content
		if rawActive, ok := m["activeForm"]; ok {
			if s, err := coerceString(rawActive); err == nil && s != "" {
				active = s
			}
		}

		out = append(out, TodoWriteItem{
			Content:    content,
			Status:     status,
			ActiveForm: active,
		})
	}
	return out, nil
}
