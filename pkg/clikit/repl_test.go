package clikit

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/chzyer/readline"
	"github.com/godeps/agentkit/pkg/api"
)

func TestIsReadTermination(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "eof", err: io.EOF, want: true},
		{name: "interrupt", err: readline.ErrInterrupt, want: true},
		{name: "nil", err: nil, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isReadTermination(tc.err); got != tc.want {
				t.Fatalf("isReadTermination(%v)=%v want=%v", tc.err, got, tc.want)
			}
		})
	}
}

type fakeReplEngine struct{}

func (f fakeReplEngine) RunStream(context.Context, string, string) (<-chan api.StreamEvent, error) {
	panic("unexpected")
}

func (f fakeReplEngine) ModelTurnCount(string) int { return 0 }

func (f fakeReplEngine) ModelTurnsSince(string, int) []ModelTurnStat { return nil }

func (f fakeReplEngine) RepoRoot() string { return "/repo" }

func (f fakeReplEngine) ModelName() string { return "model-x" }

func (f fakeReplEngine) Skills() []SkillMeta { return []SkillMeta{{Name: "b"}, {Name: "a"}} }

func TestHandleCommandListsSkills(t *testing.T) {
	var out bytes.Buffer
	sessionID := "s1"
	if quit := handleCommand("/skills", fakeReplEngine{}, &sessionID, &out); quit {
		t.Fatalf("skills command should not quit")
	}
	if got := out.String(); got != "- a\n- b\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}
