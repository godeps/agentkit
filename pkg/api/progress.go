package api

import (
	"context"
	"encoding/json"

	"github.com/godeps/agentkit/pkg/agent"
	"github.com/godeps/agentkit/pkg/middleware"
)

// streamEmitFunc is stored on context so tools can push incremental output
// without depending on middleware details.
type streamEmitFunc func(context.Context, StreamEvent)

// newProgressMiddleware surfaces Anthropic-compatible SSE progress events at
// each middleware interception point. The event ordering mirrors Anthropic's
// streaming payloads while adding agent/tool lifecycle markers.
func newProgressMiddleware(events chan<- StreamEvent) *progressMiddleware {
	return &progressMiddleware{emitter: progressEmitter{ch: events}}
}

// progressMiddleware centralises guarded writes to the event channel so the
// middleware hooks stay terse and ordered.
type progressMiddleware struct {
	emitter progressEmitter
}

func (p *progressMiddleware) Name() string { return "progress" }

func (p *progressMiddleware) emit(ctx context.Context, evt StreamEvent) {
	p.emitter.emit(ctx, evt)
}

func (p *progressMiddleware) streamEmit() streamEmitFunc {
	return p.emit
}

func (p *progressMiddleware) BeforeAgent(ctx context.Context, _ *middleware.State) error {
	p.emit(context.Background(), StreamEvent{Type: EventAgentStart})
	return nil
}

func (p *progressMiddleware) BeforeModel(ctx context.Context, st *middleware.State) error {
	iter := st.Iteration
	p.emit(ctx, StreamEvent{Type: EventIterationStart, Iteration: &iter})
	p.emit(ctx, StreamEvent{Type: EventMessageStart, Message: &Message{Role: "assistant"}})
	return nil
}

func (p *progressMiddleware) AfterModel(ctx context.Context, st *middleware.State) error {
	out, ok := st.ModelOutput.(*agent.ModelOutput)
	if !ok || out == nil {
		return nil
	}

	idx := 0
	text := out.Content
	p.textBlock(ctx, idx, text)
	if text != "" {
		idx++
	}

	for _, call := range out.ToolCalls {
		p.toolBlock(ctx, idx, call)
		idx++
	}

	reason := "end_turn"
	if len(out.ToolCalls) > 0 {
		reason = "tool_use"
	}
	p.emit(ctx, StreamEvent{Type: EventMessageDelta, Delta: &Delta{StopReason: reason}, Usage: &Usage{}})
	p.emit(ctx, StreamEvent{Type: EventMessageStop})
	return nil
}

func (p *progressMiddleware) BeforeTool(ctx context.Context, st *middleware.State) error {
	call, ok := st.ToolCall.(agent.ToolCall)
	if !ok {
		return nil
	}
	iter := st.Iteration
	p.emit(ctx, StreamEvent{Type: EventToolExecutionStart, ToolUseID: call.ID, Name: call.Name, Iteration: &iter})
	return nil
}

func (p *progressMiddleware) AfterTool(ctx context.Context, st *middleware.State) error {
	call, ok := st.ToolCall.(agent.ToolCall)
	if !ok {
		return nil
	}
	res, ok := st.ToolResult.(agent.ToolResult)
	if !ok {
		return nil
	}

	if res.Output != "" {
		p.emit(ctx, StreamEvent{Type: EventToolExecutionOutput, ToolUseID: call.ID, Name: call.Name, Output: res.Output})
	}

	payload := map[string]any{}
	if res.Output != "" {
		payload["output"] = res.Output
	}
	if len(res.Metadata) > 0 {
		payload["metadata"] = res.Metadata
	}
	p.emit(ctx, StreamEvent{Type: EventToolExecutionResult, ToolUseID: call.ID, Name: call.Name, Output: payload})
	return nil
}

func (p *progressMiddleware) AfterAgent(ctx context.Context, st *middleware.State) error {
	iter := st.Iteration
	p.emit(ctx, StreamEvent{Type: EventIterationStop, Iteration: &iter})
	p.emit(ctx, StreamEvent{Type: EventAgentStop})
	return nil
}

func (p *progressMiddleware) textBlock(ctx context.Context, idx int, content string) {
	if content == "" {
		return
	}
	p.emit(ctx, StreamEvent{Type: EventContentBlockStart, Index: &idx, ContentBlock: &ContentBlock{Type: "text"}})
	for _, r := range content {
		p.emit(ctx, StreamEvent{Type: EventContentBlockDelta, Index: &idx, Delta: &Delta{Type: "text_delta", Text: string(r)}})
	}
	p.emit(ctx, StreamEvent{Type: EventContentBlockStop, Index: &idx})
}

func (p *progressMiddleware) toolBlock(ctx context.Context, idx int, call agent.ToolCall) {
	p.emit(ctx, StreamEvent{Type: EventContentBlockStart, Index: &idx, ContentBlock: &ContentBlock{Type: "tool_use", ID: call.ID, Name: call.Name}})
	raw, err := json.Marshal(call.Input)
	if err != nil {
		raw = []byte("{}")
	}
	for _, chunk := range chunkString(string(raw), 10) {
		encoded, err := json.Marshal(chunk)
		if err != nil {
			encoded = []byte(`""`)
		}
		p.emit(ctx, StreamEvent{Type: EventContentBlockDelta, Index: &idx, Delta: &Delta{Type: "input_json_delta", PartialJSON: json.RawMessage(encoded)}})
	}
	p.emit(ctx, StreamEvent{Type: EventContentBlockStop, Index: &idx})
}

type progressEmitter struct {
	ch chan<- StreamEvent
}

func (e progressEmitter) emit(ctx context.Context, evt StreamEvent) {
	if e.ch == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// 阻塞发送保证事件不会静默丢失；在 context 取消时优雅返回。
	select {
	case <-ctx.Done():
		return
	case e.ch <- evt:
	}
}

// chunkString splits s into roughly equal sized pieces without dropping
// remainder characters to support streaming partial JSON/tool output.
func chunkString(s string, size int) []string {
	if size <= 0 || s == "" {
		return nil
	}
	out := make([]string, 0, (len(s)+size-1)/size)
	for start := 0; start < len(s); start += size {
		end := start + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[start:end])
	}
	return out
}
