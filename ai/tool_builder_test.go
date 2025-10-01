package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type SimpleInput struct {
	Message string `json:"message" description:"A simple message"`
}

type ComplexInput struct {
	Query      string   `json:"query" description:"Search query"`
	MaxResults int      `json:"max_results,omitempty" description:"Maximum number of results" default:"10"`
	Filters    []string `json:"filters,omitempty" description:"Filter criteria"`
	Enabled    bool     `json:"enabled" description:"Whether feature is enabled"`
}

type NestedInput struct {
	Name    string                 `json:"name" description:"Name field"`
	Options map[string]interface{} `json:"options,omitempty" description:"Optional settings"`
	Count   *int                   `json:"count,omitempty" description:"Optional count"`
}

type CalculatorInput struct {
	Expression string `json:"expression" description:"Mathematical expression to evaluate"`
}

func TestNewTool_Simple(t *testing.T) {
	tool := NewTool(
		"echo",
		"Echoes back a message",
		func(ctx context.Context, input SimpleInput) (string, error) {
			return "Echo: " + input.Message, nil
		},
	)

	if tool.Name != "echo" {
		t.Errorf("expected name 'echo', got %s", tool.Name)
	}

	if tool.Description != "Echoes back a message" {
		t.Errorf("expected description, got %s", tool.Description)
	}

	// Check schema generation
	schema := tool.InputSchema
	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}

	props := schema["properties"].(map[string]interface{})
	if props["message"] == nil {
		t.Error("expected 'message' property in schema")
	}

	messageProp := props["message"].(map[string]interface{})
	if messageProp["type"] != "string" {
		t.Errorf("expected message type 'string', got %v", messageProp["type"])
	}

	if messageProp["description"] != "A simple message" {
		t.Errorf("expected description, got %v", messageProp["description"])
	}

	// Check required fields
	required := schema["required"].([]string)
	if len(required) != 1 || required[0] != "message" {
		t.Errorf("expected required=['message'], got %v", required)
	}
}

func TestNewTool_ComplexTypes(t *testing.T) {
	tool := NewTool(
		"search",
		"Performs a search",
		func(ctx context.Context, input ComplexInput) (string, error) {
			return "Search results", nil
		},
	)

	schema := tool.InputSchema
	props := schema["properties"].(map[string]interface{})

	// Check string field
	if props["query"].(map[string]interface{})["type"] != "string" {
		t.Error("expected query to be string")
	}

	// Check integer field with default
	maxResults := props["max_results"].(map[string]interface{})
	if maxResults["type"] != "integer" {
		t.Error("expected max_results to be integer")
	}
	if maxResults["default"] != "10" {
		t.Errorf("expected default '10', got %v", maxResults["default"])
	}

	// Check array field
	filters := props["filters"].(map[string]interface{})
	if filters["type"] != "array" {
		t.Error("expected filters to be array")
	}

	// Check boolean field
	enabled := props["enabled"].(map[string]interface{})
	if enabled["type"] != "boolean" {
		t.Error("expected enabled to be boolean")
	}

	// Check required fields (only non-omitempty)
	required := schema["required"].([]string)
	hasQuery := false
	hasEnabled := false
	for _, r := range required {
		if r == "query" {
			hasQuery = true
		}
		if r == "enabled" {
			hasEnabled = true
		}
		if r == "max_results" || r == "filters" {
			t.Errorf("omitempty field %s should not be required", r)
		}
	}
	if !hasQuery {
		t.Error("query should be required")
	}
	if !hasEnabled {
		t.Error("enabled should be required")
	}
}

