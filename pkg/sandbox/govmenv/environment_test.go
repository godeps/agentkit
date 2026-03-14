package govmenv

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	govmclient "github.com/godeps/govm/pkg/client"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/sandbox/pathmap"
)

func TestEnvironmentRunCommandStream(t *testing.T) {
	root := t.TempDir()
	mapper, err := pathmap.New([]sandboxenv.MountSpec{
		{HostPath: filepath.Join(root, "workspace"), GuestPath: "/workspace", ReadOnly: false},
	})
	if err != nil {
		t.Fatalf("new mapper: %v", err)
	}

	env := &Environment{
		projectRoot: root,
		govm:        &sandboxenv.GovmOptions{Enabled: true},
		sessions: map[string]*sessionState{
			"sess-1": {
				prepared: &sandboxenv.PreparedSession{
					SessionID:   "sess-1",
					GuestCwd:    "/workspace",
					SandboxType: "govm",
					Meta: map[string]any{
						"path_mapper": mapper,
					},
				},
				box: &fakeGovmBox{
					result: &govmclient.ExecResult{
						ExitCode: 0,
						Stdout:   []string{"hello"},
						Stderr:   []string{"warn"},
					},
				},
			},
		},
	}

	var stdout []string
	var stderr []string
	res, err := env.RunCommandStream(context.Background(), env.sessions["sess-1"].prepared, sandboxenv.CommandRequest{
		Command: "echo hello",
		Timeout: time.Second,
	}, sandboxenv.CommandStreamCallbacks{
		OnStdout: func(line string) { stdout = append(stdout, line) },
		OnStderr: func(line string) { stderr = append(stderr, line) },
	})
	if err != nil {
		t.Fatalf("run command stream: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", res.ExitCode)
	}
	if strings.Join(stdout, ",") != "hello" {
		t.Fatalf("unexpected stdout callbacks: %v", stdout)
	}
	if strings.Join(stderr, ",") != "warn" {
		t.Fatalf("unexpected stderr callbacks: %v", stderr)
	}
	if res.Stdout != "hello" || res.Stderr != "warn" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

type fakeGovmBox struct {
	result *govmclient.ExecResult
}

func (f *fakeGovmBox) Start() error { return nil }
func (f *fakeGovmBox) Stop() error  { return nil }
func (f *fakeGovmBox) Close()       {}
func (f *fakeGovmBox) Exec(string, *govmclient.ExecOptions) (*govmclient.ExecResult, error) {
	return f.result, nil
}
func (f *fakeGovmBox) ExecStream(_ string, _ *govmclient.ExecOptions, cb govmclient.ExecStreamCallbacks) (*govmclient.ExecResult, error) {
	for _, line := range f.result.Stdout {
		if cb.OnStdout != nil {
			cb.OnStdout(line)
		}
	}
	for _, line := range f.result.Stderr {
		if cb.OnStderr != nil {
			cb.OnStderr(line)
		}
	}
	return f.result, nil
}
