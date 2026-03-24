package api

import (
	"context"
	"testing"

	"github.com/godeps/agentkit/pkg/artifact"
	runtimecache "github.com/godeps/agentkit/pkg/runtime/cache"
	"github.com/godeps/agentkit/pkg/runtime/checkpoint"
	"github.com/godeps/agentkit/pkg/pipeline"
	"github.com/godeps/agentkit/pkg/tool"
)

func TestTimelineIncludesArtifactsToolEventsAndCheckpoint(t *testing.T) {
	root := newClaudeProject(t)
	rt, err := New(context.Background(), Options{
		ProjectRoot:     root,
		Model:           &stubModel{},
		CustomTools: []tool.Tool{&pipelineStepTool{outputs: map[string]*tool.ToolResult{
			"generate": {Output: "generated", Artifacts: []artifact.ArtifactRef{artifact.NewGeneratedRef("art_out", artifact.ArtifactKindText)}},
			"review":   {Output: "reviewed"},
		}}},
		CheckpointStore: checkpoint.NewMemoryStore(),
		CacheStore:      runtimecache.NewMemoryStore(),
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{
		Pipeline: &pipeline.Step{
			Batch: &pipeline.Batch{
				Steps: []pipeline.Step{
					{
						Name: "generate",
						Tool: "pipeline_step",
						Input: []artifact.ArtifactRef{
							artifact.NewGeneratedRef("art_in", artifact.ArtifactKindImage),
						},
						With: map[string]any{"prompt": "describe"},
					},
					{
						Checkpoint: &pipeline.Checkpoint{
							Name: "review-gate",
							Step: pipeline.Step{Name: "review", Tool: "pipeline_step"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, kind := range []string{
		TimelineInputArtifact,
		TimelineGeneratedArtifact,
		TimelineToolCall,
		TimelineToolResult,
		TimelineCacheMiss,
		TimelineCheckpointCreate,
		TimelineTokenSnapshot,
		TimelineLatencySnapshot,
	} {
		if !hasTimelineKind(resp.Timeline, kind) {
			t.Fatalf("expected timeline to include %q, got %+v", kind, resp.Timeline)
		}
	}
}

func TestTimelineIncludesCacheHitAndCheckpointResume(t *testing.T) {
	root := newClaudeProject(t)
	cacheStore := runtimecache.NewMemoryStore()
	checkpointStore := checkpoint.NewMemoryStore()
	rt, err := New(context.Background(), Options{
		ProjectRoot:     root,
		Model:           &stubModel{},
		CustomTools: []tool.Tool{&pipelineStepTool{outputs: map[string]*tool.ToolResult{
			"generate": {Output: "generated"},
			"review":   {Output: "reviewed"},
			"finalize": {Output: "done"},
		}}},
		CheckpointStore: checkpointStore,
		CacheStore:      cacheStore,
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	_, err = rt.Run(context.Background(), Request{
		Pipeline: &pipeline.Step{
			Name: "generate",
			Tool: "pipeline_step",
			Input: []artifact.ArtifactRef{
				artifact.NewGeneratedRef("art_in", artifact.ArtifactKindImage),
			},
			With: map[string]any{"prompt": "cached"},
		},
	})
	if err != nil {
		t.Fatalf("prime cache: %v", err)
	}
	cachedResp, err := rt.Run(context.Background(), Request{
		Pipeline: &pipeline.Step{
			Name: "generate",
			Tool: "pipeline_step",
			Input: []artifact.ArtifactRef{
				artifact.NewGeneratedRef("art_in", artifact.ArtifactKindImage),
			},
			With: map[string]any{"prompt": "cached"},
		},
	})
	if err != nil {
		t.Fatalf("cache hit run: %v", err)
	}
	if !hasTimelineKind(cachedResp.Timeline, TimelineCacheHit) {
		t.Fatalf("expected cache hit entry, got %+v", cachedResp.Timeline)
	}

	first, err := rt.Run(context.Background(), Request{
		SessionID: "resume-timeline",
		Pipeline: &pipeline.Step{
			Batch: &pipeline.Batch{
				Steps: []pipeline.Step{
					{Name: "generate", Tool: "pipeline_step"},
					{Checkpoint: &pipeline.Checkpoint{Name: "review-gate", Step: pipeline.Step{Name: "review", Tool: "pipeline_step"}}},
					{Name: "finalize", Tool: "pipeline_step"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("checkpoint run: %v", err)
	}
	resumed, err := rt.Run(context.Background(), Request{
		SessionID:            "resume-timeline",
		ResumeFromCheckpoint: first.Result.CheckpointID,
	})
	if err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if !hasTimelineKind(resumed.Timeline, TimelineCheckpointResume) {
		t.Fatalf("expected checkpoint resume entry, got %+v", resumed.Timeline)
	}
}

func hasTimelineKind(entries []TimelineEntry, kind string) bool {
	for _, entry := range entries {
		if entry.Kind == kind {
			return true
		}
	}
	return false
}
