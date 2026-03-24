package api

import (
	"testing"
	"time"

	coreevents "github.com/godeps/agentkit/pkg/core/events"
)

func TestTimelineBuildsUnifiedEntries(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	resp := &Response{
		HookEvents: []coreevents.Event{
			{Type: coreevents.EventType(TimelineEventModelRequest), Timestamp: now},
			{Type: coreevents.EventType(TimelineEventModelResponse), Timestamp: now.Add(time.Millisecond)},
			{Type: coreevents.PreToolUse, Timestamp: now.Add(2 * time.Millisecond)},
			{Type: coreevents.PostToolUse, Timestamp: now.Add(3 * time.Millisecond)},
			{Type: coreevents.EventType(TimelineEventMiddleware), Timestamp: now.Add(4 * time.Millisecond)},
			{Type: coreevents.EventType(TimelineEventPolicy), Timestamp: now.Add(5 * time.Millisecond)},
			{Type: coreevents.TokenUsage, Timestamp: now.Add(6 * time.Millisecond)},
			{Type: coreevents.EventType(TimelineEventResume), Timestamp: now.Add(7 * time.Millisecond)},
		},
		Result: &Result{
			Interrupted:  true,
			CheckpointID: "cp-1",
		},
	}

	timeline := BuildTimeline(resp)
	if len(timeline) == 0 {
		t.Fatal("expected timeline entries")
	}
	seen := map[string]bool{}
	for _, entry := range timeline {
		seen[entry.Kind] = true
	}
	for _, kind := range []string{
		TimelineKindModelRequest,
		TimelineKindModelResponse,
		TimelineKindToolCall,
		TimelineKindToolResult,
		TimelineKindMiddleware,
		TimelineKindPolicy,
		TimelineKindInterrupt,
		TimelineKindResume,
		TimelineKindTokenUsage,
	} {
		if !seen[kind] {
			t.Fatalf("missing timeline kind %q in %+v", kind, timeline)
		}
	}
}

func TestTimelineIncludesStreamEvents(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	resp := &Response{
		StreamEvents: []StreamEvent{
			{Type: EventAgentStart, Timestamp: now},
			{Type: EventIterationStart, Timestamp: now.Add(time.Millisecond)},
			{Type: EventToolExecutionStart, Timestamp: now.Add(2 * time.Millisecond), Name: "echo"},
			{Type: EventToolExecutionResult, Timestamp: now.Add(3 * time.Millisecond), Name: "echo"},
		},
	}

	timeline := BuildTimeline(resp)
	seen := map[string]bool{}
	for _, entry := range timeline {
		seen[entry.Kind] = true
	}
	for _, kind := range []string{TimelineKindAgent, TimelineKindIteration, TimelineKindToolCall, TimelineKindToolResult} {
		if !seen[kind] {
			t.Fatalf("missing timeline kind %q in %+v", kind, timeline)
		}
	}
}
