package toolbuiltin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestAskUserQuestionSingleQuestionSingleSelect(t *testing.T) {
	tool := NewAskUserQuestionTool()
	params := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "Which database should we use?",
				"header":   "DB",
				"options": []interface{}{
					map[string]interface{}{"label": "Postgres", "description": "Use PostgreSQL for production"},
					map[string]interface{}{"label": "SQLite", "description": "Use SQLite for simplicity"},
				},
				"multiSelect": false,
			},
		},
	}

	res, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if res == nil || !res.Success {
		t.Fatalf("expected success result")
	}
	if !strings.Contains(res.Output, "[DB]") || !strings.Contains(res.Output, "Postgres") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
	data, ok := res.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected data type %T", res.Data)
	}
	qs, ok := data["questions"].([]Question)
	if !ok {
		t.Fatalf("unexpected questions type %T", data["questions"])
	}
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if qs[0].Header != "DB" || qs[0].MultiSelect {
		t.Fatalf("unexpected question: %+v", qs[0])
	}
	if len(qs[0].Options) != 2 || qs[0].Options[0].Label != "Postgres" {
		t.Fatalf("unexpected options: %+v", qs[0].Options)
	}
	if _, ok := data["answers"]; ok {
		t.Fatalf("did not expect answers in result data")
	}
}

func TestAskUserQuestionMultipleQuestions(t *testing.T) {
	tool := NewAskUserQuestionTool()
	params := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "Choose output format?",
				"header":   "Fmt",
				"options": []interface{}{
					map[string]interface{}{"label": "JSON", "description": "Machine readable output"},
					map[string]interface{}{"label": "Text", "description": "Human friendly output"},
				},
				"multiSelect": false,
			},
			map[string]interface{}{
				"question": "Enable caching?",
				"header":   "Cache",
				"options": []interface{}{
					map[string]interface{}{"label": "Yes", "description": "Cache results"},
					map[string]interface{}{"label": "No", "description": "Always recompute"},
				},
				"multiSelect": false,
			},
			map[string]interface{}{
				"question": "Pick deployment target?",
				"header":   "Deploy",
				"options": []interface{}{
					map[string]interface{}{"label": "Staging", "description": "Deploy to staging"},
					map[string]interface{}{"label": "Prod", "description": "Deploy to production"},
				},
				"multiSelect": false,
			},
		},
	}

	res, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	data := res.Data.(map[string]interface{})
	qs := data["questions"].([]Question)
	if len(qs) != 3 {
		t.Fatalf("expected 3 questions, got %d", len(qs))
	}
	if !strings.Contains(res.Output, "3 question(s)") || !strings.Contains(res.Output, "3.") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
}

func TestAskUserQuestionMultiSelect(t *testing.T) {
	tool := NewAskUserQuestionTool()
	params := map[string]interface{}{
		"questions": []interface{}{
			map[string]interface{}{
				"question": "Which platforms should we support?",
				"header":   "OS",
				"options": []interface{}{
					map[string]interface{}{"label": "Linux", "description": "Support Linux"},
					map[string]interface{}{"label": "macOS", "description": "Support macOS"},
					map[string]interface{}{"label": "Windows", "description": "Support Windows"},
				},
				"multiSelect": true,
			},
		},
	}

	res, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(res.Output, "multi-select") {
		t.Fatalf("expected output to include multi-select, got %q", res.Output)
	}
	data := res.Data.(map[string]interface{})
	qs := data["questions"].([]Question)
	if !qs[0].MultiSelect {
		t.Fatalf("expected MultiSelect true")
	}
}

func TestAskUserQuestionAcceptsTypedArraysAndAnswers(t *testing.T) {
	tool := NewAskUserQuestionTool()

	t.Run("answers map[string]interface{}", func(t *testing.T) {
		params := map[string]interface{}{
			"questions": []map[string]interface{}{
				{
					"question": "Pick one?",
					"header":   "Pick",
					"options": []map[string]interface{}{
						{"label": "A", "description": "Option A"},
						{"label": "B", "description": "Option B"},
					},
					"multiSelect": false,
				},
			},
			"answers": map[string]interface{}{"Pick": "A"},
		}
		res, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		data := res.Data.(map[string]interface{})
		answers, ok := data["answers"].(map[string]string)
		if !ok {
			t.Fatalf("unexpected answers type %T", data["answers"])
		}
		if answers["Pick"] != "A" {
			t.Fatalf("unexpected answers: %+v", answers)
		}
	})

	t.Run("answers map[string]string", func(t *testing.T) {
		params := map[string]interface{}{
			"questions": []interface{}{
				map[string]interface{}{
					"question": "Confirm?",
					"header":   "OK",
					"options": []interface{}{
						map[string]interface{}{"label": "Yes", "description": "Proceed"},
						map[string]interface{}{"label": "No", "description": "Stop"},
					},
					"multiSelect": false,
				},
			},
			"answers": map[string]string{"OK": "Yes"},
		}
		res, err := tool.Execute(context.Background(), params)
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		data := res.Data.(map[string]interface{})
		if _, ok := data["answers"].(map[string]string); !ok {
			t.Fatalf("unexpected answers type %T", data["answers"])
		}
	})
}

