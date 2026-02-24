package model

import (
	"context"
	"testing"
)

type recordingState struct {
	inputs  []any
	outputs []any
	values  map[string]any
}

func (s *recordingState) SetModelInput(v any)  { s.inputs = append(s.inputs, v) }
func (s *recordingState) SetModelOutput(v any) { s.outputs = append(s.outputs, v) }
func (s *recordingState) SetValue(key string, v any) {
	if s.values == nil {
		s.values = map[string]any{}
	}
	s.values[key] = v
}

func TestRecordModelRequestWithoutState(t *testing.T) {
	// Should be a no-op when context lacks middleware state.
	recordModelRequest(context.Background(), Request{Model: "claude"})
}

func TestRecordModelRequestAndResponse(t *testing.T) {
	st := &recordingState{}
	ctx := context.WithValue(context.Background(), MiddlewareStateKey, st)

	req := Request{Model: "claude-3"}
	recordModelRequest(ctx, req)
	if len(st.inputs) != 1 {
		t.Fatalf("request not recorded: %+v", st.inputs)
	}
	in, ok := st.inputs[0].(Request)
	if !ok || in.Model != req.Model {
		t.Fatalf("request payload mismatch: %+v", st.inputs[0])
	}

	resp := &Response{StopReason: "  end_turn  ", Usage: Usage{InputTokens: 10}}
	recordModelResponse(ctx, resp)
	if len(st.outputs) != 1 || st.outputs[0] != resp {
		t.Fatalf("response not recorded: %+v", st.outputs)
	}
	if st.values["model.response"] != resp || st.values["model.usage"] != resp.Usage {
		t.Fatalf("response metadata missing: %+v", st.values)
	}
	if st.values["model.stop_reason"] != "end_turn" {
		t.Fatalf("expected trimmed stop reason, got %+v", st.values["model.stop_reason"])
	}
}

func TestRecordModelResponseNil(t *testing.T) {
	st := &recordingState{}
	ctx := context.WithValue(context.Background(), MiddlewareStateKey, st)
	recordModelResponse(ctx, nil)
	if len(st.outputs) != 0 {
		t.Fatalf("expected no outputs recorded, got %d", len(st.outputs))
	}
}
