package main

import (
	"context"
	"fmt"
	"log"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/pipeline"
	"github.com/godeps/agentkit/pkg/runtime/checkpoint"
	"github.com/godeps/agentkit/pkg/tool"
)

func main() {
	store := checkpoint.NewMemoryStore()
	rt, err := api.New(context.Background(), api.Options{
		ProjectRoot:     ".",
		Model:           staticModel{},
		CustomTools:     []tool.Tool{reviewTool{}},
		CheckpointStore: store,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = rt.Close() }()

	first, err := rt.Run(context.Background(), api.Request{
		SessionID: "review-demo",
		Pipeline: &pipeline.Step{
			Batch: &pipeline.Batch{
				Steps: []pipeline.Step{
					{Name: "draft", Tool: "review_tool"},
					{Checkpoint: &pipeline.Checkpoint{Name: "await-human-review", Step: pipeline.Step{Name: "review", Tool: "review_tool"}}},
					{Name: "finalize", Tool: "review_tool"},
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("interrupted=%v checkpoint=%s\n", first.Result.Interrupted, first.Result.CheckpointID)

	second, err := rt.Run(context.Background(), api.Request{
		SessionID:            "review-demo",
		ResumeFromCheckpoint: first.Result.CheckpointID,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("resumed output=%q\n", second.Result.Output)
}

type reviewTool struct{}

func (reviewTool) Name() string             { return "review_tool" }
func (reviewTool) Description() string      { return "returns step-specific review text" }
func (reviewTool) Schema() *tool.JSONSchema { return &tool.JSONSchema{Type: "object"} }
func (reviewTool) Execute(_ context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	step, _ := params["step"].(string)
	return &tool.ToolResult{Output: step}, nil
}

type staticModel struct{}

func (staticModel) Complete(context.Context, model.Request) (*model.Response, error) {
	return &model.Response{Message: model.Message{Role: "assistant", Content: "unused"}}, nil
}

func (staticModel) CompleteStream(context.Context, model.Request, model.StreamHandler) error {
	return nil
}
