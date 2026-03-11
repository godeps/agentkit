package api

import (
	"context"
	"testing"
	"time"

	"github.com/godeps/agentkit/pkg/middleware"
	"github.com/godeps/agentkit/pkg/model"
	runtimeskills "github.com/godeps/agentkit/pkg/runtime/skills"
)

func TestRuntimeAvailableSkillsSnapshot(t *testing.T) {
	reg := runtimeskills.NewRegistry()
	if err := reg.Register(runtimeskills.Definition{Name: "demo", Description: "d"}, runtimeskills.HandlerFunc(func(context.Context, runtimeskills.ActivationContext) (runtimeskills.Result, error) {
		return runtimeskills.Result{}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}
	rt := &Runtime{skReg: reg}

	skills := rt.AvailableSkills()
	if len(skills) != 1 || skills[0].Name != "demo" || skills[0].Description != "d" {
		t.Fatalf("unexpected skills: %+v", skills)
	}
}

func TestModelTurnRecorderMiddlewareTracksTurns(t *testing.T) {
	recorder := NewModelTurnRecorder()
	mw := ModelTurnRecorderMiddleware(recorder)
	st := &middleware.State{
		Iteration: 1,
		Values: map[string]any{
			"session_id":        "sess-1",
			"model.stop_reason": "end_turn",
			"model.usage": model.Usage{
				InputTokens:  2,
				OutputTokens: 3,
				TotalTokens:  5,
			},
			"model.response": &model.Response{
				Message: model.Message{Role: "assistant", Content: "hello"},
			},
		},
	}

	if err := mw.AfterModel(context.Background(), st); err != nil {
		t.Fatalf("AfterModel: %v", err)
	}
	stats := recorder.Since("sess-1", 0)
	if len(stats) != 1 {
		t.Fatalf("unexpected stats len: %d", len(stats))
	}
	got := stats[0]
	if got.Iteration != 1 || got.TotalTokens != 5 || got.Preview != "hello" || got.StopReason != "end_turn" {
		t.Fatalf("unexpected stat: %+v", got)
	}
	if time.Since(got.Timestamp) > time.Minute {
		t.Fatalf("timestamp too old: %v", got.Timestamp)
	}
}
