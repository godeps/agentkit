package tool

import (
	"context"
	"time"
)

// StreamingTool extends Tool with optional streaming support. Implementations
// should send incremental output through emit while still returning a final
// ToolResult for compatibility with Execute callers.
//
// Example usage when invoking a streaming tool:
//
//	streamSink := func(chunk string, isStderr bool) { fmt.Print(chunk) }
//	executor.Execute(ctx, tool.Call{
//		Name:       "bash",
//		Params:     map[string]any{"cmd": "echo hi"},
//		StreamSink: streamSink,
//	})
type StreamingTool interface {
	Tool

	// StreamExecute mirrors Execute but emits incremental chunks as they become
	// available. Emit MUST be safe for concurrent calls and MUST return
	// promptly to avoid blocking tool execution.
	StreamExecute(ctx context.Context, params map[string]interface{}, emit func(chunk string, isStderr bool)) (*ToolResult, error)
}

// StreamChunk represents a single streaming emission. This is an optional
// helper type to standardise event payloads.
type StreamChunk struct {
	Content   string
	IsStderr  bool
	Timestamp time.Time
}
