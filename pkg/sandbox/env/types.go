package env

import (
	"context"
	"time"
)

// ExecutionEnvironment abstracts how commands and file operations are executed.
type ExecutionEnvironment interface {
	PrepareSession(context.Context, SessionContext) (*PreparedSession, error)
	RunCommand(context.Context, *PreparedSession, CommandRequest) (*CommandResult, error)
	ReadFile(context.Context, *PreparedSession, string) ([]byte, error)
	WriteFile(context.Context, *PreparedSession, string, []byte) error
	EditFile(context.Context, *PreparedSession, EditRequest) error
	Glob(context.Context, *PreparedSession, string) ([]string, error)
	Grep(context.Context, *PreparedSession, GrepRequest) ([]GrepMatch, error)
	CloseSession(context.Context, *PreparedSession) error
}

// SessionContext identifies one logical runtime session.
type SessionContext struct {
	SessionID   string
	ProjectRoot string
}

// PreparedSession stores environment-specific execution state.
type PreparedSession struct {
	SessionID   string
	GuestCwd    string
	SandboxType string
	Meta        map[string]any
}

// CommandRequest describes one shell execution request.
type CommandRequest struct {
	Command string
	Workdir string
	Timeout time.Duration
	Env     map[string]string
}

// CommandResult captures the command output and status.
type CommandResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	Duration   time.Duration
	OutputFile string
}

// EditRequest captures one edit operation.
type EditRequest struct {
	Path       string
	OldText    string
	NewText    string
	ReplaceAll bool
}

// GrepRequest captures one text search request.
type GrepRequest struct {
	Pattern       string
	Path          string
	Literal       bool
	CaseSensitive bool
}

// GrepMatch is one grep result in environment-native path space.
type GrepMatch struct {
	Path    string
	Line    int
	Column  int
	Preview string
}

// GVisorOptions configures the gVisor-backed sandbox mode.
type GVisorOptions struct {
	Enabled                    bool
	DefaultGuestCwd            string
	AutoCreateSessionWorkspace bool
	SessionWorkspaceBase       string
	HelperModeFlag             string
	Mounts                     []MountSpec
}

// MountSpec describes one host-to-guest filesystem exposure.
type MountSpec struct {
	HostPath        string `json:"host_path"`
	GuestPath       string `json:"guest_path"`
	ReadOnly        bool   `json:"read_only"`
	CreateIfMissing bool   `json:"create_if_missing"`
}
