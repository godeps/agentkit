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

func (fakeStreamRuntime) Run(context.Context, api.Request) (*api.Response, error) {
	return &api.Response{Result: &api.Result{Output: "ok"}}, nil
}

func (fakeStreamRuntime) Resume(context.Context, string) (*api.Response, error) {
	return &api.Response{Result: &api.Result{Output: "resumed"}}, nil
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
	t.Setenv("HOME", t.TempDir())
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

func TestRuntimeAdapterCachesDiscoveredSkills(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	adapter := NewRuntimeAdapter(fakeStreamRuntime{}, RuntimeAdapterConfig{
		ProjectRoot:  root,
		ConfigRoot:   filepath.Join(root, ".claude"),
		ModelName:    "model-x",
		TurnRecorder: newTurnRecorder(),
	})

	first := adapter.Skills()
	if len(first) != 1 || first[0].Name != "demo-skill" {
		t.Fatalf("unexpected first skills: %+v", first)
	}
	if err := os.RemoveAll(filepath.Join(root, ".claude", "skills")); err != nil {
		t.Fatalf("remove skills: %v", err)
	}

	second := adapter.Skills()
	if len(second) != 1 || second[0].Name != "demo-skill" {
		t.Fatalf("expected cached skills, got %+v", second)
	}
}

type captureRuntime struct {
	req api.Request
}

func (c *captureRuntime) RunStream(_ context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	c.req = req
	ch := make(chan api.StreamEvent)
	close(ch)
	return ch, nil
}

func (c *captureRuntime) Run(_ context.Context, req api.Request) (*api.Response, error) {
	c.req = req
	return &api.Response{Result: &api.Result{Output: "ok"}}, nil
}

func (c *captureRuntime) Resume(_ context.Context, checkpointID string) (*api.Response, error) {
	return &api.Response{Result: &api.Result{Output: checkpointID}}, nil
}

func TestRuntimeAdapterRunStreamPreservesRequest(t *testing.T) {
	rt := &captureRuntime{}
	adapter := NewRuntimeAdapter(rt, RuntimeAdapterConfig{
		ProjectRoot:  "/repo",
		ConfigRoot:   "/cfg",
		ModelName:    "model-x",
		TurnRecorder: newTurnRecorder(),
	})

	req := api.Request{
		Prompt:    "hi",
		SessionID: "sess-1",
		Mode: api.ModeContext{
			EntryPoint: api.EntryPointCLI,
			CLI: &api.CLIContext{
				Workspace: "/repo",
				Args:      []string{"--prompt", "hi"},
			},
		},
		Tags: map[string]string{"k": "v"},
	}
	if _, err := adapter.RunStream(context.Background(), req); err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	if rt.req.Mode.EntryPoint != api.EntryPointCLI {
		t.Fatalf("expected entry point preserved, got %+v", rt.req.Mode)
	}
	if rt.req.Mode.CLI == nil || rt.req.Mode.CLI.Workspace != "/repo" {
		t.Fatalf("expected cli context preserved, got %+v", rt.req.Mode.CLI)
	}
	if got := rt.req.Tags["k"]; got != "v" {
		t.Fatalf("expected tags preserved, got %+v", rt.req.Tags)
	}
}

func TestRuntimeAdapterResumePassesCheckpointID(t *testing.T) {
	rt := &captureRuntime{}
	adapter := NewRuntimeAdapter(rt, RuntimeAdapterConfig{
		ProjectRoot:  "/repo",
		ConfigRoot:   "/cfg",
		ModelName:    "model-x",
		TurnRecorder: newTurnRecorder(),
	})

	resp, err := adapter.Resume(context.Background(), "cp-1")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if resp == nil || resp.Result == nil || resp.Result.Output != "cp-1" {
		t.Fatalf("unexpected response: %+v", resp)
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

	stats := recorder.Since("sess-1", 0)
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
