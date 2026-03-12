package hostenv

import (
	"context"
	"path/filepath"
	"testing"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

func TestPrepareSessionUsesProjectRootAsGuestCwd(t *testing.T) {
	root := t.TempDir()
	env := New(root)
	prepared, err := env.PrepareSession(context.Background(), sandboxenv.SessionContext{SessionID: "sess"})
	if err != nil {
		t.Fatalf("prepare session: %v", err)
	}
	if prepared.GuestCwd != root {
		t.Fatalf("guest cwd = %q, want %q", prepared.GuestCwd, root)
	}
}

func TestReadAndWriteFile(t *testing.T) {
	root := t.TempDir()
	env := New(root)
	if err := env.WriteFile(context.Background(), nil, "nested/out.txt", []byte("hello")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	data, err := env.ReadFile(context.Background(), nil, filepath.Join(root, "nested/out.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("content = %q", string(data))
	}
}
