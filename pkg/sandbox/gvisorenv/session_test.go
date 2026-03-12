package gvisorenv

import (
	"context"
	"path/filepath"
	"testing"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

func TestPrepareSessionUsesConfiguredMounts(t *testing.T) {
	host := t.TempDir()
	prepared, mapper, mounts, err := prepareSession(context.Background(), t.TempDir(), &sandboxenv.GVisorOptions{
		Enabled:         true,
		DefaultGuestCwd: "/workspace",
		Mounts: []sandboxenv.MountSpec{
			{HostPath: host, GuestPath: "/workspace/src", ReadOnly: true},
		},
	}, sandboxenv.SessionContext{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("prepare session: %v", err)
	}
	if prepared.GuestCwd != "/workspace" {
		t.Fatalf("guest cwd = %q", prepared.GuestCwd)
	}
	if len(mounts) != 1 || mounts[0].GuestPath != "/workspace/src" {
		t.Fatalf("unexpected mounts %+v", mounts)
	}
	got, mount, err := mapper.GuestToHost("/workspace/src/file.txt")
	if err != nil {
		t.Fatalf("guest to host: %v", err)
	}
	if want := filepath.Join(host, "file.txt"); got != want {
		t.Fatalf("mapped host path = %q, want %q", got, want)
	}
	if !mount.ReadOnly {
		t.Fatalf("expected readonly mount")
	}
}

func TestPrepareSessionCreatesDefaultWorkspaceWhenMountsEmpty(t *testing.T) {
	root := t.TempDir()
	prepared, mapper, mounts, err := prepareSession(context.Background(), root, &sandboxenv.GVisorOptions{
		Enabled:                    true,
		AutoCreateSessionWorkspace: true,
		SessionWorkspaceBase:       filepath.Join(root, "workspace"),
	}, sandboxenv.SessionContext{SessionID: "sess-2"})
	if err != nil {
		t.Fatalf("prepare session: %v", err)
	}
	if prepared.GuestCwd != "/workspace" {
		t.Fatalf("guest cwd = %q", prepared.GuestCwd)
	}
	if len(mounts) != 1 || mounts[0].GuestPath != "/workspace" || mounts[0].ReadOnly {
		t.Fatalf("unexpected default mounts %+v", mounts)
	}
	got, _, err := mapper.GuestToHost("/workspace/out.txt")
	if err != nil {
		t.Fatalf("guest to host: %v", err)
	}
	if want := filepath.Join(root, "workspace", "sess-2", "out.txt"); got != want {
		t.Fatalf("mapped host path = %q, want %q", got, want)
	}
}

func TestPrepareSessionUsesWorkspaceSessionID(t *testing.T) {
	root := t.TempDir()
	_, _, mounts, err := prepareSession(context.Background(), root, &sandboxenv.GVisorOptions{
		Enabled:                    true,
		AutoCreateSessionWorkspace: true,
		SessionWorkspaceBase:       filepath.Join(root, "workspace"),
	}, sandboxenv.SessionContext{SessionID: "session-xyz"})
	if err != nil {
		t.Fatalf("prepare session: %v", err)
	}
	if len(mounts) != 1 {
		t.Fatalf("unexpected mounts %+v", mounts)
	}
	if want := filepath.Join(root, "workspace", "session-xyz"); mounts[0].HostPath != want {
		t.Fatalf("host path = %q, want %q", mounts[0].HostPath, want)
	}
}
