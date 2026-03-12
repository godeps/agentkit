package gvisorenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/sandbox/pathmap"
)

func prepareSession(_ context.Context, projectRoot string, gv *sandboxenv.GVisorOptions, session sandboxenv.SessionContext) (*sandboxenv.PreparedSession, *pathmap.Mapper, []sandboxenv.MountSpec, error) {
	if gv == nil {
		return nil, nil, nil, fmt.Errorf("gvisorenv: missing gvisor config")
	}
	mounts := append([]sandboxenv.MountSpec(nil), gv.Mounts...)
	if len(mounts) == 0 && gv.AutoCreateSessionWorkspace {
		base := gv.SessionWorkspaceBase
		if base == "" {
			base = filepath.Join(projectRoot, "workspace")
		}
		hostPath := filepath.Join(base, session.SessionID)
		if err := os.MkdirAll(hostPath, 0o755); err != nil {
			return nil, nil, nil, fmt.Errorf("gvisorenv: create session workspace: %w", err)
		}
		mounts = append(mounts, sandboxenv.MountSpec{
			HostPath:        hostPath,
			GuestPath:       "/workspace",
			ReadOnly:        false,
			CreateIfMissing: true,
		})
	}
	mapper, err := pathmap.New(mounts)
	if err != nil {
		return nil, nil, nil, err
	}
	guestCwd := gv.DefaultGuestCwd
	if guestCwd == "" {
		guestCwd = "/workspace"
	}
	prepared := &sandboxenv.PreparedSession{
		SessionID:   session.SessionID,
		GuestCwd:    guestCwd,
		SandboxType: "gvisor",
		Meta: map[string]any{
			"project_root": projectRoot,
			"mount_count":  len(mounts),
			"mounts":       append([]sandboxenv.MountSpec(nil), mounts...),
			"path_mapper":  mapper,
		},
	}
	return prepared, mapper, mounts, nil
}
