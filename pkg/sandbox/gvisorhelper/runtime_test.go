package gvisorhelper

import (
	"path/filepath"
	"testing"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func TestBuildBundleSpecIncludesGuestCommandAndMounts(t *testing.T) {
	bundleDir := t.TempDir()
	spec, err := buildBundleSpec(bundleDir, Request{
		Version:   "v1",
		SessionID: "sess-1",
		Command:   "printf 'hello'",
		GuestCwd:  "/workspace",
		Env: map[string]string{
			"FOO": "bar",
		},
		Mounts: []sandboxenv.MountSpec{
			{HostPath: t.TempDir(), GuestPath: "/workspace", ReadOnly: false},
			{HostPath: t.TempDir(), GuestPath: "/input", ReadOnly: true},
		},
	})
	if err != nil {
		t.Fatalf("build bundle spec: %v", err)
	}
	if spec.Root == nil {
		t.Fatalf("expected root to be set")
	}
	if want := filepath.Join(bundleDir, "rootfs"); spec.Root.Path != want {
		t.Fatalf("root path = %q, want %q", spec.Root.Path, want)
	}
	if spec.Process == nil {
		t.Fatalf("expected process to be set")
	}
	if got, want := spec.Process.Cwd, "/workspace"; got != want {
		t.Fatalf("process cwd = %q, want %q", got, want)
	}
	if len(spec.Process.Args) != 3 || spec.Process.Args[0] != "/bin/bash" || spec.Process.Args[1] != "-lc" || spec.Process.Args[2] != "printf 'hello'" {
		t.Fatalf("unexpected process args: %#v", spec.Process.Args)
	}
	if !containsEnv(spec.Process.Env, "FOO=bar") {
		t.Fatalf("expected env to contain FOO=bar: %#v", spec.Process.Env)
	}
	if !hasMount(spec, "/workspace", false) {
		t.Fatalf("expected writable workspace mount in spec")
	}
	if !hasMount(spec, "/input", true) {
		t.Fatalf("expected readonly input mount in spec")
	}
	if !hasMountType(spec, "/proc", "proc") {
		t.Fatalf("expected proc mount in spec")
	}
}

func TestBuildBundleSpecRequiresAbsoluteGuestCwd(t *testing.T) {
	_, err := buildBundleSpec(t.TempDir(), Request{
		Command:  "true",
		GuestCwd: "workspace",
	})
	if err == nil {
		t.Fatalf("expected error for relative guest cwd")
	}
}

func hasMount(spec *specs.Spec, dst string, readOnly bool) bool {
	for _, mount := range spec.Mounts {
		if mount.Destination != dst {
			continue
		}
		hasRO := false
		for _, opt := range mount.Options {
			if opt == "ro" {
				hasRO = true
				break
			}
		}
		return hasRO == readOnly
	}
	return false
}

func hasMountType(spec *specs.Spec, dst, typ string) bool {
	for _, mount := range spec.Mounts {
		if mount.Destination == dst && mount.Type == typ {
			return true
		}
	}
	return false
}

func containsEnv(env []string, entry string) bool {
	for _, item := range env {
		if item == entry {
			return true
		}
	}
	return false
}
