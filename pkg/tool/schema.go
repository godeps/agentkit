package tool

// JSONSchema captures the subset of JSON Schema we require for tool validation.
type JSONSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
	Enum       []interface{}          `json:"enum,omitempty"`
	Pattern    string                 `json:"pattern,omitempty"`
	Minimum    *float64               `json:"minimum,omitempty"`
	Maximum    *float64               `json:"maximum,omitempty"`
	Items      *JSONSchema            `json:"items,omitempty"`
}

// ToolSchema defines the structure for tool definitions passed to LLM.
// It follows the OpenAI/Anthropic tool schema format.
type ToolSchema struct {
	Type     string          `json:"type"` // Always "function"
	Function *FunctionSchema `json:"function"`
}

// FunctionSchema describes a callable tool function.
type FunctionSchema struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Parameters  *ParameterSchema `json:"parameters,omitempty"`
}

// ParameterSchema defines the input parameters for a tool.
type ParameterSchema struct {
	Type       string                     `json:"type"` // Always "object"
	Properties map[string]*PropertySchema `json:"properties"`
	Required   []string                   `json:"required,omitempty"`
}

// PropertySchema describes a single parameter property.
type PropertySchema struct {
	Type        string          `json:"type"`        // "string", "number", "boolean", "array", "object"
	Description string          `json:"description"` // Human-readable description
	Enum        []string        `json:"enum,omitempty"`
	Items       *PropertySchema `json:"items,omitempty"` // For array types
}
