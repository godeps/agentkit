package api

import (
	"context"
	"testing"

	"github.com/godeps/agentkit/pkg/model"
	"github.com/godeps/agentkit/pkg/runtime/skills"
)

func TestExecuteSkillsDedupesAutoAndForcedMatches(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}}}

	var calls int
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       mdl,
		Skills: []SkillRegistration{{
			Definition: skills.Definition{
				Name:     "tagger",
				Matchers: []skills.Matcher{skills.KeywordMatcher{Any: []string{"trigger"}}},
			},
			Handler: skills.HandlerFunc(func(context.Context, skills.ActivationContext) (skills.Result, error) {
				calls++
				return skills.Result{Output: "skill-prefix"}, nil
			}),
		}},
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{
		Prompt:      "trigger",
		ForceSkills: []string{"tagger"},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one skill execution, got %d", calls)
	}
	if len(resp.SkillResults) != 1 {
		t.Fatalf("expected one skill result, got %d", len(resp.SkillResults))
	}
	if resp.SkillResults[0].Definition.Name != "tagger" {
		t.Fatalf("unexpected skill result %+v", resp.SkillResults[0])
	}
}

func TestRunKeepsUnknownPromptSkillMarkersAsPlainText(t *testing.T) {
	root := newClaudeProject(t)
	mdl := &stubModel{responses: []*model.Response{{Message: model.Message{Role: "assistant", Content: "ok"}}}}

	var activation skills.ActivationContext
	rt, err := New(context.Background(), Options{
		ProjectRoot: root,
		Model:       mdl,
		Skills: []SkillRegistration{{
			Definition: skills.Definition{Name: "mpp-pipeline-orchestrator"},
			Handler: skills.HandlerFunc(func(_ context.Context, ac skills.ActivationContext) (skills.Result, error) {
				activation = ac
				return skills.Result{}, nil
			}),
		}},
	})
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	resp, err := rt.Run(context.Background(), Request{
		Prompt: "$mpp-pipeline-orchestrator\n处理该任务。中间结果放在 $jobId 目录。",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(resp.SkillResults) != 1 {
		t.Fatalf("expected one skill result, got %d", len(resp.SkillResults))
	}
	if got := activation.Prompt; got != "处理该任务。中间结果放在 $jobId 目录。" {
		t.Fatalf("unexpected skill activation prompt %q", got)
	}
	if len(mdl.requests) == 0 {
		t.Fatal("expected model request")
	}
	last := mdl.requests[len(mdl.requests)-1]
	if len(last.Messages) == 0 {
		t.Fatal("expected model messages")
	}
	if got := last.Messages[len(last.Messages)-1].TextContent(); got != "处理该任务。中间结果放在 $jobId 目录。" {
		t.Fatalf("unexpected model prompt %q", got)
	}
}
