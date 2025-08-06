package aigentic

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
)

func TestCreateUserMsg(t *testing.T) {
	doc1 := NewInMemoryDocument("", "file-abc123", []byte("test content"), nil)
	doc2 := NewInMemoryDocument("", "test.png", []byte("image data"), nil)
	agent := Agent{
		Documents: []*Document{
			&doc1,
			&doc2,
		},
	}

	// Test with message and attachments (no FileID)
	messages := agent.createUserMsg("Hello, please analyze these files")

	assert.Len(t, messages, 3) // 1 main message + 2 attachments

	// Check main message
	mainMsg, ok := messages[0].(ai.UserMessage)
	assert.True(t, ok)
	assert.Equal(t, "Hello, please analyze these files", mainMsg.Content)

	// Check first attachment message (should include content)
	att1Msg, ok := messages[1].(ai.ResourceMessage)
	assert.True(t, ok)
	assert.Contains(t, att1Msg.Name, "file-abc123")
	assert.Contains(t, string(att1Msg.Body.([]byte)), "test content")

	// Check second attachment message (should include content)
	att2Msg, ok := messages[2].(ai.ResourceMessage)
	assert.True(t, ok)
	assert.Contains(t, att2Msg.Name, "test.png")
	assert.Contains(t, string(att2Msg.Body.([]byte)), "image data")

}

func TestAgentRunAndWait(t *testing.T) {
	agent := Agent{
		Name:        "test-agent",
		Description: "A test agent for testing RunAndWait functionality",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Hello! I received your message and processed it successfully.",
			}, nil
		}),
	}

	result, err := agent.RunAndWait("Test message")

	assert.NoError(t, err)
	assert.Equal(t, "Hello! I received your message and processed it successfully.", result)
}

func TestAgentRunAndWaitWithError(t *testing.T) {
	agent := Agent{
		Name:        "test-agent-error",
		Description: "A test agent that returns an error",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			return ai.AIMessage{}, fmt.Errorf("simulated error")
		}),
	}

	result, err := agent.RunAndWait("Test message")

	assert.Error(t, err)
	assert.Equal(t, "", result)
	assert.Contains(t, err.Error(), "simulated error")
}

func TestAgentToolCalling(t *testing.T) {
	toolCalled := false
	toolArgs := make(map[string]interface{})
	callCount := 0

	testTool := ai.Tool{
		Name:        "test_tool",
		Description: "A test tool that records when it's called",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The message to process",
				},
			},
			"required": []string{"message"},
		},
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			toolCalled = true
			toolArgs = args
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: "Tool executed successfully with message: " + args["message"].(string),
				}},
				Error: false,
			}, nil
		},
	}

	agent := Agent{
		Name:        "test-tool-agent",
		Description: "A test agent that uses tools",
		Trace:       NewTrace(),
		Tools:       []ai.Tool{testTool},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			// First call: make a tool call
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll use the test tool to process your request.",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Name: "test_tool",
							Args: `{"message": "Hello from tool call"}`,
						},
					},
				}, nil
			}

			// Second call: return final response after tool execution
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Perfect! I've successfully used the test tool and received the response. The tool execution was completed successfully.",
			}, nil
		}),
	}

	result, err := agent.RunAndWait("Please use the test tool")

	assert.NoError(t, err)
	assert.True(t, toolCalled, "Tool should have been called")
	assert.Equal(t, "Hello from tool call", toolArgs["message"])
	assert.Contains(t, result, "Perfect! I've successfully used the test tool")
	assert.Equal(t, 2, callCount, "Model should have been called twice")
}

func TestAgentFileAttachment(t *testing.T) {
	receivedMessages := []ai.Message{}

	doc1 := NewInMemoryDocument("", "test.txt", []byte("This is a text file content"), nil)
	doc2 := NewInMemoryDocument("", "test.png", []byte("fake image data"), nil)
	agent := Agent{
		Name:        "test-attachment-agent",
		Description: "A test agent that handles file attachments",
		Documents: []*Document{
			&doc1,
			&doc2,
		},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			receivedMessages = messages

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "I've received your message and the attached files. I can see you've attached a text file and an image.",
			}, nil
		}),
	}

	result, err := agent.RunAndWait("Please analyze these attached files")

	assert.NoError(t, err)
	assert.Contains(t, result, "I've received your message and the attached files")

	// Verify that messages were received by the model
	assert.Len(t, receivedMessages, 4) // System message + User message + 2 attachments

	// Check that the user message is present
	userMsgFound := false
	for _, msg := range receivedMessages {
		if userMsg, ok := msg.(ai.UserMessage); ok {
			assert.Equal(t, "Please analyze these attached files", userMsg.Content)
			userMsgFound = true
		}
	}
	assert.True(t, userMsgFound, "User message should be present")

	// Check that both attachments are present as ResourceMessages
	textFileFound := false
	imageFileFound := false
	for _, msg := range receivedMessages {
		if resourceMsg, ok := msg.(ai.ResourceMessage); ok {
			if resourceMsg.Name == "test.txt" {
				assert.Equal(t, "text", resourceMsg.Type)
				assert.Equal(t, []byte("This is a text file content"), resourceMsg.Body)
				textFileFound = true
			} else if resourceMsg.Name == "test.png" {
				assert.Equal(t, "image", resourceMsg.Type)
				assert.Equal(t, []byte("fake image data"), resourceMsg.Body)
				imageFileFound = true
			}
		}
	}
	assert.True(t, textFileFound, "Text file attachment should be present")
	assert.True(t, imageFileFound, "Image file attachment should be present")
}

func TestAgentCallingSubAgent(t *testing.T) {
	subAgentCalled := false
	subAgentInput := ""

	// Create a sub-agent
	subAgent := Agent{
		Name:        "helper-agent",
		Description: "A helper agent that processes requests",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			subAgentCalled = true
			// Extract the user message to get the input
			for _, msg := range messages {
				if userMsg, ok := msg.(ai.UserMessage); ok {
					subAgentInput = userMsg.Content
					break
				}
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "I've processed your request and found the answer: 42",
			}, nil
		}),
	}

	// Create the main agent that will call the sub-agent
	mainAgent := Agent{
		Name:        "main-agent",
		Description: "A main agent that delegates work to sub-agents",
		Agents:      []*Agent{&subAgent},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			// Check if this is the first call (should make a tool call to sub-agent)
			if len(messages) <= 3 { // System + User + maybe some history
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll delegate this to my helper agent.",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_sub_agent",
							Type: "function",
							Name: "helper-agent",
							Args: `{"input": "Please help me with this calculation"}`,
						},
					},
				}, nil
			}

			// Second call: return final response after sub-agent execution
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Great! My helper agent has completed the task. Here's what they found: 42",
			}, nil
		}),
	}

	result, err := mainAgent.RunAndWait("I need help with a calculation")

	assert.NoError(t, err)
	assert.True(t, subAgentCalled, "Sub-agent should have been called")
	assert.Equal(t, "Please help me with this calculation", subAgentInput)
	assert.Contains(t, result, "Great! My helper agent has completed the task")
	assert.Contains(t, result, "42")
}
