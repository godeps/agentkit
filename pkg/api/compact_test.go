package api

import (
	"context"
	"errors"
	"testing"

	"github.com/godeps/agentkit/pkg/message"
	"github.com/godeps/agentkit/pkg/model"
)

type compactStubModel struct {
	resp string
	err  error
}

func (s *compactStubModel) Complete(ctx context.Context, req model.Request) (*model.Response, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &model.Response{Message: model.Message{Content: s.resp}}, nil
}

func (s *compactStubModel) CompleteStream(ctx context.Context, req model.Request, cb model.StreamHandler) error {
	resp, err := s.Complete(ctx, req)
	if err != nil {
		return err
	}
	return cb(model.StreamResult{Final: true, Response: resp})
}

func TestCompactConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := CompactConfig{Enabled: true, PreserveCount: 0, Threshold: 2, ContextLimit: -1, SummaryMaxTokens: -1}
	got := cfg.withDefaults()
	if got.PreserveCount < 1 {
		t.Fatalf("expected preserve count default")
	}
	if got.Threshold != defaultCompactThreshold {
		t.Fatalf("expected default threshold")
	}
	if got.ContextLimit != defaultContextLimit {
		t.Fatalf("expected default context limit")
	}
	if got.SummaryMaxTokens != defaultSummaryMaxTokens {
		t.Fatalf("expected default summary token budget")
	}
}

func TestCompactorMaybeCompact(t *testing.T) {
	t.Parallel()

	hist := message.NewHistory()
	hist.Append(msgWithTokens("user", 20))
	hist.Append(msgWithTokens("assistant", 20))
	hist.Append(msgWithTokens("user", 20))

	comp := newCompactor("", CompactConfig{Enabled: true, PreserveCount: 1, Threshold: 0.1}, &compactStubModel{resp: "summary"}, 1, nil)
	if comp == nil {
		t.Fatalf("expected compactor")
	}
	res, ok, err := comp.maybeCompact(context.Background(), hist, "sess", nil)
	if err != nil || !ok || res.summary == "" {
		t.Fatalf("unexpected compact result res=%+v ok=%v err=%v", res, ok, err)
	}
	if hist.Len() < 2 {
		t.Fatalf("expected compacted history retained")
	}
}

func TestCompactorCompleteSummaryError(t *testing.T) {
	t.Parallel()

	comp := &compactor{cfg: CompactConfig{Enabled: true}, model: &compactStubModel{err: errors.New("boom")}}
	if _, err := comp.completeSummary(context.Background(), model.Request{}); err == nil {
		t.Fatalf("expected summary error")
	}
}

func TestCompactorUsesConfiguredSummaryBudget(t *testing.T) {
	t.Parallel()

	hist := message.NewHistory()
	hist.Append(message.Message{Role: "user", Content: "one"})
	hist.Append(message.Message{Role: "assistant", Content: "two"})
	hist.Append(message.Message{Role: "user", Content: "three"})

	mdl := &summaryModel{content: "summary"}
	comp := newCompactor("", CompactConfig{
		Enabled:          true,
		PreserveCount:    1,
		Threshold:        0.1,
		ContextLimit:     50,
		SummaryMaxTokens: 2048,
	}, mdl, 0, nil)
	if comp == nil {
		t.Fatalf("expected compactor")
	}
	if _, err := comp.compact(context.Background(), hist, hist.All(), hist.TokenCount()); err != nil {
		t.Fatalf("expected compaction, got err=%v", err)
	}
	if len(mdl.calls) == 0 {
		t.Fatalf("expected summary model to be called")
	}
	if mdl.calls[0].MaxTokens != 2048 {
		t.Fatalf("expected summary max tokens 2048, got %d", mdl.calls[0].MaxTokens)
	}
	if comp.limit != 50 {
		t.Fatalf("expected context limit 50, got %d", comp.limit)
	}
}
