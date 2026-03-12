package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/godeps/agentkit/pkg/api"
)

type fakeRuntime struct {
	runFn       func(context.Context, api.Request) (*api.Response, error)
	runStreamFn func(context.Context, api.Request) (<-chan api.StreamEvent, error)
	closeFn     func() error
}

func (f *fakeRuntime) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	if f.runFn != nil {
		return f.runFn(ctx, req)
	}
	return &api.Response{Result: &api.Result{Output: "ok"}}, nil
}

func (f *fakeRuntime) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	if f.runStreamFn != nil {
		return f.runStreamFn(ctx, req)
	}
	ch := make(chan api.StreamEvent)
	close(ch)
	return ch, nil
}

func (f *fakeRuntime) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func TestRunACPModeNoPrompt(t *testing.T) {
	originalServe := serveACPStdio
	t.Cleanup(func() {
		serveACPStdio = originalServe
	})

	called := false
	serveACPStdio = func(ctx context.Context, options api.Options, stdin io.Reader, stdout io.Writer) error {
		called = true
		return nil
	}

	if err := run([]string{"--acp=true"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run with --acp=true should not require prompt: %v", err)
	}
	if !called {
		t.Fatalf("expected ACP serve path to be called")
	}
}

func TestRunNonACPModeWithoutPromptErrors(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	originalStdin := os.Stdin
	os.Stdin = devNull
	t.Cleanup(func() {
		os.Stdin = originalStdin
	})

	err = run(nil, io.Discard, io.Discard)
	if err == nil {
		t.Fatalf("expected error when no prompt is provided in non-ACP mode")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "prompt") {
		t.Fatalf("expected prompt-related error, got: %v", err)
	}
}

func TestRunPrintsSharedEffectiveConfig(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			runFn: func(context.Context, api.Request) (*api.Response, error) {
				return &api.Response{Mode: api.ModeContext{EntryPoint: api.EntryPointCLI}, Result: &api.Result{Output: "ok"}}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := run([]string{"--prompt", "hi", "--print-effective-config"}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stderr.String(); !strings.Contains(got, "effective-config (pre-runtime)") {
		t.Fatalf("expected shared config output, got: %s", got)
	}
}

func TestRunStreamUsesClikitRendererWhenEnabled(t *testing.T) {
	origFactory := runtimeFactory
	origRunStream := clikitRunStream
	t.Cleanup(func() {
		runtimeFactory = origFactory
		clikitRunStream = origRunStream
	})
	rt := &fakeRuntime{}
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return rt, nil
	}
	called := false
	clikitRunStream = func(ctx context.Context, out, errOut io.Writer, eng streamEngine, sessionID, prompt string, timeoutMs int, verbose bool, waterfallMode string) error {
		called = true
		if prompt != "hi" {
			t.Fatalf("unexpected prompt: %q", prompt)
		}
		return nil
	}

	if err := run([]string{"--prompt", "hi", "--stream", "--stream-format", "rendered"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !called {
		t.Fatalf("expected clikit stream renderer to be called")
	}
}

func TestRunStreamJSONRemainsMachineReadableByDefault(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			runStreamFn: func(context.Context, api.Request) (<-chan api.StreamEvent, error) {
				ch := make(chan api.StreamEvent, 1)
				ch <- api.StreamEvent{Type: api.EventMessageStop}
				close(ch)
				return ch, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := run([]string{"--prompt", "hi", "--stream"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, `"type":"message_stop"`) {
		t.Fatalf("expected json stream output, got: %s", got)
	}
}

func TestCLIReplUsesSharedBannerAndCommandLoop(t *testing.T) {
	origFactory := runtimeFactory
	origRunREPL := clikitRunREPL
	t.Cleanup(func() {
		runtimeFactory = origFactory
		clikitRunREPL = origRunREPL
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{}, nil
	}
	called := false
	clikitRunREPL = func(ctx context.Context, in io.ReadCloser, out, errOut io.Writer, eng replEngine, timeoutMs int, verbose bool, waterfallMode string, initialSessionID string) {
		called = true
	}

	if err := run([]string{"--repl"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !called {
		t.Fatalf("expected clikit repl to be called")
	}
}

func TestRunGVisorHelperMode(t *testing.T) {
	orig := runGVisorHelper
	t.Cleanup(func() {
		runGVisorHelper = orig
	})
	called := false
	runGVisorHelper = func(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
		called = true
		_, _ = io.WriteString(stdout, `{"success":true}`+"\n")
		return nil
	}

	var stdout bytes.Buffer
	if err := run([]string{"--agentkit-gvisor-helper"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run helper: %v", err)
	}
	if !called {
		t.Fatalf("expected helper path to be called")
	}
	if !strings.Contains(stdout.String(), `"success":true`) {
		t.Fatalf("unexpected helper stdout: %s", stdout.String())
	}
}

func TestRunGVisorRunscMode(t *testing.T) {
	orig := runGVisorRunsc
	t.Cleanup(func() {
		runGVisorRunsc = orig
	})
	called := false
	runGVisorRunsc = func() error {
		called = true
		return nil
	}

	if err := run([]string{"--agentkit-gvisor-runsc"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run runsc mode: %v", err)
	}
	if !called {
		t.Fatalf("expected runsc path to be called")
	}
}

func TestRunGVisorRunscModeByArgv0(t *testing.T) {
	orig := runGVisorRunsc
	origArgv0 := os.Args[0]
	t.Cleanup(func() {
		runGVisorRunsc = orig
		os.Args[0] = origArgv0
	})
	called := false
	runGVisorRunsc = func() error {
		called = true
		return nil
	}
	os.Args[0] = "runsc-gofer"

	if err := run([]string{"--root=/tmp/state", "gofer"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run argv0 runsc mode: %v", err)
	}
	if !called {
		t.Fatalf("expected runsc argv0 path to be called")
	}
}
