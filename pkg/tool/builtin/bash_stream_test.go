package toolbuiltin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/security"
)

func TestBashToolStreamExecute(t *testing.T) {
	t.Parallel()

	tool := NewBashToolWithSandbox("", security.NewDisabledSandbox())
	tool.SetOutputThresholdBytes(1)

	var out []string
	res, err := tool.StreamExecute(context.Background(), map[string]interface{}{
		"command": "echo hello",
	}, func(chunk string, _ bool) {
		out = append(out, chunk)
	})
	if err != nil {
		t.Fatalf("stream execute failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if res.Output == "" {
		data, ok := res.Data.(map[string]interface{})
		if !ok {
			t.Fatalf("expected data map, got %T", res.Data)
		}
		if _, ok := data["output_file"]; !ok {
			t.Fatalf("expected output text or output_file reference")
		}
	}
}

func TestBashToolStreamExecuteErrors(t *testing.T) {
	t.Parallel()

	if _, err := (*BashTool)(nil).StreamExecute(context.Background(), nil, nil); err == nil {
		t.Fatalf("expected nil tool error")
	}
	if _, err := NewBashToolWithSandbox("", security.NewDisabledSandbox()).StreamExecute(nil, nil, nil); err == nil {
		t.Fatalf("expected nil context error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewBashToolWithSandbox("", security.NewDisabledSandbox()).StreamExecute(ctx, map[string]interface{}{
		"command": "printf 'hi'",
	}, nil)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled error, got %v", err)
	}
}

func TestBashToolStreamExecuteTimeoutDoesNotHangWithBackgroundChild(t *testing.T) {
	t.Parallel()

	tool := NewBashToolWithSandbox("", security.NewDisabledSandbox())

	started := time.Now()
	res, err := tool.StreamExecute(context.Background(), map[string]interface{}{
		"command": "sleep 6 & while true; do sleep 1; done",
		"timeout": 0.1,
	}, nil)
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "timeout") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if res == nil || res.Success {
		t.Fatalf("expected failed result, got %#v", res)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("stream execute drained too slowly after timeout: %s", elapsed)
	}
}

func TestBashToolStreamExecuteInvalidWorkdir(t *testing.T) {
	tool := NewBashToolWithSandbox("", security.NewDisabledSandbox())
	if _, err := tool.StreamExecute(context.Background(), map[string]interface{}{
		"command": "printf 'hi'",
		"workdir": "/path/does-not-exist",
	}, nil); err == nil {
		t.Fatalf("expected workdir error")
	}
}

func TestConsumeStreamReadError(t *testing.T) {
	reader := &errReadCloser{err: errors.New("read failed")}
	if err := consumeStream(context.Background(), reader, nil, nil, false); err == nil {
		t.Fatalf("expected read error")
	}
}

func TestBashToolStreamExecuteUsesStreamingEnvironmentForVirtualizedSandbox(t *testing.T) {
	t.Parallel()

	env := &fakeStreamingEnvironment{
		prepared: &sandboxenv.PreparedSession{
			SessionID:   "sess-1",
			GuestCwd:    "/workspace",
			SandboxType: "govm",
		},
		result: &sandboxenv.CommandResult{
			Stdout:   "hello",
			Stderr:   "warn",
			ExitCode: 0,
		},
	}
	tool := NewBashToolWithSandbox("", security.NewDisabledSandbox())
	tool.SetEnvironment(env)

	var got []string
	res, err := tool.StreamExecute(context.Background(), map[string]interface{}{
		"command": "echo hello",
	}, func(chunk string, isStderr bool) {
		got = append(got, fmt.Sprintf("%t:%s", isStderr, chunk))
	})
	if err != nil {
		t.Fatalf("stream execute failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if res.Output != "hello\nwarn" {
		t.Fatalf("unexpected output %q", res.Output)
	}
	if strings.Join(got, ",") != "false:hello,true:warn" {
		t.Fatalf("unexpected stream chunks: %v", got)
	}
}

type errReadCloser struct {
	err error
}

func (e *errReadCloser) Read([]byte) (int, error) { return 0, e.err }
func (e *errReadCloser) Close() error             { return nil }

type fakeStreamingEnvironment struct {
	prepared *sandboxenv.PreparedSession
	result   *sandboxenv.CommandResult
}

func (f *fakeStreamingEnvironment) PrepareSession(context.Context, sandboxenv.SessionContext) (*sandboxenv.PreparedSession, error) {
	return f.prepared, nil
}
func (f *fakeStreamingEnvironment) RunCommand(context.Context, *sandboxenv.PreparedSession, sandboxenv.CommandRequest) (*sandboxenv.CommandResult, error) {
	return f.result, nil
}
func (f *fakeStreamingEnvironment) RunCommandStream(_ context.Context, _ *sandboxenv.PreparedSession, _ sandboxenv.CommandRequest, cb sandboxenv.CommandStreamCallbacks) (*sandboxenv.CommandResult, error) {
	if cb.OnStdout != nil {
		cb.OnStdout("hello")
	}
	if cb.OnStderr != nil {
		cb.OnStderr("warn")
	}
	return f.result, nil
}
func (f *fakeStreamingEnvironment) ReadFile(context.Context, *sandboxenv.PreparedSession, string) ([]byte, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeStreamingEnvironment) WriteFile(context.Context, *sandboxenv.PreparedSession, string, []byte) error {
	return errors.New("not implemented")
}
func (f *fakeStreamingEnvironment) EditFile(context.Context, *sandboxenv.PreparedSession, sandboxenv.EditRequest) error {
	return errors.New("not implemented")
}
func (f *fakeStreamingEnvironment) Glob(context.Context, *sandboxenv.PreparedSession, string) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeStreamingEnvironment) Grep(context.Context, *sandboxenv.PreparedSession, sandboxenv.GrepRequest) ([]sandboxenv.GrepMatch, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeStreamingEnvironment) CloseSession(context.Context, *sandboxenv.PreparedSession) error {
	return nil
}
