package hostenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

// Environment is the host-native execution environment.
type Environment struct {
	projectRoot string
}

func New(projectRoot string) *Environment {
	return &Environment{projectRoot: projectRoot}
}

func (e *Environment) PrepareSession(_ context.Context, session sandboxenv.SessionContext) (*sandboxenv.PreparedSession, error) {
	cwd := e.projectRoot
	if cwd == "" {
		cwd = session.ProjectRoot
	}
	return &sandboxenv.PreparedSession{
		SessionID:   session.SessionID,
		GuestCwd:    cwd,
		SandboxType: "host",
	}, nil
}

func (e *Environment) RunCommand(context.Context, *sandboxenv.PreparedSession, sandboxenv.CommandRequest) (*sandboxenv.CommandResult, error) {
	return nil, errors.New("sandbox hostenv: command execution is not implemented")
}

func (e *Environment) ReadFile(_ context.Context, _ *sandboxenv.PreparedSession, path string) ([]byte, error) {
	return os.ReadFile(cleanHostPath(e.projectRoot, path))
}

func (e *Environment) WriteFile(_ context.Context, _ *sandboxenv.PreparedSession, path string, data []byte) error {
	path = cleanHostPath(e.projectRoot, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("hostenv: ensure directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o666); err != nil { //nolint:gosec // respect umask for created files
		return fmt.Errorf("hostenv: write file: %w", err)
	}
	return nil
}

func (e *Environment) EditFile(ctx context.Context, ps *sandboxenv.PreparedSession, req sandboxenv.EditRequest) error {
	path := cleanHostPath(e.projectRoot, req.Path)
	data, err := e.ReadFile(ctx, ps, path)
	if err != nil {
		return err
	}
	content := string(data)
	if req.ReplaceAll {
		content = strings.ReplaceAll(content, req.OldText, req.NewText)
	} else {
		content = strings.Replace(content, req.OldText, req.NewText, 1)
	}
	return e.WriteFile(ctx, ps, path, []byte(content))
}

func (e *Environment) Glob(_ context.Context, _ *sandboxenv.PreparedSession, pattern string) ([]string, error) {
	return filepath.Glob(cleanHostPath(e.projectRoot, pattern))
}

func (e *Environment) Grep(_ context.Context, _ *sandboxenv.PreparedSession, _ sandboxenv.GrepRequest) ([]sandboxenv.GrepMatch, error) {
	return nil, errors.New("sandbox hostenv: grep is not implemented")
}

func (e *Environment) CloseSession(_ context.Context, _ *sandboxenv.PreparedSession) error {
	return nil
}

func cleanHostPath(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, path))
}
