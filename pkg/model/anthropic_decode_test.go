package model

import (
	"encoding/json"
	"testing"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
)

// TestDecodeEmptyJSON 测试空 JSON 的处理
func TestDecodeEmptyJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected map[string]any
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty input",
			input:    json.RawMessage{},
			expected: nil,
		},
		{
			name:     "empty object",
			input:    json.RawMessage(`{}`),
			expected: map[string]any{},
		},
		{
			name:     "object with command",
			input:    json.RawMessage(`{"command":"ls"}`),
			expected: map[string]any{"command": "ls"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeJSON(tt.input)
			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
			}
			if tt.expected != nil && result == nil {
				t.Errorf("expected %v, got nil", tt.expected)
			}
			if tt.expected != nil && result != nil {
				if len(tt.expected) != len(result) {
					t.Errorf("length mismatch: expected %d, got %d", len(tt.expected), len(result))
				}
			}
		})
	}
}

// TestToolCallFromBlockWithEmptyInput 测试空输入的工具调用
func TestToolCallFromBlockWithEmptyInput(t *testing.T) {
	tests := []struct {
		name          string
		block         anthropicsdk.ContentBlockUnion
		expectNil     bool
		expectArgs    map[string]any
		expectArgsLen int
	}{
		{
			name: "tool_use with empty input",
			block: anthropicsdk.ContentBlockUnion{
				Type:  "tool_use",
				ID:    "call-1",
				Name:  "bash_execute",
				Input: json.RawMessage(`{}`),
			},
			expectNil:     false,
			expectArgs:    map[string]any{},
			expectArgsLen: 0,
		},
		{
			name: "tool_use with null input",
			block: anthropicsdk.ContentBlockUnion{
				Type:  "tool_use",
				ID:    "call-2",
				Name:  "bash_execute",
				Input: json.RawMessage(`null`),
			},
			expectNil:     false,
			expectArgs:    nil,
			expectArgsLen: 0,
		},
		{
			name: "tool_use with command parameter",
			block: anthropicsdk.ContentBlockUnion{
				Type:  "tool_use",
				ID:    "call-3",
				Name:  "bash_execute",
				Input: json.RawMessage(`{"command":"ls -la"}`),
			},
			expectNil:     false,
			expectArgsLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := toolCallFromBlock(tt.block)
			if tt.expectNil && call != nil {
				t.Errorf("expected nil, got %+v", call)
				return
			}
			if !tt.expectNil && call == nil {
				t.Error("expected non-nil tool call")
				return
			}
			if call != nil {
				if len(call.Arguments) != tt.expectArgsLen {
					t.Errorf("expected %d arguments, got %d: %+v", tt.expectArgsLen, len(call.Arguments), call.Arguments)
				}
			}
		})
	}
}
