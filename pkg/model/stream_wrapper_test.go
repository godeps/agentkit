package model

import (
	"context"
	"errors"
	"testing"
)

// fakeModel is a test double that records which methods were called.
type fakeModel struct {
	completeResp   *Response
	completeErr    error
	streamResults  []StreamResult
	streamErr      error
	completeCalled bool
	streamCalled   bool
}

func (f *fakeModel) Complete(_ context.Context, _ Request) (*Response, error) {
	f.completeCalled = true
	return f.completeResp, f.completeErr
}

func (f *fakeModel) CompleteStream(_ context.Context, _ Request, cb StreamHandler) error {
	f.streamCalled = true
	for _, sr := range f.streamResults {
		if err := cb(sr); err != nil {
			return err
		}
	}
	return f.streamErr
}

func TestStreamOnlyModel_Complete_UsesStream(t *testing.T) {
	want := &Response{
		Message:    Message{Role: "assistant", Content: "hello"},
		StopReason: "end_turn",
	}
	inner := &fakeModel{
		streamResults: []StreamResult{
			{Delta: "hel"},
			{Delta: "lo"},
			{Final: true, Response: want},
		},
	}
	wrapper := NewStreamOnlyModel(inner)

	got, err := wrapper.Complete(context.Background(), Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inner.streamCalled {
		t.Fatal("expected CompleteStream to be called")
	}
	if inner.completeCalled {
		t.Fatal("Complete should NOT be called on inner model")
	}
	if got.Message.Content != want.Message.Content {
		t.Fatalf("content mismatch: got %q, want %q", got.Message.Content, want.Message.Content)
	}
	if got.StopReason != want.StopReason {
		t.Fatalf("stop_reason mismatch: got %q, want %q", got.StopReason, want.StopReason)
	}
}

func TestStreamOnlyModel_Complete_ToolCalls(t *testing.T) {
	tc := ToolCall{ID: "t1", Name: "Bash", Arguments: map[string]any{"command": "ls"}}
	want := &Response{
		Message: Message{
			Role:      "assistant",
			Content:   "Let me list files.",
			ToolCalls: []ToolCall{tc},
		},
		StopReason: "tool_use",
	}
	inner := &fakeModel{
		streamResults: []StreamResult{
			{Delta: "Let me list files."},
			{ToolCall: &tc},
			{Final: true, Response: want},
		},
	}
	wrapper := NewStreamOnlyModel(inner)

	got, err := wrapper.Complete(context.Background(), Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(got.Message.ToolCalls))
	}
	if got.Message.ToolCalls[0].Arguments["command"] != "ls" {
		t.Fatalf("tool call args mismatch: %v", got.Message.ToolCalls[0].Arguments)
	}
}

func TestStreamOnlyModel_Complete_StreamError(t *testing.T) {
	inner := &fakeModel{
		streamErr: errors.New("stream failed"),
	}
	wrapper := NewStreamOnlyModel(inner)

	_, err := wrapper.Complete(context.Background(), Request{})
	if err == nil || err.Error() != "stream failed" {
		t.Fatalf("expected stream error, got: %v", err)
	}
}

func TestStreamOnlyModel_Complete_NoFinalResponse(t *testing.T) {
	inner := &fakeModel{
		streamResults: []StreamResult{
			{Delta: "partial"},
		},
	}
	wrapper := NewStreamOnlyModel(inner)

	_, err := wrapper.Complete(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected error for missing final response")
	}
}

func TestStreamOnlyModel_Complete_NilInner(t *testing.T) {
	wrapper := &StreamOnlyModel{Inner: nil}
	_, err := wrapper.Complete(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected error for nil inner model")
	}
}

func TestStreamOnlyModel_CompleteStream_Delegates(t *testing.T) {
	want := &Response{Message: Message{Role: "assistant", Content: "ok"}}
	inner := &fakeModel{
		streamResults: []StreamResult{
			{Final: true, Response: want},
		},
	}
	wrapper := NewStreamOnlyModel(inner)

	var got *Response
	err := wrapper.CompleteStream(context.Background(), Request{}, func(sr StreamResult) error {
		if sr.Final {
			got = sr.Response
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Message.Content != "ok" {
		t.Fatalf("expected response with content 'ok', got %+v", got)
	}
}
