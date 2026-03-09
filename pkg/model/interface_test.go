package model

import "testing"

func TestMessage_TextContent(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{
			name: "fallback to Content when no blocks",
			msg:  Message{Content: "hello"},
			want: "hello",
		},
		{
			name: "single text block",
			msg: Message{
				Content:       "ignored",
				ContentBlocks: []ContentBlock{{Type: ContentBlockText, Text: "from block"}},
			},
			want: "from block",
		},
		{
			name: "multiple text blocks concatenated",
			msg: Message{
				ContentBlocks: []ContentBlock{
					{Type: ContentBlockText, Text: "a"},
					{Type: ContentBlockImage, Data: "img"},
					{Type: ContentBlockText, Text: "b"},
				},
			},
			want: "ab",
		},
		{
			name: "only non-text blocks falls back to Content",
			msg: Message{
				Content:       "fallback",
				ContentBlocks: []ContentBlock{{Type: ContentBlockImage, Data: "img"}},
			},
			want: "fallback",
		},
		{
			name: "empty blocks falls back to Content",
			msg:  Message{Content: "text", ContentBlocks: []ContentBlock{}},
			want: "text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.TextContent(); got != tt.want {
				t.Errorf("TextContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestStructuredOutputTypes(t *testing.T) {
	req := Request{
		ResponseFormat: &ResponseFormat{
			Type: "json_schema",
			JSONSchema: &OutputJSONSchema{
				Name:        "storyboard",
				Description: "Storyboard output",
				Schema: map[string]any{
					"type": "array",
				},
				Strict: true,
			},
		},
	}

	if req.ResponseFormat == nil {
		t.Fatalf("expected response format")
	}
	if req.ResponseFormat.Type != "json_schema" {
		t.Fatalf("unexpected response format type %q", req.ResponseFormat.Type)
	}
	if req.ResponseFormat.JSONSchema == nil {
		t.Fatalf("expected json schema")
	}
	if req.ResponseFormat.JSONSchema.Name != "storyboard" {
		t.Fatalf("unexpected schema name %q", req.ResponseFormat.JSONSchema.Name)
	}
	if !req.ResponseFormat.JSONSchema.Strict {
		t.Fatalf("expected strict schema")
	}
}
