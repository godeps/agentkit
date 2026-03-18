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
