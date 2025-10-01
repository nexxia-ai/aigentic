package ai

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// NewTool creates a Tool with auto-generated JSON schema from a typed function.
// The input parameter T must be a struct with json tags for schema generation.
//
// Example:
//
//	type CalculatorInput struct {
//	    Expression string `json:"expression" description:"Mathematical expression to evaluate"`
//	}
//
//	tool := ai.NewTool(
//	    "calculator",
//	    "Performs mathematical calculations",
//	    func(ctx context.Context, input CalculatorInput) (string, error) {
//	        return evaluateExpression(input.Expression), nil
//	    },
//	)
//
// Panics if the struct has exported fields without json tags.
func NewTool[T any, C any](name, description string, fn func(C, T) (string, error)) *Tool {
	var zero T
	typ := reflect.TypeOf(zero)

	// Validate struct has proper tags
	if err := validateStructTags(typ); err != nil {
		panic(fmt.Sprintf("NewTool(%s): %v", name, err))
	}

	schema := generateSchema(typ)

	return &Tool{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Execute: func(args map[string]interface{}) (*ToolResult, error) {
			// Marshal args to JSON and back to get proper types
			jsonData, err := json.Marshal(args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal arguments: %w", err)
			}

			var params T
			if err := json.Unmarshal(jsonData, &params); err != nil {
				return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
			}

			// Execute the typed function with a zero context value
			var ctx C
			result, err := fn(ctx, params)
			if err != nil {
				return nil, err
			}

			return &ToolResult{
				Content: []ToolContent{{Type: "text", Content: result}},
			}, nil
		},
	}
}

// validateStructTags checks if all exported fields have json tags
func validateStructTags(typ reflect.Type) error {
	// Handle pointer types
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Non-struct types are allowed (simple types)
	if typ.Kind() != reflect.Struct {
		return nil
	}

	var missingTags []string

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Check for json tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			missingTags = append(missingTags, field.Name)
		}
	}

	if len(missingTags) > 0 {
		return fmt.Errorf("struct %s has exported fields without json tags: %v. Add json tags like `json:\"field_name\"` to these fields", typ.Name(), missingTags)
	}

	return nil
}

// generateSchema creates a JSON schema from a reflect.Type
func generateSchema(typ reflect.Type) map[string]interface{} {
	// Handle pointer types
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		// For non-struct types, return a simple schema
		return map[string]interface{}{
			"type": "object",
		}
	}

	properties := make(map[string]interface{})
	var required []string

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Parse json tag (e.g., "field_name,omitempty")
		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		omitempty := len(parts) > 1 && parts[1] == "omitempty"

		// Build property schema
		propSchema := buildPropertySchema(field)

		properties[fieldName] = propSchema

		// Add to required if not omitempty
		if !omitempty {
			required = append(required, fieldName)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// buildPropertySchema creates the schema for a single field
func buildPropertySchema(field reflect.StructField) map[string]interface{} {
	schema := make(map[string]interface{})

	// Get description from tag
	if desc := field.Tag.Get("description"); desc != "" {
		schema["description"] = desc
	}

	// Get default from tag
	if defaultVal := field.Tag.Get("default"); defaultVal != "" {
		schema["default"] = defaultVal
	}

	// Map Go type to JSON schema type
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	switch fieldType.Kind() {
	case reflect.String:
		schema["type"] = "string"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema["type"] = "integer"

	case reflect.Float32, reflect.Float64:
		schema["type"] = "number"

	case reflect.Bool:
		schema["type"] = "boolean"

	case reflect.Slice, reflect.Array:
		schema["type"] = "array"
		// For slices, try to infer item type
		if fieldType.Elem().Kind() == reflect.String {
			schema["items"] = map[string]interface{}{"type": "string"}
		} else if fieldType.Elem().Kind() == reflect.Struct {
			schema["items"] = generateSchema(fieldType.Elem())
		}

	case reflect.Map:
		schema["type"] = "object"

	case reflect.Struct:
		// For nested structs, recursively generate schema
		return generateSchema(fieldType)

	default:
		schema["type"] = "string"
	}

	return schema
}
