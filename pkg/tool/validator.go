package tool

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
)

// Validator validates tool parameters before execution.
type Validator interface {
	Validate(params map[string]interface{}, schema *JSONSchema) error
}

// DefaultValidator implements a small subset of JSON Schema validation for tool
// parameters (required fields, primitive types, nested objects/arrays, enum,
// pattern, minimum/maximum).
type DefaultValidator struct{}

// Validate ensures that params satisfy the provided schema.
func (v DefaultValidator) Validate(params map[string]interface{}, schema *JSONSchema) error {
	if schema == nil {
		return nil
	}

	if params == nil {
		params = map[string]interface{}{}
	}

	return v.validateValue(params, schema, "")
}

func (v DefaultValidator) validateValue(value any, schema *JSONSchema, path string) error {
	if schema == nil {
		return nil
	}

	expectedType := schema.Type
	if expectedType == "" {
		switch {
		case schema.Items != nil:
			expectedType = "array"
		case len(schema.Properties) > 0 || len(schema.Required) > 0:
			expectedType = "object"
		}
	}

	if expectedType != "" {
		if err := validateType(value, expectedType); err != nil {
			return wrapFieldError(path, err)
		}
	}

	if len(schema.Enum) > 0 && !valueInEnum(value, schema.Enum) {
		return wrapFieldError(path, fmt.Errorf("expected one of %v but got %v", schema.Enum, value))
	}

	if schema.Pattern != "" {
		str, ok := value.(string)
		if !ok {
			return wrapFieldError(path, fmt.Errorf("expected string but got %T", value))
		}
		re, err := regexp.Compile(schema.Pattern)
		if err != nil {
			return wrapFieldError(path, fmt.Errorf("invalid pattern %q: %w", schema.Pattern, err))
		}
		if !re.MatchString(str) {
			return wrapFieldError(path, fmt.Errorf("string %q does not match pattern %q", str, schema.Pattern))
		}
	}

	if schema.Minimum != nil || schema.Maximum != nil {
		num, ok := toFloat64(value)
		if !ok {
			return wrapFieldError(path, fmt.Errorf("expected number but got %T", value))
		}
		if schema.Minimum != nil && num < *schema.Minimum {
			return wrapFieldError(path, fmt.Errorf("value %v is less than minimum %v", num, *schema.Minimum))
		}
		if schema.Maximum != nil && num > *schema.Maximum {
			return wrapFieldError(path, fmt.Errorf("value %v exceeds maximum %v", num, *schema.Maximum))
		}
	}

	switch expectedType {
	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return wrapFieldError(path, fmt.Errorf("expected object but got %T", value))
		}
		for _, field := range schema.Required {
			if _, exists := obj[field]; !exists {
				return fmt.Errorf("missing required field: %s", joinPath(path, field))
			}
		}
		for key, child := range obj {
			propDef, ok := schema.Properties[key]
			if !ok {
				continue
			}
			propSchema, ok := schemaFromDefinition(propDef)
			if !ok {
				continue
			}
			if err := v.validateValue(child, propSchema, joinPath(path, key)); err != nil {
				return err
			}
		}
	case "array":
		items := schema.Items
		if items == nil {
			return nil
		}
		arr, ok := value.([]interface{})
		if !ok {
			return wrapFieldError(path, fmt.Errorf("expected array but got %T", value))
		}
		for idx, item := range arr {
			if err := v.validateValue(item, items, indexPath(path, idx)); err != nil {
				return err
			}
		}
	}

	return nil
}

func schemaFromDefinition(definition interface{}) (*JSONSchema, bool) {
	switch def := definition.(type) {
	case map[string]interface{}:
		return schemaFromMap(def), true
	case *JSONSchema:
		return def, true
	default:
		return nil, false
	}
}

func schemaFromMap(def map[string]interface{}) *JSONSchema {
	schema := &JSONSchema{}
	if value, ok := def["type"].(string); ok {
		schema.Type = value
	}
	if props, ok := def["properties"].(map[string]interface{}); ok {
		schema.Properties = props
	}
	schema.Required = extractStringSlice(def["required"])
	schema.Enum = extractEnum(def["enum"])
	if pattern, ok := def["pattern"].(string); ok {
		schema.Pattern = pattern
	}
	if min, ok := extractFloatPointer(def["minimum"]); ok {
		schema.Minimum = min
	}
	if max, ok := extractFloatPointer(def["maximum"]); ok {
		schema.Maximum = max
	}
	if itemsDef, ok := def["items"]; ok && itemsDef != nil {
		if itemsSchema, ok := schemaFromDefinition(itemsDef); ok {
			schema.Items = itemsSchema
		}
	}
	return schema
}

func extractStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, len(v))
		copy(out, v)
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func extractEnum(raw any) []interface{} {
	switch v := raw.(type) {
	case []interface{}:
		out := make([]interface{}, len(v))
		copy(out, v)
		return out
	case []string:
		out := make([]interface{}, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func extractFloatPointer(raw any) (*float64, bool) {
	if raw == nil {
		return nil, false
	}
	value, ok := toFloat64(raw)
	if !ok {
		return nil, false
	}
	return &value, true
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func valueInEnum(value any, values []interface{}) bool {
	for _, candidate := range values {
		if enumEqual(value, candidate) {
			return true
		}
	}
	return false
}

func enumEqual(a, b any) bool {
	if aNum, ok := toFloat64(a); ok {
		if bNum, ok := toFloat64(b); ok {
			return aNum == bNum
		}
	}
	return reflect.DeepEqual(a, b)
}

func joinPath(base, field string) string {
	if base == "" {
		return field
	}
	return base + "." + field
}

func indexPath(base string, idx int) string {
	if base == "" {
		return fmt.Sprintf("[%d]", idx)
	}
	return fmt.Sprintf("%s[%d]", base, idx)
}

func wrapFieldError(path string, err error) error {
	if path == "" {
		return err
	}
	return fmt.Errorf("field %s: %w", path, err)
}

func validateType(value interface{}, expected string) error {
	switch expected {
	case "string":
		if _, ok := value.(string); ok {
			return nil
		}
	case "number":
		if isNumber(value) {
			return nil
		}
	case "integer":
		if isInteger(value) {
			return nil
		}
	case "boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "object":
		if value == nil {
			break
		}
		if _, ok := value.(map[string]interface{}); ok {
			return nil
		}
	case "array":
		if _, ok := value.([]interface{}); ok {
			return nil
		}
	case "null":
		if value == nil {
			return nil
		}
	default:
		return fmt.Errorf("unsupported schema type %q", expected)
	}
	return fmt.Errorf("expected %s but got %T", expected, value)
}

func isNumber(value interface{}) bool {
	switch v := value.(type) {
	case float32, float64:
		return true
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case json.Number:
		_, err := v.Float64()
		return err == nil
	}
	return false
}

func isInteger(value interface{}) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return math.Trunc(float64(v)) == float64(v)
	case float64:
		return math.Trunc(v) == v
	case json.Number:
		_, err := v.Int64()
		return err == nil
	}
	return false
}
