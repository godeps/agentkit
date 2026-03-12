package gvisorhelper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"github.com/godeps/agentkit/pkg/sandbox"
	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

func executeGVisorRequest(ctx context.Context, req Request) Response {
	start := time.Now()
	bundleDir, stateDir, cleanup, err := prepareBundle(req)
	if err != nil {
		return Response{ExitCode: -1, DurationMs: time.Since(start).Milliseconds(), Error: err.Error()}
	}
	debugDir := filepath.Join(bundleDir, "debug")
	_ = os.MkdirAll(debugDir, 0o755)
	defer func() {
		if err == nil {
			cleanup()
		}
	}()

	exe, err := os.Executable()
	if err != nil {
		return Response{ExitCode: -1, DurationMs: time.Since(start).Milliseconds(), Error: fmt.Sprintf("resolve executable: %v", err)}
	}

	containerID := sanitizeContainerID(req.SessionID)
	if containerID == "" {
		containerID = fmt.Sprintf("agentkit-%d", time.Now().UnixNano())
	}
	networkMode := strings.TrimSpace(req.Network)
	if networkMode == "" {
		networkMode = "none"
	}
	rootless := os.Geteuid() != 0

	args := []string{
		"--debug=true",
		"--debug-log=" + debugDir + string(os.PathSeparator),
		"--panic-log=" + filepath.Join(bundleDir, "panic.log"),
		"--ignore-cgroups",
		"--network=" + networkMode,
		"--root=" + stateDir,
		"run",
		"--bundle=" + bundleDir,
		containerID,
	}
	if platform := strings.TrimSpace(os.Getenv("AGENTKIT_GVISOR_PLATFORM")); platform != "" {
		args = append(args[:5], append([]string{"--platform=" + platform}, args[5:]...)...)
	}
	if rootless {
		args = append(args[:3], append([]string{"--TESTONLY-unsafe-nonroot", "--rootless"}, args[3:]...)...)
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(os.Environ(), envRunscMode+"=1")
	if rootless {
		cmd.Env = append(cmd.Env, "AGENTKIT_GVISOR_SKIP_USERNS_REEXEC=1")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
			UidMappings: []syscall.SysProcIDMap{
				{ContainerID: 0, HostID: os.Getuid(), Size: 1},
			},
			GidMappings: []syscall.SysProcIDMap{
				{ContainerID: 0, HostID: os.Getgid(), Size: 1},
			},
			Credential:                 &syscall.Credential{Uid: 0, Gid: 0},
			GidMappingsEnableSetgroups: false,
			Pdeathsig:                  syscall.SIGKILL,
			Setsid:                     true,
		}
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGKILL,
			Setsid:    true,
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	resp := Response{
		Success:    err == nil,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: time.Since(start).Milliseconds(),
	}
	if err == nil {
		return resp
	}
	resp.Error = err.Error()
	if debugTail := collectDebugTail(debugDir); strings.TrimSpace(debugTail) != "" {
		if strings.TrimSpace(resp.Stderr) != "" {
			resp.Stderr += "\n"
		}
		resp.Stderr += debugTail
	}
	if strings.TrimSpace(resp.Stderr) != "" {
		resp.Stderr += "\n"
	}
	resp.Stderr += "debug_bundle=" + bundleDir
	if exitErr, ok := err.(*exec.ExitError); ok {
		resp.ExitCode = exitErr.ExitCode()
		return resp
	}
	resp.ExitCode = -1
	return resp
}

func prepareBundle(req Request) (bundleDir string, stateDir string, cleanup func(), err error) {
	bundleDir, err = os.MkdirTemp("", "agentkit-gvisor-bundle-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("create bundle dir: %w", err)
	}
	cleanupFuncs := make([]func(), 0, 4)
	cleanup = func() {
		for _, fn := range slices.Backward(cleanupFuncs) {
			fn()
		}
		_ = os.RemoveAll(bundleDir)
	}
	rootfs := filepath.Join(bundleDir, "rootfs")
	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("create rootfs dir: %w", err)
	}
	stateDir = filepath.Join(bundleDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("create state dir: %w", err)
	}
	stagedMounts, stagedCleanup, err := stageMountSources(bundleDir, req.Mounts)
	if err != nil {
		cleanup()
		return "", "", nil, err
	}
	cleanupFuncs = append(cleanupFuncs, stagedCleanup)
	req.Mounts = stagedMounts
	spec, err := buildBundleSpec(bundleDir, req)
	if err != nil {
		cleanup()
		return "", "", nil, err
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("marshal bundle spec: %w", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "config.json"), data, 0o644); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("write config.json: %w", err)
	}
	return bundleDir, stateDir, cleanup, nil
}

