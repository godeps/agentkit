package govmenv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	govmclient "github.com/godeps/govm/pkg/client"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/sandbox/pathmap"
)

type sessionState struct {
	prepared *sandboxenv.PreparedSession
	mounts   []sandboxenv.MountSpec
	box      *govmclient.Box
	boxName  string
}

type Environment struct {
	projectRoot string
	govm        *sandboxenv.GovmOptions

	mu       sync.Mutex
	runtime  *govmclient.Runtime
	sessions map[string]*sessionState
}

func New(projectRoot string, opts *sandboxenv.GovmOptions) *Environment {
	return &Environment{
		projectRoot: projectRoot,
		govm:        opts,
		sessions:    make(map[string]*sessionState),
	}
}

func (e *Environment) PrepareSession(ctx context.Context, session sandboxenv.SessionContext) (*sandboxenv.PreparedSession, error) {
	e.mu.Lock()
	if existing, ok := e.sessions[session.SessionID]; ok {
		prepared := existing.prepared
		e.mu.Unlock()
		return prepared, nil
	}
	e.mu.Unlock()

	prepared, _, mounts, err := prepareSession(ctx, e.projectRoot, e.govm, session)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if existing, ok := e.sessions[session.SessionID]; ok {
		return existing.prepared, nil
	}
	e.sessions[session.SessionID] = &sessionState{
		prepared: prepared,
		mounts:   append([]sandboxenv.MountSpec(nil), mounts...),
	}
	return prepared, nil
}

