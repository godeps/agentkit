package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/tool"
)

// EchoTool is a simple custom tool used for demonstration.
type EchoTool struct{}

func (t *EchoTool) Name() string        { return "echo" }
func (t *EchoTool) Description() string { return "return the provided text" }
func (t *EchoTool) Schema() *tool.JSONSchema {
	return &tool.JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"text": map[string]any{"type": "string", "description": "text to return"},
		},
		Required: []string{"text"},
	}
}
func (t *EchoTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	return &tool.ToolResult{Output: fmt.Sprint(params["text"])}, nil
}

func main() {
	ctx := context.Background()

	provider := &model.AnthropicProvider{ModelName: "claude-sonnet-4-5-20250929"}

	rt, err := api.New(ctx, api.Options{
		ProjectRoot:         ".",
		ModelFactory:        provider,
		EnabledBuiltinTools: []string{"bash", "file_read"}, // nil=all, []string{}=none
		CustomTools:         []tool.Tool{&EchoTool{}},      // appended when Tools is empty
	})
	if err != nil {
		log.Fatalf("build runtime: %v", err)
	}
	defer rt.Close()

	resp, err := rt.Run(ctx, api.Request{
		Prompt:    "Use the echo tool to repeat 'hello from custom tool'",
		SessionID: "custom-tools-demo",
	})
	if err != nil {
		log.Fatalf("run: %v", err)
	}

	if resp.Result != nil {
		fmt.Println(resp.Result.Output)
	}
}
