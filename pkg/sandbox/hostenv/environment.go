package hostenv

import (
	"context"
	"errors"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

var errHostEnvNotImplemented = errors.New("sandbox hostenv: operation not implemented")

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
	return nil, errHostEnvNotImplemented
}

func (e *Environment) ReadFile(context.Context, *sandboxenv.PreparedSession, string) ([]byte, error) {
	return nil, errHostEnvNotImplemented
}

func (e *Environment) WriteFile(context.Context, *sandboxenv.PreparedSession, string, []byte) error {
	return errHostEnvNotImplemented
}

func (e *Environment) EditFile(context.Context, *sandboxenv.PreparedSession, sandboxenv.EditRequest) error {
	return errHostEnvNotImplemented
}

func (e *Environment) Glob(context.Context, *sandboxenv.PreparedSession, string) ([]string, error) {
	return nil, errHostEnvNotImplemented
}

func (e *Environment) Grep(context.Context, *sandboxenv.PreparedSession, sandboxenv.GrepRequest) ([]sandboxenv.GrepMatch, error) {
	return nil, errHostEnvNotImplemented
}

func (e *Environment) CloseSession(context.Context, *sandboxenv.PreparedSession) error {
	return nil
}