func (e *Environment) RunCommand(ctx context.Context, ps *sandboxenv.PreparedSession, req sandboxenv.CommandRequest) (*sandboxenv.CommandResult, error) {
	state, err := e.ensureSessionBox(ctx, ps)
	if err != nil {
		return nil, err
	}
	workdir := req.Workdir
	if strings.TrimSpace(workdir) == "" {
		workdir = ps.GuestCwd
	}
	start := time.Now()
	res, err := state.box.Exec("/bin/sh", &govmclient.ExecOptions{
		Args:       []string{"-lc", req.Command},
		Env:        req.Env,
		Timeout:    req.Timeout,
		WorkingDir: workdir,
	})
	if err != nil {
		return nil, fmt.Errorf("govmenv: exec command: %w", err)
	}
	return &sandboxenv.CommandResult{
		Stdout:   strings.Join(res.Stdout, "\n"),
		Stderr:   strings.Join(res.Stderr, "\n"),
		ExitCode: res.ExitCode,
		Duration: time.Since(start),
	}, nil
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
		return nil, fmt.Errorf("govmenv: read file: %w", err)
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
		return fmt.Errorf("govmenv: guest path is read-only: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return fmt.Errorf("govmenv: ensure directory: %w", err)
	}
	if err := os.WriteFile(hostPath, data, 0o666); err != nil { //nolint:gosec
		return fmt.Errorf("govmenv: write file: %w", err)
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
			return nil, fmt.Errorf("govmenv: stat glob root: %w", err)
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

func (e *Environment) Grep(ctx context.Context, ps *sandboxenv.PreparedSession, req sandboxenv.GrepRequest) ([]sandboxenv.GrepMatch, error) {
	mapper, err := mapperFromPreparedSession(ps)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(req.Pattern)
	if err != nil {
		return nil, fmt.Errorf("govmenv: compile grep pattern: %w", err)
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
			return nil, fmt.Errorf("govmenv: stat grep root: %w", err)
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
				return fmt.Errorf("govmenv: read grep file: %w", err)
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

func (e *Environment) CloseSession(ctx context.Context, ps *sandboxenv.PreparedSession) error {
	if ps == nil {
		return nil
	}
	e.mu.Lock()
	state := e.sessions[ps.SessionID]
	delete(e.sessions, ps.SessionID)
	rt := e.runtime
	e.mu.Unlock()
	if state == nil {
		return nil
	}
	if state.box != nil {
		_ = state.box.Stop()
		state.box.Close()
	}
	if rt != nil && state.boxName != "" {
		if err := rt.RemoveBox(ctx, state.boxName, true); err != nil {
			return fmt.Errorf("govmenv: remove box: %w", err)
		}
	}
	return nil
}

func (e *Environment) createAndStartBox(ctx context.Context, sessionID string, ps *sandboxenv.PreparedSession, mounts []sandboxenv.MountSpec) (*govmclient.Box, string, error) {
	rt, err := e.ensureRuntime()
	if err != nil {
		return nil, "", err
	}
	boxName := fmt.Sprintf("agentkit-%s-%d", sanitizeName(sessionID), time.Now().UnixNano())
	boxOpts := govmclient.BoxOptions{
		Image:        e.govm.Image,
		OfflineImage: e.govm.OfflineImage,
		CPUs:         e.govm.CPUs,
		MemoryMB:     e.govm.MemoryMB,
		WorkingDir:   ps.GuestCwd,
		Network:      &govmclient.NetworkConfig{Enabled: false, Mode: govmclient.NetworkDisabled},
	}
	for _, mount := range mounts {
		boxOpts.Mounts = append(boxOpts.Mounts, govmclient.Mount{
			HostPath:  mount.HostPath,
			GuestPath: mount.GuestPath,
			ReadOnly:  mount.ReadOnly,
		})
	}
	box, err := rt.CreateBox(ctx, boxName, boxOpts)
	if err != nil {
		return nil, "", fmt.Errorf("govmenv: create box: %w", err)
	}
	if err := box.Start(); err != nil {
		box.Close()
		_ = rt.RemoveBox(ctx, boxName, true)
		return nil, "", fmt.Errorf("govmenv: start box: %w", err)
	}
	return box, boxName, nil
}

func (e *Environment) ensureRuntime() (*govmclient.Runtime, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.runtime != nil {
		return e.runtime, nil
	}
	if e.govm == nil {
		return nil, errors.New("govmenv: missing govm config")
	}
	if err := os.MkdirAll(e.govm.RuntimeHome, 0o755); err != nil {
		return nil, fmt.Errorf("govmenv: create runtime home: %w", err)
	}
	rt, err := govmclient.NewRuntime(&govmclient.RuntimeOptions{HomeDir: e.govm.RuntimeHome})
	if err != nil {
		return nil, fmt.Errorf("govmenv: new runtime: %w", err)
	}
	e.runtime = rt
	return rt, nil
}

func (e *Environment) session(ps *sandboxenv.PreparedSession) (*sessionState, error) {
	if ps == nil {
		return nil, errors.New("govmenv: missing prepared session")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	state := e.sessions[ps.SessionID]
	if state == nil {
		return nil, errors.New("govmenv: session is not initialized")
	}
	return state, nil
}

func (e *Environment) ensureSessionBox(ctx context.Context, ps *sandboxenv.PreparedSession) (*sessionState, error) {
	state, err := e.session(ps)
	if err != nil {
		return nil, err
	}
	if state.box != nil {
		return state, nil
	}

	box, boxName, err := e.createAndStartBox(ctx, ps.SessionID, state.prepared, state.mounts)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	current := e.sessions[ps.SessionID]
	if current == nil {
		_ = box.Stop()
		box.Close()
		if rt := e.runtime; rt != nil {
			_ = rt.RemoveBox(ctx, boxName, true)
		}
		return nil, errors.New("govmenv: session is not initialized")
	}
	if current.box == nil {
		current.box = box
		current.boxName = boxName
		return current, nil
	}

	_ = box.Stop()
	box.Close()
	if rt := e.runtime; rt != nil {
		_ = rt.RemoveBox(ctx, boxName, true)
	}
	return current, nil
}

func mapperFromPreparedSession(ps *sandboxenv.PreparedSession) (*pathmap.Mapper, error) {
	if ps == nil || ps.Meta == nil {
		return nil, errors.New("govmenv: prepared session metadata is missing")
	}
	raw, ok := ps.Meta["path_mapper"]
	if !ok || raw == nil {
		return nil, errors.New("govmenv: path mapper is missing")
	}
	mapper, ok := raw.(*pathmap.Mapper)
	if !ok {
		return nil, errors.New("govmenv: invalid path mapper")
	}
	return mapper, nil
}

func withinGuestRoot(path, root string) bool {
	if strings.TrimSpace(path) == "" {
		return true
	}
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	return path == root || strings.HasPrefix(path+string(filepath.Separator), root+string(filepath.Separator)) ||
		root == path || strings.HasPrefix(root+string(filepath.Separator), path+string(filepath.Separator))
}

func sanitizeName(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range in {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.Trim(b.String(), "-")
}
