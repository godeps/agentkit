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
