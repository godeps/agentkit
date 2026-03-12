package gvisorenv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/sandbox/gvisorhelper"
	"github.com/godeps/agentkit/pkg/sandbox/pathmap"
)

var errGVisorNotImplemented = errors.New("sandbox gvisorenv: operation not implemented")

// Environment is the gVisor-backed execution environment placeholder.
type Environment struct {
	projectRoot string
	gvisor      *sandboxenv.GVisorOptions
}

func New(projectRoot string, opts *sandboxenv.GVisorOptions) *Environment {
	return &Environment{projectRoot: projectRoot, gvisor: opts}
}

func (e *Environment) PrepareSession(ctx context.Context, session sandboxenv.SessionContext) (*sandboxenv.PreparedSession, error) {
	prepared, _, _, err := prepareSession(ctx, e.projectRoot, e.gvisor, session)
	return prepared, err
}

func (e *Environment) RunCommand(ctx context.Context, ps *sandboxenv.PreparedSession, req sandboxenv.CommandRequest) (*sandboxenv.CommandResult, error) {
	mapper, err := mapperFromPreparedSession(ps)
	if err != nil {
		return nil, err
	}
	workdir := req.Workdir
	if strings.TrimSpace(workdir) == "" {
		workdir = ps.GuestCwd
	}
	hostCwd, _, err := mapper.GuestToHost(workdir)
	if err != nil {
		return nil, err
	}
	resp, err := gvisorhelper.Invoke(ctx, gvisorhelper.Request{
		Version:   "v1",
		SessionID: ps.SessionID,
		Command:   req.Command,
		GuestCwd:  hostCwd,
		TimeoutMs: req.Timeout.Milliseconds(),
		Env:       req.Env,
		Mounts:    mountsFromPreparedSession(ps),
		Network:   "none",
	}, e.helperFlag())
	if err != nil {
		return nil, err
	}
	result := &sandboxenv.CommandResult{
		Stdout:     resp.Stdout,
		Stderr:     resp.Stderr,
		ExitCode:   resp.ExitCode,
		Duration:   time.Duration(resp.DurationMs) * time.Millisecond,
		OutputFile: "",
	}
	if !resp.Success && resp.Error != "" {
		return result, fmt.Errorf("gvisorenv: command failed: %s", resp.Error)
	}
	return result, nil
}

func (e *Environment) helperFlag() string {
	if e == nil || e.gvisor == nil || strings.TrimSpace(e.gvisor.HelperModeFlag) == "" {
		return "--agentkit-gvisor-helper"
	}
	return e.gvisor.HelperModeFlag
}

func mountsFromPreparedSession(ps *sandboxenv.PreparedSession) []sandboxenv.MountSpec {
	if ps == nil || ps.Meta == nil {
		return nil
	}
	raw, ok := ps.Meta["mounts"]
	if !ok || raw == nil {
		return nil
	}
	mounts, ok := raw.([]sandboxenv.MountSpec)
	if !ok {
		return nil
	}
	return append([]sandboxenv.MountSpec(nil), mounts...)
}

