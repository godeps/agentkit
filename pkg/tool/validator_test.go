package tool

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidatorNestedObjectValidation(t *testing.T) {
	t.Parallel()

	v := DefaultValidator{}
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"user": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
					"age":  map[string]any{"type": "integer"},
				},
				"required": []string{"name"},
			},
		},
		Required: []string{"user"},
	}

	if err := v.Validate(map[string]any{
		"user": map[string]any{"name": "alice", "age": 30},
	}, schema); err != nil {
		t.Fatalf("expected nested object validation success: %v", err)
	}

	err := v.Validate(map[string]any{
		"user": map[string]any{"age": 30},
	}, schema)
	if err == nil || !strings.Contains(err.Error(), "missing required field: user.name") {
		t.Fatalf("expected missing nested required error, got %v", err)
	}
}

func TestValidatorArrayItemsValidation(t *testing.T) {
	t.Parallel()

	v := DefaultValidator{}
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"people": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"role": map[string]any{"type": "string", "enum": []string{"dev", "ops"}},
					},
					"required": []string{"name", "role"},
				},
			},
		},
		Required: []string{"people"},
	}

	if err := v.Validate(map[string]any{
		"people": []any{
			map[string]any{"name": "a", "role": "dev"},
			map[string]any{"name": "b", "role": "ops"},
		},
	}, schema); err != nil {
		t.Fatalf("expected array items validation success: %v", err)
	}

	err := v.Validate(map[string]any{
		"people": []any{
			map[string]any{"name": "a", "role": "dev"},
			map[string]any{"name": "b"},
		},
	}, schema)
	if err == nil || !strings.Contains(err.Error(), "missing required field: people[1].role") {
		t.Fatalf("expected missing required error for array item, got %v", err)
	}
}

func TestValidatorEnumValidation(t *testing.T) {
	t.Parallel()

	v := DefaultValidator{}
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"mode": map[string]any{"type": "string", "enum": []any{"a", "b"}},
			"lvl":  map[string]any{"type": "number", "enum": []any{1, 2}},
		},
		Required: []string{"mode", "lvl"},
	}

	if err := v.Validate(map[string]any{"mode": "a", "lvl": 1.0}, schema); err != nil {
		t.Fatalf("expected enum validation success: %v", err)
	}

	if err := v.Validate(map[string]any{"mode": "c", "lvl": 1.0}, schema); err == nil {
		t.Fatalf("expected enum validation failure for mode")
	}

	if err := v.Validate(map[string]any{"mode": "a", "lvl": json.Number("2")}, schema); err != nil {
		t.Fatalf("expected numeric enum validation success: %v", err)
	}
}

func TestValidatorPatternValidation(t *testing.T) {
	t.Parallel()

	v := DefaultValidator{}
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"code": map[string]any{"type": "string", "pattern": "^[a-z]{3}$"},
		},
		Required: []string{"code"},
	}

	if err := v.Validate(map[string]any{"code": "abc"}, schema); err != nil {
		t.Fatalf("expected pattern validation success: %v", err)
	}

	if err := v.Validate(map[string]any{"code": "ab1"}, schema); err == nil {
		t.Fatalf("expected pattern validation failure")
	}
}

func TestValidatorMinimumMaximumValidation(t *testing.T) {
	t.Parallel()

	v := DefaultValidator{}
	min := 0.0
	max := 10.0
	schema := &JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"count": &JSONSchema{Type: "integer", Minimum: &min, Maximum: &max},
		},
		Required: []string{"count"},
	}

	if err := v.Validate(map[string]any{"count": 0}, schema); err != nil {
		t.Fatalf("expected minimum/maximum success: %v", err)
	}

	if err := v.Validate(map[string]any{"count": 11}, schema); err == nil {
		t.Fatalf("expected maximum failure")
	}
	if err := v.Validate(map[string]any{"count": -1}, schema); err == nil {
		t.Fatalf("expected minimum failure")
	}
}
