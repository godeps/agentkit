package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/pipeline"
	runtimecache "github.com/godeps/agentkit/pkg/runtime/cache"
	"github.com/godeps/agentkit/pkg/tool"
)

func main() {
	rt, err := api.New(context.Background(), api.Options{
		ProjectRoot: ".",
		Model:       staticModel{},
		CustomTools: []tool.Tool{timelineTool{}},
		CacheStore:  runtimecache.NewMemoryStore(),
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = rt.Close() }()

	resp, err := rt.Run(context.Background(), api.Request{
		Pipeline: &pipeline.Step{
			Name: "timeline-step",
			Tool: "timeline_tool",
			Input: []artifact.ArtifactRef{
				artifact.NewGeneratedRef("timeline-input", artifact.ArtifactKindImage),
			},
			With: map[string]any{"prompt": "inspect"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range resp.Timeline {
		fmt.Printf("%s %s\n", entry.Kind, entry.Name)
	}
}

type timelineTool struct{}

func (timelineTool) Name() string             { return "timeline_tool" }
func (timelineTool) Description() string      { return "returns one artifact for timeline inspection" }
func (timelineTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (timelineTool) Execute(context.Context, map[string]interface{}) (*tool.ToolResult, error) {
	return &tool.ToolResult{
		Output: "timeline complete",
		Artifacts: []artifact.ArtifactRef{
			artifact.NewGeneratedRef("timeline-output", artifact.ArtifactKindText),
		},
	}, nil
}

type staticModel struct{}

func (staticModel) Complete(context.Context, model.Request) (*model.Response, error) {
	return &model.Response{Message: model.Message{Role: "assistant", Content: "unused"}}, nil
}

func (staticModel) CompleteStream(context.Context, model.Request, model.StreamHandler) error {
	return nil
}
