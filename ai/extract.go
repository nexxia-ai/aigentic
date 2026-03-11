package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
// Returns the LLM content body (for display/notification), one KeyValue per slot in the same order,
// and an error. Content is accumulated from streaming chunks when onStream is used, or from the
// response message on the non-streaming path. Relies on a single tool call (submit_extraction) for
// structured output.
//
// onStream is optional: if non-nil, Extract uses streaming and calls onStream with each content chunk;
// the tool is still invoked once at the end. If onStream is nil, a single non-streaming call is made.
func Extract(ctx context.Context, model *Model, text string, slots []KeyValue, onStream func(chunk string) error) (content string, result []KeyValue, err error) {
	if model == nil {
		return "", nil, fmt.Errorf("model is required")
	}
	if len(slots) == 0 {
		return "", nil, nil
	}

	sysContent := buildExtractSystemPrompt(slots)
	messages := []Message{
		SystemMessage{Role: SystemRole, Content: sysContent},
		UserMessage{Role: UserRole, Content: text},
	}
	tool := extractTool()

	if onStream != nil && model.callStreamingFunc != nil {
		slog.Debug("Extract using streaming path")
		var body strings.Builder
		send := func(s string) error {
			if s != "" {
				body.WriteString(s)
				return onStream(s)
			}
			return nil
		}
		chunkFn := func(chunk AIMessage) error {
			s := chunk.Content
			if s == "" && len(chunk.Parts) > 0 {
				for _, p := range chunk.Parts {
					if p.Type == ContentPartText && p.Text != "" {
						s = p.Text
						break
					}
				}
			}
			if s != "" {
				slog.Debug("Extract forwarding content chunk to callback", "len", len(s), "preview", truncatePreview(s, 80))
				return send(s)
			}
			return nil
		}
		resp, err := model.Stream(ctx, messages, []Tool{tool}, chunkFn)
		if err != nil {
			return "", nil, err
		}
		// Include any content only present in the final message
		if rest := messageContent(resp); rest != "" {
			body.WriteString(rest)
			_ = onStream(rest)
		}
		kv, err := normaliseExtractResult(slots, resp)
		return body.String(), kv, err
	}

	slog.Debug("Extract using non-streaming path", "streaming_available", model.callStreamingFunc != nil)
	resp, err := model.Call(ctx, messages, []Tool{tool})
	if err != nil {
		return "", nil, err
	}
	body := messageContent(resp)
	if body != "" && onStream != nil {
		_ = onStream(body)
	}
	kv, err := normaliseExtractResult(slots, resp)
	return body, kv, err
}

func buildExtractSystemPrompt(slots []KeyValue) string {
	var b strings.Builder
	b.WriteString("Fill each key from the user message. Use one short phrase or line per key. ")
	b.WriteString("You may stream a single brief status (e.g. \"Extracting...\") then call submit_extraction immediately. ")
	b.WriteString("Call submit_extraction exactly once with all keys; use empty string for any key you cannot fill.\n\n")
	b.WriteString("Keys:\n\n")
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
					"type":        "array",
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

func messageContent(msg AIMessage) string {
	s := strings.TrimSpace(msg.Content)
	if s != "" {
		return s
	}
	for _, p := range msg.Parts {
		if p.Type == ContentPartText && p.Text != "" {
			return strings.TrimSpace(p.Text)
		}
	}
	return ""
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

func truncatePreview(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