func stageMountSources(bundleDir string, mounts []sandboxenv.MountSpec) ([]sandboxenv.MountSpec, func(), error) {
	if len(mounts) == 0 {
		return nil, func() {}, nil
	}
	stageRoot := filepath.Join(bundleDir, "hostmounts")
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create mount staging dir: %w", err)
	}
	staged := make([]sandboxenv.MountSpec, 0, len(mounts))
	stagedPaths := make([]string, 0, len(mounts))
	for i, mount := range mounts {
		source := filepath.Clean(mount.HostPath)
		info, err := os.Stat(source)
		if err != nil {
			return nil, nil, fmt.Errorf("stat mount source %q: %w", source, err)
		}
		stagePath := filepath.Join(stageRoot, fmt.Sprintf("%02d", i))
		switch {
		case info.IsDir():
			if err := os.MkdirAll(stagePath, 0o755); err != nil {
				return nil, nil, fmt.Errorf("create staged mount dir %q: %w", stagePath, err)
			}
		default:
			parent := filepath.Dir(stagePath)
			if err := os.MkdirAll(parent, 0o755); err != nil {
				return nil, nil, fmt.Errorf("create staged mount parent %q: %w", parent, err)
			}
			f, err := os.OpenFile(stagePath, os.O_CREATE, 0o644)
			if err != nil {
				return nil, nil, fmt.Errorf("create staged mount file %q: %w", stagePath, err)
			}
			_ = f.Close()
		}
		if err := unix.Mount(source, stagePath, "", uintptr(unix.MS_BIND|unix.MS_REC), ""); err != nil {
			return nil, nil, fmt.Errorf("bind mount %q -> %q: %w", source, stagePath, err)
		}
		stagedPaths = append(stagedPaths, stagePath)
		mount.HostPath = stagePath
		staged = append(staged, mount)
	}
	cleanup := func() {
		for i := len(stagedPaths) - 1; i >= 0; i-- {
			_ = unix.Unmount(stagedPaths[i], unix.MNT_DETACH)
		}
	}
	return staged, cleanup, nil
}

func buildBundleSpec(bundleDir string, req Request) (*specs.Spec, error) {
	guestCwd := strings.TrimSpace(req.GuestCwd)
	if guestCwd == "" {
		guestCwd = "/workspace"
	}
	if !filepath.IsAbs(guestCwd) {
		return nil, fmt.Errorf("guest cwd must be absolute: %s", guestCwd)
	}
	rootfs := filepath.Join(bundleDir, "rootfs")
	env := append([]string(nil), os.Environ()...)
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}

	spec := &specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Args: []string{"/bin/bash", "-lc", req.Command},
			Cwd:  filepath.Clean(guestCwd),
			Env:  env,
		},
		Root: &specs.Root{
			Path:     rootfs,
			Readonly: true,
		},
		Mounts: defaultRuntimeMounts(),
		Linux: &specs.Linux{
			Resources: buildLinuxResources(req.Limits),
		},
	}
	for _, mount := range req.Mounts {
		spec.Mounts = append(spec.Mounts, toOCIMount(mount))
	}
	return spec, nil
}

func defaultRuntimeMounts() []specs.Mount {
	mounts := []specs.Mount{
		{Destination: "/proc", Type: "proc", Source: "proc"},
		{Destination: "/tmp", Type: "tmpfs", Source: "tmpfs", Options: []string{"nosuid", "nodev", "mode=1777"}},
	}
	for _, item := range []sandboxenv.MountSpec{
		{HostPath: "/bin", GuestPath: "/bin", ReadOnly: true},
		{HostPath: "/sbin", GuestPath: "/sbin", ReadOnly: true},
		{HostPath: "/lib", GuestPath: "/lib", ReadOnly: true},
		{HostPath: "/lib64", GuestPath: "/lib64", ReadOnly: true},
		{HostPath: "/usr", GuestPath: "/usr", ReadOnly: true},
		{HostPath: "/etc", GuestPath: "/etc", ReadOnly: true},
		{HostPath: "/dev", GuestPath: "/dev", ReadOnly: false},
	} {
		if _, err := os.Stat(item.HostPath); err == nil {
			mounts = append(mounts, toOCIMount(item))
		}
	}
	return mounts
}

func toOCIMount(m sandboxenv.MountSpec) specs.Mount {
	opts := []string{"rbind"}
	if m.ReadOnly {
		opts = append(opts, "ro")
	} else {
		opts = append(opts, "rw")
	}
	return specs.Mount{
		Type:        "bind",
		Source:      m.HostPath,
		Destination: filepath.Clean(m.GuestPath),
		Options:     opts,
	}
}

func buildLinuxResources(limits sandbox.ResourceLimits) *specs.LinuxResources {
	res := &specs.LinuxResources{}
	if limits.MaxMemoryBytes > 0 {
		v := int64(limits.MaxMemoryBytes)
		res.Memory = &specs.LinuxMemory{Limit: &v}
	}
	if res.Memory == nil {
		return nil
	}
	return res
}

func sanitizeContainerID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range sessionID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

func collectDebugTail(debugDir string) string {
	files, err := filepath.Glob(filepath.Join(debugDir, "*.txt"))
	if err != nil || len(files) == 0 {
		return ""
	}
	latest := files[0]
	latestInfo, err := os.Stat(latest)
	if err != nil {
		return ""
	}
	for _, candidate := range files[1:] {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestInfo.ModTime()) {
			latest = candidate
			latestInfo = info
		}
	}
	data, err := os.ReadFile(latest)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n")
}
