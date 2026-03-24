package api

import (
	"sort"
	"time"

	coreevents "github.com/godeps/agentkit/pkg/core/events"
)

const (
	TimelineEventModelRequest  = "TimelineModelRequest"
	TimelineEventModelResponse = "TimelineModelResponse"
	TimelineEventPolicy        = "TimelinePolicyDecision"
	TimelineEventMiddleware    = "TimelineMiddlewareDecision"
	TimelineEventResume        = "TimelineResume"

	TimelineKindModelRequest  = "model_request"
	TimelineKindModelResponse = "model_response"
	TimelineKindToolCall      = "tool_call"
	TimelineKindToolResult    = "tool_result"
	TimelineKindMiddleware    = "middleware"
	TimelineKindPolicy        = "policy"
	TimelineKindInterrupt     = "interrupt"
	TimelineKindResume        = "resume"
	TimelineKindTokenUsage    = "token_usage"
)

type TimelineEntry struct {
	Kind      string    `json:"kind"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Source    string    `json:"source,omitempty"`
}

func BuildTimeline(resp *Response) []TimelineEntry {
	if resp == nil {
		return nil
	}
	out := make([]TimelineEntry, 0, len(resp.HookEvents)+2)
	for _, evt := range resp.HookEvents {
		if kind := timelineKindForEvent(evt.Type); kind != "" {
			out = append(out, TimelineEntry{Kind: kind, Timestamp: evt.Timestamp, Source: string(evt.Type)})
		}
	}
	if resp.Result != nil && resp.Result.Interrupted {
		out = append(out, TimelineEntry{Kind: TimelineKindInterrupt})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

func timelineKindForEvent(eventType coreevents.EventType) string {
	switch eventType {
	case coreevents.EventType(TimelineEventModelRequest):
		return TimelineKindModelRequest
	case coreevents.EventType(TimelineEventModelResponse):
		return TimelineKindModelResponse
	case coreevents.PreToolUse:
		return TimelineKindToolCall
	case coreevents.PostToolUse, coreevents.PostToolUseFailure:
		return TimelineKindToolResult
	case coreevents.EventType(TimelineEventMiddleware):
		return TimelineKindMiddleware
	case coreevents.EventType(TimelineEventPolicy):
		return TimelineKindPolicy
	case coreevents.EventType(TimelineEventResume):
		return TimelineKindResume
	case coreevents.TokenUsage:
		return TimelineKindTokenUsage
	default:
		return ""
	}
}
