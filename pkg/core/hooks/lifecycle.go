package hooks

import (
	"context"

	"github.com/godeps/agentkit/pkg/core/events"
)

// Individual hook interfaces allow hook implementations to opt-in to only the
// callbacks they care about while keeping type safety.
type (
	PreToolUseHook interface {
		PreToolUse(context.Context, events.ToolUsePayload) error
	}
	PostToolUseHook interface {
		PostToolUse(context.Context, events.ToolResultPayload) error
	}
	UserPromptSubmitHook interface {
		UserPromptSubmit(context.Context, events.UserPromptPayload) error
	}
	StopHook interface {
		Stop(context.Context, events.StopPayload) error
	}
	NotificationHook interface {
		Notification(context.Context, events.NotificationPayload) error
	}
)

// AllHook represents a strongly typed hook object; individual methods remain
// optional via the narrow interfaces above.
type AllHook interface {
	PreToolUseHook
	PostToolUseHook
	UserPromptSubmitHook
	StopHook
	NotificationHook
}
