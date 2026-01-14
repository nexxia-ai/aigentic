package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
)

type AgentTool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	Execute     func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error)
}

func (t *AgentTool) call(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
	if t.Execute != nil {
		return t.Execute(run, args)
	}
	return nil, nil
}

func (t *AgentTool) toTool(run *AgentRun) ai.Tool {
	return ai.Tool{
		Name:        t.Name,
		Description: t.Description,
		InputSchema: t.InputSchema,
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			return t.Execute(run, args)
		},
	}
}

func WrapTool(tool ai.Tool) AgentTool {
	return AgentTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: tool.InputSchema,
		Execute: func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			return tool.Execute(args)
		},
	}
}

// NewTool creates an AgentTool with auto-generated JSON schema from a typed function.
// The input parameter T must be a struct with json tags for schema generation.
//
// Example:
//
//	type CalculatorInput struct {
//	    Expression string `json:"expression" description:"Mathematical expression to evaluate"`
//	}
//
//	tool := NewTool(
//	    "calculator",
//	    "Performs mathematical calculations",
//	    func(run *AgentRun, input CalculatorInput) (string, error) {
//	        return evaluateExpression(input.Expression), nil
//	    },
//	)
func NewTool[T any](name, description string, fn func(*AgentRun, T) (string, error)) AgentTool {
	var zero T
	typ := reflect.TypeOf(zero)

	if err := validateStructTags(typ); err != nil {
		panic(fmt.Sprintf("NewTool(%s): %v", name, err))
	}

	schema := generateSchema(typ)

	return AgentTool{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Execute: func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			if run == nil {
				return nil, errors.New("AgentRun is nil")
			}

			jsonData, err := json.Marshal(args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal arguments: %w", err)
			}

			var params T
			if err := json.Unmarshal(jsonData, &params); err != nil {
				return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
			}

			result, err := fn(run, params)
			if err != nil {
				return nil, err
			}

			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: result}},
			}, nil
		},
	}
}

func validateStructTags(typ reflect.Type) error {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return nil
	}

	var missingTags []string

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

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

func generateSchema(typ reflect.Type) map[string]interface{} {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return map[string]interface{}{
			"type": "object",
		}
	}

	properties := make(map[string]interface{})
	var required []string

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		omitempty := len(parts) > 1 && parts[1] == "omitempty"

		propSchema := buildPropertySchema(field)

		properties[fieldName] = propSchema

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

func buildPropertySchema(field reflect.StructField) map[string]interface{} {
	schema := make(map[string]interface{})

	if desc := field.Tag.Get("description"); desc != "" {
		schema["description"] = desc
	}

	if defaultVal := field.Tag.Get("default"); defaultVal != "" {
		schema["default"] = defaultVal
	}

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
		if fieldType.Elem().Kind() == reflect.String {
			schema["items"] = map[string]interface{}{"type": "string"}
		} else if fieldType.Elem().Kind() == reflect.Struct {
			schema["items"] = generateSchema(fieldType.Elem())
		}

	case reflect.Map:
		schema["type"] = "object"

	case reflect.Struct:
		return generateSchema(fieldType)

	default:
		schema["type"] = "string"
	}

	return schema
}
