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
		{name: "interrupt", err: readline.ErrInterrupt, want: false},
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

func (f fakeReplEngine) RunStream(context.Context, api.Request) (<-chan api.StreamEvent, error) {
	panic("unexpected")
}

func (f fakeReplEngine) ModelTurnCount(string) int { return 0 }

func (f fakeReplEngine) ModelTurnsSince(string, int) []ModelTurnStat { return nil }

func (f fakeReplEngine) RepoRoot() string { return "/repo" }

func (f fakeReplEngine) ModelName() string { return "model-x" }

func (f fakeReplEngine) Skills() []SkillMeta { return []SkillMeta{{Name: "b"}, {Name: "a"}} }

func (f fakeReplEngine) SandboxBackend() string { return "govm" }

func TestHandleCommandListsSkills(t *testing.T) {
	var out bytes.Buffer
	sessionID := "s1"
	handled, quit := handleCommand("/skills", fakeReplEngine{}, &sessionID, &out)
	if !handled {
		t.Fatalf("skills command should be handled")
	}
	if quit {
		t.Fatalf("skills command should not quit")
	}
	if got := out.String(); got != "- a\n- b\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

type scriptedReplEngine struct {
	calls []api.Request
	fail  bool
}

func (s *scriptedReplEngine) RunStream(_ context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	s.calls = append(s.calls, req)
	if s.fail {
		s.fail = false
		return nil, io.ErrUnexpectedEOF
	}
	ch := make(chan api.StreamEvent)
	close(ch)
	return ch, nil
}

func (s *scriptedReplEngine) ModelTurnCount(string) int                   { return 0 }
func (s *scriptedReplEngine) ModelTurnsSince(string, int) []ModelTurnStat { return nil }
func (s *scriptedReplEngine) RepoRoot() string                            { return "/repo" }
func (s *scriptedReplEngine) ModelName() string                           { return "model-x" }
func (s *scriptedReplEngine) Skills() []SkillMeta                         { return []SkillMeta{{Name: "b"}, {Name: "a"}} }
func (s *scriptedReplEngine) SandboxBackend() string                      { return "govm" }

func TestInteractiveShellPrintsStatusAndContinuesAfterErrors(t *testing.T) {
	in := io.NopCloser(bytes.NewBufferString("/session\nhello\nworld\n/quit\n"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	eng := &scriptedReplEngine{fail: true}

	shell := NewInteractiveShell(InteractiveShellConfig{
		Engine:            eng,
		InitialSessionID:  "sess-1",
		TimeoutMs:         100,
		Verbose:           false,
		WaterfallMode:     WaterfallModeOff,
		ShowStatusPerTurn: true,
	})
	if err := shell.Run(context.Background(), in, &out, &errOut); err != nil {
		t.Fatalf("run shell: %v", err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("Session: sess-1")) {
		t.Fatalf("expected session status, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Model: model-x")) {
		t.Fatalf("expected model status, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Repo: /repo")) {
		t.Fatalf("expected repo status, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Sandbox: govm")) {
		t.Fatalf("expected sandbox status, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Skills: 2")) {
		t.Fatalf("expected skills count, got %q", got)
	}
	if len(eng.calls) != 2 {
		t.Fatalf("expected two prompts to be attempted, got %+v", eng.calls)
	}
	if eng.calls[0].SessionID != "sess-1" || eng.calls[0].Prompt != "hello" {
		t.Fatalf("unexpected first call: %+v", eng.calls[0])
	}
	if errText := errOut.String(); errText == "" || !bytes.Contains([]byte(errText), []byte("run failed")) {
		t.Fatalf("expected run failure on stderr, got %q", errText)
	}
	if !bytes.Contains([]byte(got), []byte("bye")) {
		t.Fatalf("expected clean exit, got %q", got)
	}
	if bytes.Count([]byte(got), []byte("bye")) != 1 {
		t.Fatalf("expected single bye, got %q", got)
	}
}

func TestInteractiveShellUnknownSlashInputFallsThrough(t *testing.T) {
	in := io.NopCloser(bytes.NewBufferString("/unknown hi\n/quit\n"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	eng := &scriptedReplEngine{}

	shell := NewInteractiveShell(InteractiveShellConfig{
		Engine:            eng,
		InitialSessionID:  "sess-2",
		TimeoutMs:         100,
		Verbose:           false,
		WaterfallMode:     WaterfallModeOff,
		ShowStatusPerTurn: false,
	})
	if err := shell.Run(context.Background(), in, &out, &errOut); err != nil {
		t.Fatalf("run shell: %v", err)
	}

	if len(eng.calls) != 1 || eng.calls[0].SessionID != "sess-2" || eng.calls[0].Prompt != "/unknown hi" {
		t.Fatalf("unexpected stream calls: %+v", eng.calls)
	}
	if got := out.String(); bytes.Contains([]byte(got), []byte("unknown command")) {
		t.Fatalf("unexpected unknown command output: %q", got)
	}
}

func TestIsReadTerminationIgnoresInterrupt(t *testing.T) {
	if isReadTermination(readline.ErrInterrupt) {
		t.Fatalf("interrupt should not terminate repl")
	}
}
