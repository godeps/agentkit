package gvisorhelper

import (
	"github.com/godeps/agentkit/pkg/sandbox"
	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

// Request is the stdin protocol payload for helper mode.
type Request struct {
	Version   string                 `json:"version"`
	SessionID string                 `json:"session_id"`
	Command   string                 `json:"command"`
	GuestCwd  string                 `json:"guest_cwd"`
	TimeoutMs int64                  `json:"timeout_ms"`
	Env       map[string]string      `json:"env,omitempty"`
	Mounts    []sandboxenv.MountSpec `json:"mounts,omitempty"`
	Network   string                 `json:"network,omitempty"`
	Limits    sandbox.ResourceLimits `json:"limits"`
}

// Response is the stdout protocol payload for helper mode.
type Response struct {
	Success    bool   `json:"success"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}
