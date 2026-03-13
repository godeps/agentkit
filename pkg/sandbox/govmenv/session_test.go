package govmenv

import (
	"context"
	"path/filepath"
	"testing"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

func TestPrepareSessionAddsDefaultWorkspaceMount(t *testing.T) {
	root := t.TempDir()
	prepared, mapper, mounts, err := prepareSession(context.Background(), root, &sandboxenv.GovmOptions{
		Enabled:                    true,
		AutoCreateSessionWorkspace: true,
		SessionWorkspaceBase:       filepath.Join(root, "workspace"),
		DefaultGuestCwd:            "/workspace",
	}, sandboxenv.SessionContext{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("prepare session: %v", err)
	}
	if prepared.GuestCwd != "/workspace" {
		t.Fatalf("unexpected guest cwd %q", prepared.GuestCwd)
	}
	if len(mounts) != 1 || mounts[0].GuestPath != "/workspace" || mounts[0].ReadOnly {
		t.Fatalf("unexpected mounts %#v", mounts)
	}
	hostPath, _, err := mapper.GuestToHost("/workspace/out.txt")
	if err != nil {
		t.Fatalf("map guest path: %v", err)
	}
	want := filepath.Join(root, "workspace", "sess-1", "out.txt")
	if hostPath != want {
		t.Fatalf("unexpected host path %q want %q", hostPath, want)
	}
}

func TestPrepareSessionAppendsDefaultWorkspaceMountWhenCustomMountsExist(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "shared")
	prepared, mapper, mounts, err := prepareSession(context.Background(), root, &sandboxenv.GovmOptions{
		Enabled:                    true,
		AutoCreateSessionWorkspace: true,
		SessionWorkspaceBase:       filepath.Join(root, "workspace"),
		DefaultGuestCwd:            "/workspace",
		Mounts: []sandboxenv.MountSpec{
			{HostPath: shared, GuestPath: "/shared", ReadOnly: false, CreateIfMissing: true},
		},
	}, sandboxenv.SessionContext{SessionID: "sess-2"})
	if err != nil {
		t.Fatalf("prepare session: %v", err)
	}
	if prepared.GuestCwd != "/workspace" {
		t.Fatalf("unexpected guest cwd %q", prepared.GuestCwd)
	}
	if len(mounts) != 2 {
		t.Fatalf("unexpected mounts %#v", mounts)
	}
	hostPath, _, err := mapper.GuestToHost("/workspace/out.txt")
	if err != nil {
		t.Fatalf("map guest path: %v", err)
	}
	want := filepath.Join(root, "workspace", "sess-2", "out.txt")
	if hostPath != want {
		t.Fatalf("unexpected host path %q want %q", hostPath, want)
	}
}
