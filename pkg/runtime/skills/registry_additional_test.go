package skills

import (
	"context"
	"testing"
)

func TestDefinitionValidateInvalidChar(t *testing.T) {
	def := Definition{Name: "Bad$Name"}
	if err := def.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid characters")
	}
}

func TestNormalizeDefinition(t *testing.T) {
	matcher := MatcherFunc(func(ActivationContext) MatchResult { return MatchResult{Matched: true} })
	meta := map[string]string{"key": "value"}
	def := Definition{
		Name:        "  MIXED  ",
		Description: "desc",
		Priority:    -5,
		MutexKey:    " Key ",
		Metadata:    meta,
		Matchers:    []Matcher{matcher},
	}

	norm := normalizeDefinition(def)
	if norm.Name != "mixed" {
		t.Fatalf("expected lowercase trimmed name, got %q", norm.Name)
	}
	if norm.Priority != 0 {
		t.Fatalf("expected negative priority to be clamped to 0, got %d", norm.Priority)
	}
	if norm.MutexKey != "key" {
		t.Fatalf("expected mutex key trimmed and lowercased, got %q", norm.MutexKey)
	}
	if &norm.Matchers[0] == &def.Matchers[0] {
		t.Fatalf("expected matchers slice to be copied")
	}
	if norm.Metadata["key"] != "value" {
		t.Fatalf("expected metadata copy, got %v", norm.Metadata)
	}
}

func TestSkillHandlerAccessor(t *testing.T) {
	r := NewRegistry()
	handler := HandlerFunc(func(context.Context, ActivationContext) (Result, error) { return Result{Output: "ok"}, nil })
	if err := r.Register(Definition{Name: "demo"}, handler); err != nil {
		t.Fatalf("register: %v", err)
	}
	skill, ok := r.Get("demo")
	if !ok {
		t.Fatalf("expected skill lookup to succeed")
	}
	got := skill.Handler()
	if got == nil {
		t.Fatalf("handler accessor returned nil")
	}
	if res, err := got.Execute(context.Background(), ActivationContext{Prompt: "ok"}); err != nil || res.Output != "ok" {
		t.Fatalf("unexpected handler result: %v %#v", err, res)
	}

	var nilSkill *Skill
	if nilSkill.Handler() != nil {
		t.Fatalf("nil skill should return nil handler")
	}
}
