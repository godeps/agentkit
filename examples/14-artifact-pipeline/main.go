package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/artifact"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/pipeline"
	"github.com/godeps/agentkit/pkg/tool"
)

func main() {
	rt, err := api.New(context.Background(), api.Options{
		ProjectRoot: ".",
		Model:       staticModel{},
		CustomTools: []tool.Tool{artifactTool{}},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = rt.Close() }()

	resp, err := rt.Run(context.Background(), api.Request{
		Pipeline: &pipeline.Step{
			Name: "generate-report",
			Tool: "artifact_tool",
			Input: []artifact.ArtifactRef{
				artifact.NewGeneratedRef("input-image", artifact.ArtifactKindImage),
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("output=%q artifacts=%d structured=%v\n", resp.Result.Output, len(resp.Result.Artifacts), resp.Result.Structured)
}

type artifactTool struct{}

func (artifactTool) Name() string             { return "artifact_tool" }
func (artifactTool) Description() string      { return "returns one generated artifact" }
func (artifactTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (artifactTool) Execute(context.Context, map[string]interface{}) (*tool.ToolResult, error) {
	return &tool.ToolResult{
		Output:     "artifact pipeline complete",
		Structured: map[string]any{"status": "ok"},
		Artifacts: []artifact.ArtifactRef{
			artifact.NewGeneratedRef("report-artifact", artifact.ArtifactKindDocument),
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
