//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/godeps/agentkit/pkg/api"
	"github.com/godeps/agentkit/pkg/clikit"
)

type fakeSharedEngine struct {
	events []api.StreamEvent
	turns  []clikit.ModelTurnStat
	skills []clikit.SkillMeta
}

func (f fakeSharedEngine) RunStream(context.Context, string, string) (<-chan api.StreamEvent, error) {
	ch := make(chan api.StreamEvent, len(f.events))
	for _, evt := range f.events {
		ch <- evt
	}
	close(ch)
	return ch, nil
}

func (f fakeSharedEngine) ModelTurnCount(string) int { return 0 }

func (f fakeSharedEngine) ModelTurnsSince(string, int) []clikit.ModelTurnStat {
	return append([]clikit.ModelTurnStat(nil), f.turns...)
}

func (f fakeSharedEngine) RepoRoot() string { return "" }

func (f fakeSharedEngine) ModelName() string { return "test-model" }

func (f fakeSharedEngine) Skills() []clikit.SkillMeta {
	return append([]clikit.SkillMeta(nil), f.skills...)
}

func TestSharedClikitStreamRendersToolProgress(t *testing.T) {
	index := 0
	isError := false
	engine := fakeSharedEngine{
		events: []api.StreamEvent{
			{Type: api.EventMessageStart},
			{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "hello"}},
			{Type: api.EventContentBlockStart, Index: &index, ContentBlock: &api.ContentBlock{Type: "tool_use", ID: "tool-1", Name: "file_read"}},
			{Type: api.EventContentBlockDelta, Index: &index, Delta: &api.Delta{Type: "input_json_delta", PartialJSON: []byte(`{"path":"README.md"}`)}},
			{Type: api.EventContentBlockStop, Index: &index},
			{Type: api.EventToolExecutionStart, Name: "file_read", ToolUseID: "tool-1"},
			{Type: api.EventToolExecutionResult, Name: "file_read", ToolUseID: "tool-1", Output: map[string]any{"ok": true}, IsError: &isError},
			{Type: api.EventMessageStop},
		},
		turns: []clikit.ModelTurnStat{{
			Iteration:    0,
			InputTokens:  3,
			OutputTokens: 2,
			TotalTokens:  5,
			StopReason:   "end_turn",
			Preview:      "hello",
			Timestamp:    time.Now(),
		}},
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := clikit.RunStream(context.Background(), &out, &errOut, engine, "sess-1", "hi", 0, false, clikit.WaterfallModeSummary); err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	got := out.String()
	for _, sub := range []string{"[LLM]", "[RUNNING] file_read", "[OK] file_read", "POST VALIDATION", "WATERFALL"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("missing %q in output: %s", sub, got)
		}
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestSharedClikitReplSupportsSlashCommands(t *testing.T) {
	engine := fakeSharedEngine{
		skills: []clikit.SkillMeta{{Name: "beta"}, {Name: "alpha"}},
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	input := io.NopCloser(strings.NewReader("/skills\n/quit\n"))
	clikit.PrintBanner(&out, engine.ModelName(), engine.Skills())
	clikit.RunREPL(context.Background(), input, &out, &errOut, engine, 0, false, clikit.WaterfallModeOff, "sess-1")

	got := out.String()
	for _, sub := range []string{"Agentkit CLI", "- alpha", "- beta", "bye"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("missing %q in output: %s", sub, got)
		}
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestSharedClikitConfigOutputShowsSkillsDirs(t *testing.T) {
	var out bytes.Buffer
	clikit.PrintEffectiveConfig(&out, "/repo", clikit.EffectiveConfig{
		ModelName:       "m",
		ConfigRoot:      "/cfg",
		SkillsDirs:      []string{"/skills/a", "/skills/b"},
		SkillsRecursive: func() *bool { v := true; return &v }(),
	}, 123)
	got := out.String()
	for _, sub := range []string{"repo_root: /repo", "config_root: /cfg", "/skills/a", "/skills/b"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("missing %q in output: %s", sub, got)
		}
	}
}
