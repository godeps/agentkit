package gvisorenv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
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

func (e *Environment) RunCommand(context.Context, *sandboxenv.PreparedSession, sandboxenv.CommandRequest) (*sandboxenv.CommandResult, error) {
	return nil, errGVisorNotImplemented
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

func (e *Environment) Glob(context.Context, *sandboxenv.PreparedSession, string) ([]string, error) {
	return nil, errGVisorNotImplemented
}

func (e *Environment) Grep(context.Context, *sandboxenv.PreparedSession, sandboxenv.GrepRequest) ([]sandboxenv.GrepMatch, error) {
	return nil, errGVisorNotImplemented
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