func (e *Environment) ReadFile(_ context.Context, ps *sandboxenv.PreparedSession, path string) ([]byte, error) {
	mapper, err := mapperFromPreparedSession(ps)
	if err != nil {
		return nil, err
	}
	hostPath, _, err := mapper.GuestToHost(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(hostPath)
	if err != nil {
		return nil, fmt.Errorf("gvisorenv: read file: %w", err)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, fmt.Errorf("binary file %s is not supported", path)
	}
	return data, nil
}

func (e *Environment) WriteFile(_ context.Context, ps *sandboxenv.PreparedSession, path string, data []byte) error {
	mapper, err := mapperFromPreparedSession(ps)
	if err != nil {
		return err
	}
	hostPath, mount, err := mapper.GuestToHost(path)
	if err != nil {
		return err
	}
	if mount.ReadOnly {
		return fmt.Errorf("gvisorenv: guest path is read-only: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("gvisorenv: ensure directory: %w", err)
	}
	if err := os.WriteFile(hostPath, data, 0o666); err != nil { //nolint:gosec // respect umask for created files
		return fmt.Errorf("gvisorenv: write file: %w", err)
	}
	return nil
}

func (e *Environment) EditFile(ctx context.Context, ps *sandboxenv.PreparedSession, req sandboxenv.EditRequest) error {
	data, err := e.ReadFile(ctx, ps, req.Path)
	if err != nil {
		return err
	}
	content := string(data)
	if req.ReplaceAll {
		content = strings.ReplaceAll(content, req.OldText, req.NewText)
	} else {
		content = strings.Replace(content, req.OldText, req.NewText, 1)
	}
	return e.WriteFile(ctx, ps, req.Path, []byte(content))
}

func (e *Environment) Grep(ctx context.Context, ps *sandboxenv.PreparedSession, req sandboxenv.GrepRequest) ([]sandboxenv.GrepMatch, error) {
	mapper, err := mapperFromPreparedSession(ps)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(req.Pattern)
	if err != nil {
		return nil, fmt.Errorf("gvisorenv: compile grep pattern: %w", err)
	}
	var matches []sandboxenv.GrepMatch
	for _, root := range mapper.VisibleRoots() {
		if !withinGuestRoot(req.Path, root) {
			continue
		}
		hostRoot, mount, err := mapper.GuestToHost(root)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(hostRoot)
		if err != nil {
			return nil, fmt.Errorf("gvisorenv: stat grep root: %w", err)
		}
		if !info.IsDir() {
			continue
		}
		walkErr := filepath.Walk(hostRoot, func(hostPath string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			guestPath := filepath.Join(mount.GuestPath, strings.TrimPrefix(strings.TrimPrefix(hostPath, hostRoot), string(filepath.Separator)))
			if req.Path != "" && !withinGuestRoot(guestPath, req.Path) {
				return nil
			}
			data, err := os.ReadFile(hostPath)
			if err != nil {
				return fmt.Errorf("gvisorenv: read grep file: %w", err)
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if !re.MatchString(line) {
					continue
				}
				matches = append(matches, sandboxenv.GrepMatch{
					Path:    filepath.Clean(guestPath),
					Line:    i + 1,
					Column:  1,
					Preview: strings.TrimRight(line, "\r"),
				})
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return matches, nil
}

func (e *Environment) CloseSession(context.Context, *sandboxenv.PreparedSession) error {
	return nil
}

func mapperFromPreparedSession(ps *sandboxenv.PreparedSession) (*pathmap.Mapper, error) {
	if ps == nil || ps.Meta == nil {
		return nil, errors.New("gvisorenv: prepared session metadata is missing")
	}
	raw, ok := ps.Meta["path_mapper"]
	if !ok || raw == nil {
		return nil, errors.New("gvisorenv: path mapper is missing")
	}
	mapper, ok := raw.(*pathmap.Mapper)
	if !ok {
		return nil, errors.New("gvisorenv: invalid path mapper")
	}
	return mapper, nil
}

func (e *Environment) Glob(ctx context.Context, ps *sandboxenv.PreparedSession, pattern string) ([]string, error) {
	mapper, err := mapperFromPreparedSession(ps)
	if err != nil {
		return nil, err
	}
	pattern = filepath.Clean(pattern)
	var matches []string
	for _, root := range mapper.VisibleRoots() {
		hostRoot, mount, err := mapper.GuestToHost(root)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(hostRoot)
		if err != nil {
			return nil, fmt.Errorf("gvisorenv: stat glob root: %w", err)
		}
		if !info.IsDir() {
			continue
		}
		walkErr := filepath.Walk(hostRoot, func(hostPath string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			guestPath := mount.GuestPath
			if hostPath != hostRoot {
				guestPath = filepath.Join(mount.GuestPath, strings.TrimPrefix(strings.TrimPrefix(hostPath, hostRoot), string(filepath.Separator)))
			}
			guestPath = filepath.Clean(guestPath)
			ok, err := filepath.Match(pattern, guestPath)
			if err == nil && ok {
				matches = append(matches, guestPath)
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return matches, nil
}

func withinGuestRoot(path, root string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return true
	}
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}
