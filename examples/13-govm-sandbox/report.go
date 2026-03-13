package main

import (
	"fmt"
	"strings"
)

func (r *demoReport) Render() string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\n", r.SessionID)
	b.WriteString("Mounts:\n")
	for _, mount := range r.Mounts {
		mode := "rw"
		if mount.ReadOnly {
			mode = "ro"
		}
		fmt.Fprintf(&b, "- %s -> %s (%s)\n", mount.HostPath, mount.GuestPath, mode)
	}
	b.WriteString("Steps:\n")
	for i, step := range r.Steps {
		fmt.Fprintf(&b, "STEP %d %s: %s", i+1, step.Name, step.Status)
		if strings.TrimSpace(step.Details) != "" {
			fmt.Fprintf(&b, " [%s]", step.Details)
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "Shared host file: %s\n", r.SharedHostPath)
	fmt.Fprintf(&b, "Workspace host file: %s\n", r.WorkspaceHostPath)
	return b.String()
}