func TestAskUserQuestionConcurrentExecutions(t *testing.T) {
	tool := NewAskUserQuestionTool()

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			params := map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": fmt.Sprintf("Worker %d ok?", i),
						"header":   fmt.Sprintf("W%d", i),
						"options": []interface{}{
							map[string]interface{}{"label": "Yes", "description": "Proceed"},
							map[string]interface{}{"label": "No", "description": "Stop"},
						},
						"multiSelect": false,
					},
				},
			}
			if _, err := tool.Execute(context.Background(), params); err != nil {
				t.Errorf("worker %d execute error: %v", i, err)
			}
		}()
	}
	wg.Wait()
}

func TestAskUserQuestionMetadata(t *testing.T) {
	tool := NewAskUserQuestionTool()
	if tool.Name() != "AskUserQuestion" {
		t.Fatalf("unexpected name %q", tool.Name())
	}
	if tool.Description() != askUserQuestionDescription {
		t.Fatalf("unexpected description")
	}
	schema := tool.Schema()
	if schema == nil || schema.Type != "object" {
		t.Fatalf("unexpected schema: %#v", schema)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "questions" {
		t.Fatalf("unexpected schema required: %#v", schema.Required)
	}
	if _, ok := schema.Properties["questions"]; !ok {
		t.Fatalf("schema missing questions property")
	}
}

func TestAskUserQuestionErrors(t *testing.T) {
	tool := NewAskUserQuestionTool()

	cases := []struct {
		name   string
		ctx    context.Context
		params map[string]interface{}
		want   string
	}{
		{name: "nil context", ctx: nil, params: map[string]interface{}{}, want: "context is nil"},
		{name: "nil params", ctx: context.Background(), params: nil, want: "params is nil"},
		{name: "missing questions", ctx: context.Background(), params: map[string]interface{}{}, want: "questions is required"},
		{name: "questions not array", ctx: context.Background(), params: map[string]interface{}{"questions": "oops"}, want: "questions must be an array"},
		{name: "question entry not object", ctx: context.Background(), params: map[string]interface{}{"questions": []interface{}{"bad"}}, want: "questions[0] must be object"},
		{
			name: "question missing question field",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"header": "H",
						"options": []interface{}{
							map[string]interface{}{"label": "A", "description": "a"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
						"multiSelect": false,
					},
				},
			},
			want: "questions[0].question",
		},
		{
			name: "question missing header field",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one?",
						"options": []interface{}{
							map[string]interface{}{"label": "A", "description": "a"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
						"multiSelect": false,
					},
				},
			},
			want: "questions[0].header",
		},
		{
			name: "options missing",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question":    "Pick one?",
						"header":      "H",
						"multiSelect": false,
					},
				},
			},
			want: "questions[0].options",
		},
		{
			name: "options not array",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question":    "Pick one?",
						"header":      "H",
						"options":     "nope",
						"multiSelect": false,
					},
				},
			},
			want: "options must be an array",
		},
		{
			name: "option entry not object",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question":    "Pick one?",
						"header":      "H",
						"options":     []interface{}{"bad", map[string]interface{}{"label": "B", "description": "b"}},
						"multiSelect": false,
					},
				},
			},
			want: "options[0] must be object",
		},
		{
			name: "option missing label",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one?",
						"header":   "H",
						"options": []interface{}{
							map[string]interface{}{"description": "a"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
						"multiSelect": false,
					},
				},
			},
			want: "options[0].label",
		},
		{
			name: "option empty label",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one?",
						"header":   "H",
						"options": []interface{}{
							map[string]interface{}{"label": "   ", "description": "a"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
						"multiSelect": false,
					},
				},
			},
			want: "cannot be empty",
		},
		{
			name: "option missing description",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one?",
						"header":   "H",
						"options": []interface{}{
							map[string]interface{}{"label": "A"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
						"multiSelect": false,
					},
				},
			},
			want: "options[0].description",
		},
		{
			name: "multiSelect missing",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one?",
						"header":   "H",
						"options": []interface{}{
							map[string]interface{}{"label": "A", "description": "a"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
					},
				},
			},
			want: "multiSelect: field is required",
		},
		{
			name: "multiSelect wrong type",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one?",
						"header":   "H",
						"options": []interface{}{
							map[string]interface{}{"label": "A", "description": "a"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
						"multiSelect": "false",
					},
				},
			},
			want: "multiSelect must be boolean",
		},
		{
			name: "answers non-string value",
			ctx:  context.Background(),
			params: map[string]interface{}{
				"questions": []interface{}{
					map[string]interface{}{
						"question": "Pick one?",
						"header":   "H",
						"options": []interface{}{
							map[string]interface{}{"label": "A", "description": "a"},
							map[string]interface{}{"label": "B", "description": "b"},
						},
						"multiSelect": false,
					},
				},
				"answers": map[string]interface{}{
					"H": map[string]interface{}{"nested": true},
				},
			},
			want: "answers[\"H\"] must be string",
		},
		{name: "answers wrong type", ctx: context.Background(), params: map[string]interface{}{"questions": []interface{}{map[string]interface{}{
			"question": "Pick one?",
			"header":   "H",
			"options": []interface{}{
				map[string]interface{}{"label": "A", "description": "a"},
				map[string]interface{}{"label": "B", "description": "b"},
			},
			"multiSelect": false,
		}}, "answers": "oops"}, want: "answers must be object"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tool.Execute(tc.ctx, tc.params)
			if err == nil {
				t.Fatalf("expected error")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error mismatch: want %q got %v", tc.want, err)
			}
		})
	}
}
