package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sandboxenv "github.com/godeps/agentkit/pkg/sandbox/env"
	"github.com/godeps/agentkit/pkg/sandbox/govmenv"
)

type demoReport struct {
	SessionID         string
	Mounts            []sandboxenv.MountSpec
	Steps             []demoStep
	SharedHostPath    string
	WorkspaceHostPath string
}

type demoStep struct {
	Name    string
	Status  string
	Details string
}

func runDemo(ctx context.Context, cfg demoConfig) (*demoReport, error) {
	cfg = cfg.normalize()
	if cfg.ProjectRoot == "" {
		return nil, fmt.Errorf("project root is required")
	}
	if err := os.MkdirAll(filepath.Join(cfg.ProjectRoot, "testdata", "shared"), 0o755); err != nil {
		return nil, fmt.Errorf("ensure shared dir: %w", err)
	}

	opts := buildGovmOptions(cfg.ProjectRoot)
	env := govmenv.New(cfg.ProjectRoot, opts)
	ps, err := env.PrepareSession(ctx, sandboxenv.SessionContext{
		SessionID:   cfg.SessionID,
		ProjectRoot: cfg.ProjectRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare govm session: %w", err)
	}
	defer func() { _ = env.CloseSession(context.Background(), ps) }()

	report := &demoReport{
		SessionID:         cfg.SessionID,
		Mounts:            append([]sandboxenv.MountSpec(nil), opts.Mounts...),
		SharedHostPath:    filepath.Join(cfg.ProjectRoot, "testdata", "shared", "result.txt"),
		WorkspaceHostPath: filepath.Join(cfg.ProjectRoot, "workspace", cfg.SessionID, "session-note.txt"),
	}
	report.Mounts = append(report.Mounts, sandboxenv.MountSpec{
		HostPath:  filepath.Join(cfg.ProjectRoot, "workspace", cfg.SessionID),
		GuestPath: "/workspace",
		ReadOnly:  false,
	})

	if data, err := env.ReadFile(ctx, ps, "/inputs/policy.txt"); err != nil {
		return nil, fmt.Errorf("step 1 readonly read: %w", err)
	} else {
		report.Steps = append(report.Steps, demoStep{
			Name:    "READONLY_READ",
			Status:  "OK",
			Details: strings.TrimSpace(string(data)),
		})
	}

	if err := env.WriteFile(ctx, ps, "/inputs/blocked.txt", []byte("denied")); err == nil {
		return nil, fmt.Errorf("step 2 readonly write unexpectedly succeeded")
	} else {
		report.Steps = append(report.Steps, demoStep{
			Name:    "READONLY_WRITE",
			Status:  "EXPECTED_DENIED",
			Details: err.Error(),
		})
	}

	sharedContent := fmt.Sprintf("shared result for session %s", cfg.SessionID)
	if err := env.WriteFile(ctx, ps, "/shared/result.txt", []byte(sharedContent)); err != nil {
		return nil, fmt.Errorf("step 3 shared write: %w", err)
	}
	report.Steps = append(report.Steps, demoStep{
		Name:    "SHARED_WRITE",
		Status:  "OK",
		Details: "/shared/result.txt",
	})

	workspaceContent := fmt.Sprintf("workspace note for session %s", cfg.SessionID)
	if err := env.WriteFile(ctx, ps, "/workspace/session-note.txt", []byte(workspaceContent)); err != nil {
		return nil, fmt.Errorf("step 4 workspace write: %w", err)
	}
	report.Steps = append(report.Steps, demoStep{
		Name:    "WORKSPACE_WRITE",
		Status:  "OK",
		Details: "/workspace/session-note.txt",
	})

	sharedData, err := os.ReadFile(report.SharedHostPath)
	if err != nil {
		return nil, fmt.Errorf("step 5 host verify shared: %w", err)
	}
	workspaceData, err := os.ReadFile(report.WorkspaceHostPath)
	if err != nil {
		return nil, fmt.Errorf("step 5 host verify workspace: %w", err)
	}
	report.Steps = append(report.Steps, demoStep{
		Name:   "HOST_VERIFY",
		Status: "OK",
		Details: strings.Join([]string{
			strings.TrimSpace(string(sharedData)),
			strings.TrimSpace(string(workspaceData)),
		}, " | "),
	})
	return report, nil
}
