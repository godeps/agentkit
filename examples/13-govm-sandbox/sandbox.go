package main

import (
	"path/filepath"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
)

type demoConfig struct {
	ProjectRoot string
	SessionID   string
}

func (c demoConfig) normalize() demoConfig {
	cfg := c
	if cfg.SessionID == "" {
		cfg.SessionID = "govm-demo"
	}
	return cfg
}

func buildGovmOptions(projectRoot string) *sandboxenv.GovmOptions {
	return &sandboxenv.GovmOptions{
		Enabled:                    true,
		DefaultGuestCwd:            "/workspace",
		AutoCreateSessionWorkspace: true,
		SessionWorkspaceBase:       filepath.Join(projectRoot, "workspace"),
		RuntimeHome:                filepath.Join(projectRoot, ".govm"),
		OfflineImage:               "py312-alpine",
		Mounts: []sandboxenv.MountSpec{
			{
				HostPath:        filepath.Join(projectRoot, "testdata", "readonly"),
				GuestPath:       "/inputs",
				ReadOnly:        true,
				CreateIfMissing: true,
			},
			{
				HostPath:        filepath.Join(projectRoot, "testdata", "shared"),
				GuestPath:       "/shared",
				ReadOnly:        false,
				CreateIfMissing: true,
			},
		},
	}
}
