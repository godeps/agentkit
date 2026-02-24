package toolbuiltin

import "github.com/godeps/agentkit/pkg/tool"

var askUserQuestionSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"questions": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "The complete question to ask the user. Should be clear, specific, and end with a question mark.",
					},
					"header": map[string]interface{}{
						"type":        "string",
						"description": "Very short label displayed as a chip/tag (max 12 chars).",
					},
					"options": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"label": map[string]interface{}{
									"type":        "string",
									"description": "The display text for this option (1-5 words).",
								},
								"description": map[string]interface{}{
									"type":        "string",
									"description": "Explanation of what this option means.",
								},
							},
							"required":             []string{"label", "description"},
							"additionalProperties": false,
						},
						"minItems":    2,
						"maxItems":    4,
						"description": "2-4 available choices for this question.",
					},
					"multiSelect": map[string]interface{}{
						"type":        "boolean",
						"description": "Allow multiple options to be selected.",
					},
				},
				"required":             []string{"question", "header", "options", "multiSelect"},
				"additionalProperties": false,
			},
			"minItems":    1,
			"maxItems":    4,
			"description": "Questions to ask the user (1-4 questions)",
		},
		"answers": map[string]interface{}{
			"type": "object",
			"additionalProperties": map[string]interface{}{
				"type": "string",
			},
			"description": "User answers collected by the permission component",
		},
	},
	Required: []string{"questions"},
}