func TestNewTool_Execution(t *testing.T) {
	tool := NewTool(
		"calculator",
		"Performs calculations",
		func(ctx context.Context, input CalculatorInput) (string, error) {
			return "Result: " + input.Expression, nil
		},
	)

	// Execute the tool
	args := map[string]interface{}{
		"expression": "2 + 2",
	}

	result, err := tool.Call(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content[0].Content != "Result: 2 + 2" {
		t.Errorf("expected 'Result: 2 + 2', got %s", result.Content[0].Content)
	}
}

func TestNewTool_ExecutionWithOmitempty(t *testing.T) {
	tool := NewTool(
		"search",
		"Performs search",
		func(ctx context.Context, input ComplexInput) (string, error) {
			if input.MaxResults == 0 {
				input.MaxResults = 10
			}
			return "Found results", nil
		},
	)

	// Execute without optional fields
	args := map[string]interface{}{
		"query":   "test",
		"enabled": true,
	}

	result, err := tool.Call(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Content[0].Content != "Found results" {
		t.Errorf("expected 'Found results', got %s", result.Content[0].Content)
	}
}

func TestNewTool_NestedStructs(t *testing.T) {
	tool := NewTool(
		"configure",
		"Configure settings",
		func(ctx context.Context, input NestedInput) (string, error) {
			return "Configured: " + input.Name, nil
		},
	)

	schema := tool.InputSchema
	props := schema["properties"].(map[string]interface{})

	// Check map field
	options := props["options"].(map[string]interface{})
	if options["type"] != "object" {
		t.Error("expected options to be object")
	}

	// Check pointer field
	count := props["count"].(map[string]interface{})
	if count["type"] != "integer" {
		t.Error("expected count to be integer (pointer unwrapped)")
	}
}

func TestNewTool_InvalidArgs(t *testing.T) {
	tool := NewTool(
		"test",
		"Test tool",
		func(ctx context.Context, input SimpleInput) (string, error) {
			return input.Message, nil
		},
	)

	// Try with invalid argument type
	args := map[string]interface{}{
		"message": 123, // Should be string
	}

	_, err := tool.Call(args)
	if err == nil {
		t.Error("expected error with invalid argument type")
	}
}

type NoTagsInput struct {
	Query   string
	Enabled bool
}

type PartialTagsInput struct {
	Query   string `json:"query" description:"Search query"`
	Enabled bool   // Missing tag
	Count   int    // Missing tag
}

type MixedTagsInput struct {
	Name        string  `json:"name" description:"Name field"`
	Description string  `json:"-"` // Explicitly excluded
	Internal    string  // Missing tag
	Score       float64 // Missing tag
}

func TestNewTool_MissingTags_AllFields(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when struct has no json tags")
		}

		panicMsg := fmt.Sprint(r)
		if !strings.Contains(panicMsg, "NoTagsInput") {
			t.Errorf("panic message should contain struct name 'NoTagsInput', got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "Query") {
			t.Errorf("panic message should mention missing field 'Query', got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "Enabled") {
			t.Errorf("panic message should mention missing field 'Enabled', got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "without json tags") {
			t.Errorf("panic message should indicate missing json tags, got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "no_tags") {
			t.Errorf("panic message should contain tool name 'no_tags', got: %s", panicMsg)
		}
	}()

	NewTool(
		"no_tags",
		"Tool with no tags",
		func(ctx context.Context, input NoTagsInput) (string, error) {
			return "test", nil
		},
	)
}

func TestNewTool_MissingTags_PartialFields(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when struct has partial json tags")
		}

		panicMsg := fmt.Sprint(r)
		if !strings.Contains(panicMsg, "PartialTagsInput") {
			t.Errorf("panic message should contain struct name 'PartialTagsInput', got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "Enabled") {
			t.Errorf("panic message should mention missing field 'Enabled', got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "Count") {
			t.Errorf("panic message should mention missing field 'Count', got: %s", panicMsg)
		}
		if strings.Contains(panicMsg, "Query") {
			t.Errorf("panic message should NOT mention 'Query' since it has a tag, got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "json:\"field_name\"") {
			t.Errorf("panic message should include example fix, got: %s", panicMsg)
		}
	}()

	NewTool(
		"partial_tags",
		"Tool with partial tags",
		func(ctx context.Context, input PartialTagsInput) (string, error) {
			return "test", nil
		},
	)
}

func TestNewTool_MissingTags_WithExplicitExclusion(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic when struct has missing json tags")
		}

		panicMsg := fmt.Sprint(r)
		// Should report Internal and Score, but NOT Description (which has json:"-")
		if !strings.Contains(panicMsg, "Internal") {
			t.Errorf("panic message should mention missing field 'Internal', got: %s", panicMsg)
		}
		if !strings.Contains(panicMsg, "Score") {
			t.Errorf("panic message should mention missing field 'Score', got: %s", panicMsg)
		}
		if strings.Contains(panicMsg, "Description") {
			t.Errorf("panic message should NOT mention 'Description' (has json:\"-\"), got: %s", panicMsg)
		}
		if strings.Contains(panicMsg, "Name") {
			t.Errorf("panic message should NOT mention 'Name' (has tag), got: %s", panicMsg)
		}
	}()

	NewTool(
		"mixed_tags",
		"Tool with mixed tags",
		func(ctx context.Context, input MixedTagsInput) (string, error) {
			return "test", nil
		},
	)
}
