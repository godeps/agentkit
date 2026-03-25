package clikit

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

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

func (f fakeReplEngine) Run(context.Context, api.Request) (*api.Response, error) {
	panic("unexpected")
}

func (f fakeReplEngine) Resume(context.Context, string) (*api.Response, error) {
	panic("unexpected")
}

func (f fakeReplEngine) ModelTurnCount(string) int { return 0 }

func (f fakeReplEngine) ModelTurnsSince(string, int) []ModelTurnStat { return nil }

func (f fakeReplEngine) RepoRoot() string { return "/repo" }

func (f fakeReplEngine) ModelName() string { return "model-x" }

func (f fakeReplEngine) Skills() []SkillMeta { return []SkillMeta{{Name: "b"}, {Name: "a"}} }

func (f fakeReplEngine) SandboxBackend() string { return "govm" }

func (f fakeReplEngine) Timeline(resp *api.Response) []api.TimelineEntry {
	if resp == nil {
		return nil
	}
	return resp.Timeline
}

func TestHandleCommandListsSkills(t *testing.T) {
	var out bytes.Buffer
	sessionID := "s1"
	handled, quit := handleCommand("/skills", fakeReplEngine{}, &sessionID, &shellState{}, context.Background(), &out, io.Discard, 0)
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
	calls       []api.Request
	streamCalls []api.Request
	runResponse *api.Response
	runErr      error
	resumeIDs   []string
	resumeResp  *api.Response
	resumeErr   error
	resumeFn    func(context.Context, string) (*api.Response, error)
}

func (s *scriptedReplEngine) RunStream(_ context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	s.streamCalls = append(s.streamCalls, req)
	if s.runErr != nil {
		err := s.runErr
		s.runErr = nil
		return nil, err
	}
	ch := make(chan api.StreamEvent)
	close(ch)
	return ch, nil
}

func (s *scriptedReplEngine) Run(_ context.Context, req api.Request) (*api.Response, error) {
	s.calls = append(s.calls, req)
	if s.runErr != nil {
		err := s.runErr
		s.runErr = nil
		return nil, err
	}
	if s.runResponse != nil {
		return s.runResponse, nil
	}
	return &api.Response{Result: &api.Result{Output: "ok"}}, nil
}

func (s *scriptedReplEngine) Resume(ctx context.Context, checkpointID string) (*api.Response, error) {
	s.resumeIDs = append(s.resumeIDs, checkpointID)
	if s.resumeFn != nil {
		return s.resumeFn(ctx, checkpointID)
	}
	if s.resumeErr != nil {
		err := s.resumeErr
		s.resumeErr = nil
		return nil, err
	}
	if s.resumeResp != nil {
		return s.resumeResp, nil
	}
	return &api.Response{Result: &api.Result{Output: "resumed"}}, nil
}

func (s *scriptedReplEngine) ModelTurnCount(string) int                   { return 0 }
func (s *scriptedReplEngine) ModelTurnsSince(string, int) []ModelTurnStat { return nil }
func (s *scriptedReplEngine) RepoRoot() string                            { return "/repo" }
func (s *scriptedReplEngine) ModelName() string                           { return "model-x" }
func (s *scriptedReplEngine) Skills() []SkillMeta                         { return []SkillMeta{{Name: "b"}, {Name: "a"}} }
func (s *scriptedReplEngine) SandboxBackend() string                      { return "govm" }
func (s *scriptedReplEngine) Timeline(resp *api.Response) []api.TimelineEntry {
	if resp == nil {
		return nil
	}
	return resp.Timeline
}

