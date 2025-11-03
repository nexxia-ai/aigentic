package aigentic

import (
	"context"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
)

func TestMessageOrderWithMultipleStartCalls(t *testing.T) {
	history := NewConversationHistory()
	callCount := 0

	testTool := AgentTool{
		Name:        "echo",
		Description: "Echoes back the input",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"text"},
		},
		NewExecute: func(run *AgentRun, validationResult ValidationResult) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{
					{Type: "text", Content: "echoed: test"},
				},
			}, nil
		},
	}

	agent := Agent{
		Name:                "test-agent",
		ConversationHistory: history,
		AgentTools:          []AgentTool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			if callCount == 1 {
				return ai.AIMessage{
					Role: ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Name: "echo",
							Args: `{"text": "test"}`,
						},
					},
				}, nil
			}

			if callCount == 2 {
				verifyMessageOrder(t, messages)
				verifyToolCallIDsMatch(t, messages)
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Second call completed",
				}, nil
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Final response",
			}, nil
		}),
	}

	run1, err := agent.Start("First message - use echo tool")
	assert.NoError(t, err)
	_, err = run1.Wait(0)
	assert.NoError(t, err)

	run2, err := agent.Start("Second message")
	assert.NoError(t, err)
	_, err = run2.Wait(0)
	assert.NoError(t, err)
}

func verifyToolCallIDsMatch(t *testing.T, messages []ai.Message) {
	var currentAssistantMessage ai.AIMessage
	var assistantHasToolCalls bool

	for _, msg := range messages {
		role, _ := msg.Value()

		if role == ai.AssistantRole {
			if aiMsg, ok := msg.(ai.AIMessage); ok {
				currentAssistantMessage = aiMsg
				assistantHasToolCalls = len(aiMsg.ToolCalls) > 0
			}
		}

		if role == ai.ToolRole {
			if toolMsg, ok := msg.(ai.ToolMessage); ok {
				if assistantHasToolCalls {
					// Check if this tool_call_id exists in the assistant message's tool_calls
					found := false
					for _, tc := range currentAssistantMessage.ToolCalls {
						if tc.ID == toolMsg.ToolCallID {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Tool message with tool_call_id '%s' does not match any tool_call ID in the preceding assistant message", toolMsg.ToolCallID)
					}
				}
			}
		}
	}
}

func verifyMessageOrder(t *testing.T, messages []ai.Message) {
	assistantWithToolCallsFound := false
	toolMessageFound := false
	lastAssistantIndex := -1

	for i, msg := range messages {
		role, _ := msg.Value()

		if role == ai.AssistantRole {
			if aiMsg, ok := msg.(ai.AIMessage); ok && len(aiMsg.ToolCalls) > 0 {
				assistantWithToolCallsFound = true
				lastAssistantIndex = i
			}
		}

		if role == ai.ToolRole {
			if !assistantWithToolCallsFound {
				t.Errorf("Tool message found at index %d without preceding assistant message with tool_calls. Last assistant was at index %d", i, lastAssistantIndex)
			}
			if lastAssistantIndex >= i {
				t.Errorf("Tool message found at index %d but assistant with tool_calls is at index %d (should come before)", i, lastAssistantIndex)
			}
			toolMessageFound = true
		}
	}

	assert.True(t, assistantWithToolCallsFound, "Should have assistant message with tool_calls")
	assert.True(t, toolMessageFound, "Should have tool message")
}
