package gvisorenv

import (
	"context"
	"errors"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

var errGVisorNotImplemented = errors.New("sandbox gvisorenv: operation not implemented")

// Environment is the gVisor-backed execution environment placeholder.
type Environment struct {
	projectRoot string
}

func New(projectRoot string) *Environment {
	return &Environment{projectRoot: projectRoot}
}

func (e *Environment) PrepareSession(_ context.Context, session sandboxenv.SessionContext) (*sandboxenv.PreparedSession, error) {
	return &sandboxenv.PreparedSession{
		SessionID:   session.SessionID,
		GuestCwd:    "/workspace",
		SandboxType: "gvisor",
		Meta: map[string]any{
			"project_root": e.projectRoot,
		},
	}, nil
}

func (e *Environment) RunCommand(context.Context, *sandboxenv.PreparedSession, sandboxenv.CommandRequest) (*sandboxenv.CommandResult, error) {
	return nil, errGVisorNotImplemented
}

func (e *Environment) ReadFile(context.Context, *sandboxenv.PreparedSession, string) ([]byte, error) {
	return nil, errGVisorNotImplemented
}

func (e *Environment) WriteFile(context.Context, *sandboxenv.PreparedSession, string, []byte) error {
	return errGVisorNotImplemented
}

func (e *Environment) EditFile(context.Context, *sandboxenv.PreparedSession, sandboxenv.EditRequest) error {
	return errGVisorNotImplemented
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
