package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// KeyValue is a key-value pair for extraction, turn metadata, and user-prompt enrichment.
// Used by ai.Extract, Run() metadata, and ctxt turn tags.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

const extractToolName = "submit_extraction"

// Extract uses the model to fill key-value slots from text. Slots define the keys and their
// descriptions; the prompt uses those descriptions so the model knows what to produce per key.
// Returns one KeyValue per slot in the same order; Value is empty string if the model did not fill it.
// Relies on a single tool call (submit_extraction) for structured output.
func Extract(ctx context.Context, model *Model, text string, slots []KeyValue) ([]KeyValue, error) {
	if model == nil {
		return nil, fmt.Errorf("model is required")
	}
	if len(slots) == 0 {
		return nil, nil
	}

	sysContent := buildExtractSystemPrompt(slots)
	messages := []Message{
		SystemMessage{Role: SystemRole, Content: sysContent},
		UserMessage{Role: UserRole, Content: text},
	}
	tool := extractTool()
	resp, err := model.Call(ctx, messages, []Tool{tool})
	if err != nil {
		return nil, err
	}

	return normaliseExtractResult(slots, resp)
}

func buildExtractSystemPrompt(slots []KeyValue) string {
	var b strings.Builder
	b.WriteString("You are an extraction assistant. Analyze the user message and fill each of the following keys. ")
	b.WriteString("You must call the submit_extraction tool exactly once with a list of key-value pairs. ")
	b.WriteString("Include every key; use an empty string for any key you cannot fill. ")
	b.WriteString("Do not respond with free-form text—only call the tool.\n\n")
	b.WriteString("Keys to produce (use the description to know what to write for each key):\n\n")
	for _, s := range slots {
		b.WriteString("- ")
		b.WriteString(s.Key)
		b.WriteString(": ")
		b.WriteString(s.Value)
		b.WriteString("\n")
	}
	return b.String()
}

func extractTool() Tool {
	return Tool{
		Name:        extractToolName,
		Description: "Submit the extracted key-value pairs. Call once with all keys.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pairs": map[string]interface{}{
					"type": "array",
					"description": "List of key-value pairs",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"key":   map[string]interface{}{"type": "string", "description": "Key name"},
							"value": map[string]interface{}{"type": "string", "description": "Extracted value"},
						},
						"required": []string{"key", "value"},
					},
				},
			},
			"required": []string{"pairs"},
		},
		Execute: func(args map[string]interface{}) (*ToolResult, error) {
			return &ToolResult{}, nil
		},
	}
}

func normaliseExtractResult(slots []KeyValue, resp AIMessage) ([]KeyValue, error) {
	result := make([]KeyValue, len(slots))
	for i := range slots {
		result[i] = KeyValue{Key: slots[i].Key, Value: ""}
	}

	if len(resp.ToolCalls) == 0 {
		return result, nil
	}

	tc := resp.ToolCalls[0]
	if tc.Name != extractToolName || tc.Args == "" {
		return result, nil
	}

	var payload struct {
		Pairs []KeyValue `json:"pairs"`
	}
	if err := json.Unmarshal([]byte(tc.Args), &payload); err != nil {
		return result, nil
	}

	byKey := make(map[string]string)
	for _, p := range payload.Pairs {
		if p.Key != "" {
			byKey[p.Key] = p.Value
		}
	}
	for i := range result {
		if v, ok := byKey[slots[i].Key]; ok {
			result[i].Value = v
		}
	}
	return result, nil
}