func TestInteractiveShellPrintsStatusAndContinuesAfterErrors(t *testing.T) {
	origRunStream := runStreamRenderer
	t.Cleanup(func() { runStreamRenderer = origRunStream })
	runStreamRenderer = func(parent context.Context, out, errOut io.Writer, eng StreamEngine, req api.Request, timeoutMs int, verbose bool, waterfallMode string) error {
		if timeoutMs != 100 || verbose || waterfallMode != WaterfallModeOff {
			t.Fatalf("unexpected renderer config timeout=%d verbose=%v waterfall=%s", timeoutMs, verbose, waterfallMode)
		}
		_, err := eng.RunStream(parent, req)
		return err
	}
	in := io.NopCloser(bytes.NewBufferString("/session\nhello\nworld\n/quit\n"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	eng := &scriptedReplEngine{}
	eng.runErr = io.ErrUnexpectedEOF

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
	if len(eng.streamCalls) != 2 {
		t.Fatalf("expected two prompts to be attempted, got %+v", eng.streamCalls)
	}
	if eng.streamCalls[0].SessionID != "sess-1" || eng.streamCalls[0].Prompt != "hello" {
		t.Fatalf("unexpected first call: %+v", eng.streamCalls[0])
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
	origRunStream := runStreamRenderer
	t.Cleanup(func() { runStreamRenderer = origRunStream })
	runStreamRenderer = func(parent context.Context, out, errOut io.Writer, eng StreamEngine, req api.Request, timeoutMs int, verbose bool, waterfallMode string) error {
		_, err := eng.RunStream(parent, req)
		return err
	}
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

	if len(eng.streamCalls) != 1 || eng.streamCalls[0].SessionID != "sess-2" || eng.streamCalls[0].Prompt != "/unknown hi" {
		t.Fatalf("unexpected stream calls: %+v", eng.streamCalls)
	}
	if got := out.String(); bytes.Contains([]byte(got), []byte("unknown command")) {
		t.Fatalf("unexpected unknown command output: %q", got)
	}
}

func TestInteractiveShellUsesRunStreamRendererForNormalInput(t *testing.T) {
	origRunStream := runStreamRenderer
	t.Cleanup(func() { runStreamRenderer = origRunStream })
	called := false
	runStreamRenderer = func(parent context.Context, out, errOut io.Writer, eng StreamEngine, req api.Request, timeoutMs int, verbose bool, waterfallMode string) error {
		called = true
		if req.Prompt != "hello" || req.SessionID != "sess-4" {
			t.Fatalf("unexpected request: %+v", req)
		}
		if timeoutMs != 250 || !verbose || waterfallMode != WaterfallModeSummary {
			t.Fatalf("unexpected renderer config timeout=%d verbose=%v waterfall=%s", timeoutMs, verbose, waterfallMode)
		}
		return nil
	}

	in := io.NopCloser(bytes.NewBufferString("hello\n/quit\n"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	eng := &scriptedReplEngine{}
	shell := NewInteractiveShell(InteractiveShellConfig{
		Engine:            eng,
		InitialSessionID:  "sess-4",
		TimeoutMs:         250,
		Verbose:           true,
		WaterfallMode:     WaterfallModeSummary,
		ShowStatusPerTurn: false,
	})
	if err := shell.Run(context.Background(), in, &out, &errOut); err != nil {
		t.Fatalf("run shell: %v", err)
	}
	if !called {
		t.Fatalf("expected renderer to be called")
	}
	if len(eng.calls) != 0 {
		t.Fatalf("normal input should not use Run path: %+v", eng.calls)
	}
}

func TestHandleCommandResumeUsesPerTurnTimeoutContext(t *testing.T) {
	var out bytes.Buffer
	sessionID := "s1"
	state := &shellState{}
	eng := &scriptedReplEngine{
		resumeFn: func(ctx context.Context, checkpointID string) (*api.Response, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatalf("expected deadline on resume context")
			}
			if time.Until(deadline) <= 0 || time.Until(deadline) > 200*time.Millisecond {
				t.Fatalf("unexpected deadline delta: %v", time.Until(deadline))
			}
			return &api.Response{Result: &api.Result{Output: checkpointID}}, nil
		},
	}
	handled, quit := handleCommand("/resume cp-1", eng, &sessionID, state, context.Background(), &out, io.Discard, 150)
	if !handled || quit {
		t.Fatalf("expected handled resume command")
	}
}

func TestIsReadTerminationIgnoresInterrupt(t *testing.T) {
	if isReadTermination(readline.ErrInterrupt) {
		t.Fatalf("interrupt should not terminate repl")
	}
}

func TestInteractiveShellResumeTimelineAndCheckpointCommands(t *testing.T) {
	in := io.NopCloser(bytes.NewBufferString("/resume cp-1\n/checkpoint\n/timeline\n/quit\n"))
	var out bytes.Buffer
	var errOut bytes.Buffer
	eng := &scriptedReplEngine{
		resumeResp: &api.Response{
			Result: &api.Result{
				Output:       "resumed output",
				CheckpointID: "cp-2",
			},
			Timeline: []api.TimelineEntry{
				{Kind: api.TimelineKindResume, Source: api.TimelineEventResume},
				{Kind: api.TimelineKindToolResult, Source: api.EventToolExecutionResult},
			},
		},
	}

	shell := NewInteractiveShell(InteractiveShellConfig{
		Engine:            eng,
		InitialSessionID:  "sess-3",
		TimeoutMs:         100,
		Verbose:           false,
		WaterfallMode:     WaterfallModeOff,
		ShowStatusPerTurn: false,
	})
	if err := shell.Run(context.Background(), in, &out, &errOut); err != nil {
		t.Fatalf("run shell: %v", err)
	}

	if len(eng.resumeIDs) != 1 || eng.resumeIDs[0] != "cp-1" {
		t.Fatalf("unexpected resume ids: %+v", eng.resumeIDs)
	}
	got := out.String()
	for _, sub := range []string{"resumed output", "checkpoint: cp-2", "resume", "tool_result"} {
		if !bytes.Contains([]byte(got), []byte(sub)) {
			t.Fatalf("missing %q in output: %q", sub, got)
		}
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}

func TestHandleCommandTimelineWithoutLastResponse(t *testing.T) {
	var out bytes.Buffer
	sessionID := "s1"
	state := &shellState{}
	handled, quit := handleCommand("/timeline", fakeReplEngine{}, &sessionID, state, context.Background(), &out, io.Discard, 0)
	if !handled || quit {
		t.Fatalf("timeline command should be handled without quit")
	}
	if got := out.String(); got != "no timeline available\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}
