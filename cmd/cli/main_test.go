package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godeps/agentkit/pkg/api"
	govmclient "github.com/godeps/govm/pkg/client"
)

type fakeRuntime struct {
	runFn       func(context.Context, api.Request) (*api.Response, error)
	runStreamFn func(context.Context, api.Request) (<-chan api.StreamEvent, error)
	resumeFn    func(context.Context, string) (*api.Response, error)
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

func (f *fakeRuntime) Resume(ctx context.Context, checkpointID string) (*api.Response, error) {
	if f.resumeFn != nil {
		return f.resumeFn(ctx, checkpointID)
	}
	return &api.Response{Result: &api.Result{Output: "resumed"}}, nil
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

func TestRunNonACPModeWithoutPromptDefaultsToInteractiveShell(t *testing.T) {
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

	origFactory := runtimeFactory
	origRunInteractive := clikitRunInteractiveShell
	t.Cleanup(func() {
		runtimeFactory = origFactory
		clikitRunInteractiveShell = origRunInteractive
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{}, nil
	}
	called := false
	clikitRunInteractiveShell = func(ctx context.Context, in io.ReadCloser, out, errOut io.Writer, eng replEngine, timeoutMs int, verbose bool, waterfallMode string, initialSessionID string) error {
		called = true
		return nil
	}

	err = run(nil, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected interactive shell fallback, got: %v", err)
	}
	if !called {
		t.Fatal("expected interactive shell to be called")
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

func TestRunResumeDoesNotRequirePrompt(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			resumeFn: func(_ context.Context, checkpointID string) (*api.Response, error) {
				if checkpointID != "cp-1" {
					t.Fatalf("unexpected checkpoint id: %q", checkpointID)
				}
				return &api.Response{
					Mode: api.ModeContext{EntryPoint: api.EntryPointCLI},
					Result: &api.Result{
						Output:       "resumed ok",
						CheckpointID: "cp-2",
					},
				}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := run([]string{"--resume", "cp-1"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := stdout.String()
	for _, sub := range []string{"resumed ok", "checkpoint_id: cp-2"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("missing %q in output: %s", sub, got)
		}
	}
}

func TestRunPrintTimelineForResume(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			resumeFn: func(_ context.Context, checkpointID string) (*api.Response, error) {
				return &api.Response{
					Mode:   api.ModeContext{EntryPoint: api.EntryPointCLI},
					Result: &api.Result{Output: checkpointID},
					Timeline: []api.TimelineEntry{
						{Kind: api.TimelineKindResume, Source: api.TimelineEventResume},
					},
				}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := run([]string{"--resume", "cp-1", "--print-timeline"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "timeline:") || !strings.Contains(got, "resume") {
		t.Fatalf("expected timeline output, got: %s", got)
	}
}

func TestRunPrintTimelineForNormalRun(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			runFn: func(context.Context, api.Request) (*api.Response, error) {
				return &api.Response{
					Mode:   api.ModeContext{EntryPoint: api.EntryPointCLI},
					Result: &api.Result{Output: "ok"},
					Timeline: []api.TimelineEntry{
						{Kind: api.TimelineKindToolResult, Source: api.EventToolExecutionResult},
					},
				}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := run([]string{"--prompt", "hi", "--print-timeline"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "timeline:") || !strings.Contains(got, "tool_result") {
		t.Fatalf("expected timeline output, got: %s", got)
	}
}

func TestRunInterruptedPrintsResumeHint(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			runFn: func(context.Context, api.Request) (*api.Response, error) {
				return &api.Response{
					Mode: api.ModeContext{EntryPoint: api.EntryPointCLI},
					Result: &api.Result{
						Output:       "paused",
						Interrupted:  true,
						CheckpointID: "cp-7",
					},
				}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := run([]string{"--prompt", "hi"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := stdout.String()
	for _, sub := range []string{"interrupted: true", "checkpoint_id: cp-7", "next: agentkit --resume cp-7"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("missing %q in output: %s", sub, got)
		}
	}
}

func TestRunResumeErrorIncludesCheckpointID(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			resumeFn: func(_ context.Context, checkpointID string) (*api.Response, error) {
				return nil, errors.New("missing checkpoint")
			},
		}, nil
	}

	err := run([]string{"--resume", "cp-404"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); !strings.Contains(got, "resume failed for checkpoint cp-404") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPrintTimelineUsesIndexedLines(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			runFn: func(context.Context, api.Request) (*api.Response, error) {
				return &api.Response{
					Mode:   api.ModeContext{EntryPoint: api.EntryPointCLI},
					Result: &api.Result{Output: "ok"},
					Timeline: []api.TimelineEntry{
						{Kind: api.TimelineKindResume, Source: api.TimelineEventResume},
						{Kind: api.TimelineKindToolResult, Source: api.EventToolExecutionResult},
					},
				}, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	if err := run([]string{"--prompt", "hi", "--print-timeline"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := stdout.String()
	for _, sub := range []string{"timeline:", "1. resume source=TimelineResume", "2. tool_result source=tool_execution_result"} {
		if !strings.Contains(got, sub) {
			t.Fatalf("missing %q in output: %s", sub, got)
		}
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
	clikitRunStream = func(ctx context.Context, out, errOut io.Writer, eng streamEngine, req api.Request, timeoutMs int, verbose bool, waterfallMode string) error {
		called = true
		if req.Prompt != "hi" {
			t.Fatalf("unexpected prompt: %q", req.Prompt)
		}
		if req.Mode.EntryPoint != api.EntryPointCLI || req.Mode.CLI == nil {
			t.Fatalf("expected full request context, got %+v", req.Mode)
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

func TestRunStreamJSONReturnsErrorOnEventError(t *testing.T) {
	origFactory := runtimeFactory
	t.Cleanup(func() {
		runtimeFactory = origFactory
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{
			runStreamFn: func(context.Context, api.Request) (<-chan api.StreamEvent, error) {
				ch := make(chan api.StreamEvent, 1)
				ch <- api.StreamEvent{Type: api.EventError, Output: "boom"}
				close(ch)
				return ch, nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	err := run([]string{"--prompt", "hi", "--stream"}, &stdout, io.Discard)
	if err == nil {
		t.Fatalf("expected stream error")
	}
	if got := stdout.String(); !strings.Contains(got, `"type":"error"`) {
		t.Fatalf("expected json event output, got: %s", got)
	}
}

func TestCLIReplUsesSharedBannerAndCommandLoop(t *testing.T) {
	origFactory := runtimeFactory
	origRunInteractive := clikitRunInteractiveShell
	t.Cleanup(func() {
		runtimeFactory = origFactory
		clikitRunInteractiveShell = origRunInteractive
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{}, nil
	}
	called := false
	clikitRunInteractiveShell = func(ctx context.Context, in io.ReadCloser, out, errOut io.Writer, eng replEngine, timeoutMs int, verbose bool, waterfallMode string, initialSessionID string) error {
		called = true
		return nil
	}

	if err := run([]string{"--repl"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !called {
		t.Fatalf("expected clikit repl to be called")
	}
}

func TestCLIReplUsesInteractiveShell(t *testing.T) {
	origFactory := runtimeFactory
	origRunInteractive := clikitRunInteractiveShell
	t.Cleanup(func() {
		runtimeFactory = origFactory
		clikitRunInteractiveShell = origRunInteractive
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{}, nil
	}

	called := false
	clikitRunInteractiveShell = func(ctx context.Context, in io.ReadCloser, out, errOut io.Writer, eng replEngine, timeoutMs int, verbose bool, waterfallMode string, initialSessionID string) error {
		called = true
		return nil
	}

	if err := run([]string{"--repl"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !called {
		t.Fatalf("expected interactive shell to be called")
	}
}

func TestCLIReplPropagatesInteractiveShellError(t *testing.T) {
	origFactory := runtimeFactory
	origRunInteractive := clikitRunInteractiveShell
	t.Cleanup(func() {
		runtimeFactory = origFactory
		clikitRunInteractiveShell = origRunInteractive
	})
	runtimeFactory = func(context.Context, api.Options) (runtimeClient, error) {
		return &fakeRuntime{}, nil
	}

	want := errors.New("shell boom")
	clikitRunInteractiveShell = func(ctx context.Context, in io.ReadCloser, out, errOut io.Writer, eng replEngine, timeoutMs int, verbose bool, waterfallMode string, initialSessionID string) error {
		return want
	}

	err := run([]string{"--repl"}, io.Discard, io.Discard)
	if !errors.Is(err, want) {
		t.Fatalf("expected interactive shell error, got %v", err)
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

func TestRunBuildsGovmSandboxOptions(t *testing.T) {
	origFactory := runtimeFactory
	origGovmCheck := validateGovmRuntime
	t.Cleanup(func() {
		runtimeFactory = origFactory
		validateGovmRuntime = origGovmCheck
	})

	root := t.TempDir()
	validateGovmRuntime = func(api.GovmOptions) error { return nil }

	var captured api.Options
	runtimeFactory = func(_ context.Context, opts api.Options) (runtimeClient, error) {
		captured = opts
		return &fakeRuntime{
			runFn: func(_ context.Context, req api.Request) (*api.Response, error) {
				if req.SessionID == "" {
					t.Fatal("expected generated session id")
				}
				return &api.Response{Mode: api.ModeContext{EntryPoint: api.EntryPointCLI}, Result: &api.Result{Output: "ok"}}, nil
			},
		}, nil
	}

	if err := run([]string{"--project", root, "--prompt", "hi", "--sandbox-backend=govm"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}

	if captured.Sandbox.Type != "govm" {
		t.Fatalf("sandbox type = %q", captured.Sandbox.Type)
	}
	if captured.Sandbox.Govm == nil || !captured.Sandbox.Govm.Enabled {
		t.Fatal("expected govm config enabled")
	}
	if captured.Sandbox.Govm.RuntimeHome != filepath.Join(root, ".govm") {
		t.Fatalf("runtime home = %q", captured.Sandbox.Govm.RuntimeHome)
	}
	if captured.Sandbox.Govm.OfflineImage != "py312-alpine" {
		t.Fatalf("offline image = %q", captured.Sandbox.Govm.OfflineImage)
	}
	if !captured.Sandbox.Govm.AutoCreateSessionWorkspace {
		t.Fatal("expected auto session workspace enabled")
	}
	if captured.Sandbox.Govm.SessionWorkspaceBase != filepath.Join(root, "workspace") {
		t.Fatalf("workspace base = %q", captured.Sandbox.Govm.SessionWorkspaceBase)
	}
	if len(captured.Sandbox.Govm.Mounts) != 1 {
		t.Fatalf("expected one project mount, got %+v", captured.Sandbox.Govm.Mounts)
	}
	mount := captured.Sandbox.Govm.Mounts[0]
	if mount.HostPath != root || mount.GuestPath != "/project" || !mount.ReadOnly {
		t.Fatalf("unexpected project mount %+v", mount)
	}
}

func TestRunGovmProjectMountOff(t *testing.T) {
	origFactory := runtimeFactory
	origGovmCheck := validateGovmRuntime
	t.Cleanup(func() {
		runtimeFactory = origFactory
		validateGovmRuntime = origGovmCheck
	})

	validateGovmRuntime = func(api.GovmOptions) error { return nil }

	var captured api.Options
	runtimeFactory = func(_ context.Context, opts api.Options) (runtimeClient, error) {
		captured = opts
		return &fakeRuntime{
			runFn: func(_ context.Context, req api.Request) (*api.Response, error) {
				return &api.Response{Mode: api.ModeContext{EntryPoint: api.EntryPointCLI}, Result: &api.Result{Output: "ok"}}, nil
			},
		}, nil
	}

	if err := run([]string{"--project", t.TempDir(), "--prompt", "hi", "--sandbox-backend=govm", "--sandbox-project-mount=off"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if captured.Sandbox.Govm == nil {
		t.Fatal("expected govm config")
	}
	if len(captured.Sandbox.Govm.Mounts) != 0 {
		t.Fatalf("expected no project mounts, got %+v", captured.Sandbox.Govm.Mounts)
	}
}

func TestRunRejectsInvalidSandboxProjectMount(t *testing.T) {
	err := run([]string{"--prompt", "hi", "--sandbox-backend=govm", "--sandbox-project-mount=invalid"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "sandbox-project-mount") {
		t.Fatalf("expected sandbox-project-mount error, got %v", err)
	}
}

func TestRunRejectsUnsupportedGovmPlatform(t *testing.T) {
	origPlatformCheck := validateGovmPlatform
	t.Cleanup(func() {
		validateGovmPlatform = origPlatformCheck
	})
	validateGovmPlatform = func() error {
		return errors.New("govm requires linux/amd64, linux/arm64, or darwin/arm64")
	}

	err := run([]string{"--prompt", "hi", "--sandbox-backend=govm"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "govm requires") {
		t.Fatalf("expected govm platform error, got %v", err)
	}
}

func TestRunRejectsUnavailableGovmRuntime(t *testing.T) {
	origPlatformCheck := validateGovmPlatform
	origGovmCheck := validateGovmRuntime
	t.Cleanup(func() {
		validateGovmPlatform = origPlatformCheck
		validateGovmRuntime = origGovmCheck
	})
	validateGovmPlatform = func() error { return nil }
	validateGovmRuntime = func(api.GovmOptions) error { return govmclient.ErrNativeUnavailable }

	err := run([]string{"--prompt", "hi", "--sandbox-backend=govm"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "govm native runtime unavailable") {
		t.Fatalf("expected govm runtime error, got %v", err)
	}
}

func TestBuildSandboxOptionsResolvesAbsolutePathsForGovm(t *testing.T) {
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	opts, err := buildSandboxOptions(".", "govm", "ro", "")
	if err != nil {
		t.Fatalf("build sandbox options: %v", err)
	}
	if opts.Govm == nil {
		t.Fatal("expected govm options")
	}
	if !filepath.IsAbs(opts.Govm.RuntimeHome) {
		t.Fatalf("expected absolute runtime home, got %q", opts.Govm.RuntimeHome)
	}
	if len(opts.Govm.Mounts) != 1 || !filepath.IsAbs(opts.Govm.Mounts[0].HostPath) {
		t.Fatalf("expected absolute host mount, got %+v", opts.Govm.Mounts)
	}
}
