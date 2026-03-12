package pathmap

import (
	"path/filepath"
	"reflect"
	"testing"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

func TestGuestToHostResolvesWritableMount(t *testing.T) {
	host := t.TempDir()
	mapper, err := New([]sandboxenv.MountSpec{
		{HostPath: host, GuestPath: "/workspace", ReadOnly: false},
	})
	if err != nil {
		t.Fatalf("new mapper: %v", err)
	}
	got, mount, err := mapper.GuestToHost("/workspace/out.txt")
	if err != nil {
		t.Fatalf("guest to host: %v", err)
	}
	if want := filepath.Join(host, "out.txt"); got != want {
		t.Fatalf("host path = %q, want %q", got, want)
	}
	if mount.ReadOnly {
		t.Fatalf("expected writable mount metadata")
	}
}

func TestGuestToHostRejectsUnmountedPath(t *testing.T) {
	mapper, err := New([]sandboxenv.MountSpec{
		{HostPath: t.TempDir(), GuestPath: "/workspace"},
	})
	if err != nil {
		t.Fatalf("new mapper: %v", err)
	}
	if _, _, err := mapper.GuestToHost("/other/file.txt"); err == nil {
		t.Fatal("expected unmapped guest path error")
	}
}

func TestGuestToHostPreservesReadOnlyMetadata(t *testing.T) {
	mapper, err := New([]sandboxenv.MountSpec{
		{HostPath: t.TempDir(), GuestPath: "/workspace", ReadOnly: true},
	})
	if err != nil {
		t.Fatalf("new mapper: %v", err)
	}
	_, mount, err := mapper.GuestToHost("/workspace/in.txt")
	if err != nil {
		t.Fatalf("guest to host: %v", err)
	}
	if !mount.ReadOnly {
		t.Fatalf("expected readonly mount metadata")
	}
}

func TestMountOverlapRejected(t *testing.T) {
	_, err := New([]sandboxenv.MountSpec{
		{HostPath: t.TempDir(), GuestPath: "/workspace"},
		{HostPath: t.TempDir(), GuestPath: "/workspace/out"},
	})
	if err == nil {
		t.Fatal("expected overlap error")
	}
}

func TestVisibleRoots(t *testing.T) {
	mapper, err := New([]sandboxenv.MountSpec{
		{HostPath: t.TempDir(), GuestPath: "/workspace"},
		{HostPath: t.TempDir(), GuestPath: "/output"},
	})
	if err != nil {
		t.Fatalf("new mapper: %v", err)
	}
	if got, want := mapper.VisibleRoots(), []string{"/workspace", "/output"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("visible roots = %v, want %v", got, want)
	}
}
