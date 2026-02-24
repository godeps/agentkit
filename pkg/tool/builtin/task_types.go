package toolbuiltin

import (
	"strings"

	"github.com/godeps/agentkit/pkg/runtime/tasks"
)

const (
	TaskStatusPending    = string(tasks.TaskPending)
	TaskStatusInProgress = string(tasks.TaskInProgress)
	TaskStatusCompleted  = string(tasks.TaskCompleted)
	TaskStatusBlocked    = string(tasks.TaskBlocked)
)

func normalizeTaskStatus(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return TaskStatusPending
	}
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	switch trimmed {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusCompleted, TaskStatusBlocked:
		return trimmed
	case "complete", "done":
		return TaskStatusCompleted
	default:
		return ""
	}
}
