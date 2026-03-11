package clikit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/middleware"
	"github.com/godeps/agentkit/pkg/model"
)

type fakeStreamRuntime struct{}

func (fakeStreamRuntime) RunStream(context.Context, api.Request) (<-chan api.StreamEvent, error) {
	ch := make(chan api.StreamEvent)
	close(ch)
	return ch, nil
}

func TestRuntimeAdapterExposesModelNameAndRepoRoot(t *testing.T) {
	recorder := newTurnRecorder()
	adapter := NewRuntimeAdapter(fakeStreamRuntime{}, RuntimeAdapterConfig{
		ProjectRoot:     "/repo",
		ConfigRoot:      "/cfg",
		ModelName:       "model-x",
		SkillsRecursive: boolPtr(true),
		TurnRecorder:    recorder,
	})

	if got := adapter.ModelName(); got != "model-x" {
		t.Fatalf("ModelName()=%q", got)
	}
	if got := adapter.RepoRoot(); got != "/repo" {
		t.Fatalf("RepoRoot()=%q", got)
	}
	if got := adapter.SettingsRoot(); got != "/cfg" {
		t.Fatalf("SettingsRoot()=%q", got)
	}
	if !adapter.SkillsRecursive() {
		t.Fatalf("SkillsRecursive() should be true")
	}
}

func TestRuntimeAdapterReturnsDiscoveredSkills(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `---
name: demo-skill
description: demo
---

body`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	adapter := NewRuntimeAdapter(fakeStreamRuntime{}, RuntimeAdapterConfig{
		ProjectRoot:  root,
		ConfigRoot:   filepath.Join(root, ".claude"),
		ModelName:    "model-x",
		TurnRecorder: newTurnRecorder(),
	})

	skills := adapter.Skills()
	if len(skills) != 1 || skills[0].Name != "demo-skill" {
		t.Fatalf("unexpected skills: %+v", skills)
	}
}

func TestTurnRecorderMiddlewareTracksModelTurns(t *testing.T) {
	recorder := newTurnRecorder()
	mw := TurnRecorderMiddleware(recorder)
	st := &middleware.State{
		Iteration: 2,
		ModelOutput: map[string]any{
			"content": "ignored",
		},
		Values: map[string]any{
			"session_id":        "sess-1",
			"model.stop_reason": "end_turn",
			"model.usage": model.Usage{
				InputTokens:  3,
				OutputTokens: 4,
				TotalTokens:  7,
			},
			"model.response": &model.Response{
				Message: model.Message{Role: "assistant", Content: "hello world"},
			},
		},
	}

	if err := mw.AfterModel(context.Background(), st); err != nil {
		t.Fatalf("AfterModel: %v", err)
	}

	stats := recorder.since("sess-1", 0)
	if len(stats) != 1 {
		t.Fatalf("unexpected stats len: %d", len(stats))
	}
	got := stats[0]
	if got.Iteration != 2 || got.InputTokens != 3 || got.OutputTokens != 4 || got.TotalTokens != 7 {
		t.Fatalf("unexpected stat: %+v", got)
	}
	if got.StopReason != "end_turn" {
		t.Fatalf("unexpected stop reason: %q", got.StopReason)
	}
	if got.Preview != "hello world" {
		t.Fatalf("unexpected preview: %q", got.Preview)
	}
	if time.Since(got.Timestamp) > time.Minute {
		t.Fatalf("timestamp too old: %v", got.Timestamp)
	}
}
