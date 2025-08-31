package aigentic

import (
	"context"
	"fmt"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/stretchr/testify/assert"
)

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

	result, err := agent.Execute("Test message")

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

	result, err := agent.Execute("Test message")

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
		AgentTools:  []AgentTool{WrapTool(testTool)},
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

	result, err := agent.Execute("Please use the test tool")

	assert.NoError(t, err)
	assert.True(t, toolCalled, "Tool should have been called")
	assert.Equal(t, "Hello from tool call", toolArgs["message"])
	assert.Contains(t, result, "Perfect! I've successfully used the test tool")
	assert.Equal(t, 2, callCount, "Model should have been called twice")
}

func TestAgentFileAttachment(t *testing.T) {
	receivedMessages := []ai.Message{}

	doc1 := document.NewInMemoryDocument("", "test.txt", []byte("This is a text file content"), nil)
	doc2 := document.NewInMemoryDocument("", "test.png", []byte("fake image data"), nil)
	agent := Agent{
		Name:        "test-attachment-agent",
		Description: "A test agent that handles file attachments",
		Documents: []*document.Document{
			doc1,
			doc2,
		},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			receivedMessages = messages

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "I've received your message and the attached files. I can see you've attached a text file and an image.",
			}, nil
		}),
	}

	result, err := agent.Execute("Please analyze these attached files")

	assert.NoError(t, err)
	assert.Contains(t, result, "I've received your message and the attached files")

	// Verify that at least the expected messages were received by the model
	if !assert.GreaterOrEqual(t, len(receivedMessages), 4) {
		t.FailNow()
	}

	// Check that the user message is present
	assert.Contains(t, receivedMessages[1].(ai.UserMessage).Content, "Please analyze these attached files")

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
	callCount := 0

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
		Agents:      []Agent{subAgent},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++
			// First call: request sub-agent tool call regardless of message count
			if callCount == 1 {
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

	result, err := mainAgent.Execute("I need help with a calculation")

	assert.NoError(t, err)
	assert.True(t, subAgentCalled, "Sub-agent should have been called")
	assert.Contains(t, subAgentInput, "Please help me with this calculation")
	assert.Contains(t, result, "Great! My helper agent has completed the task")
	assert.Contains(t, result, "42")
}

func TestAgentMultipleToolRequestsWithSameTool(t *testing.T) {
	tool1Called := 0
	tool2Called := 0
	callCount := 0

	// Create a test tool that will be called multiple times
	testTool1 := ai.Tool{
		Name:        "lookup_company",
		Description: "A test tool for looking up companies",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "The input to lookup",
				},
			},
			"required": []string{"input"},
		},
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			tool1Called++
			input := args["input"].(string)
			if input == "Look up company 150" {
				return &ai.ToolResult{
					Content: []ai.ToolContent{{
						Type:    "text",
						Content: "COMPANY: Nexxia",
					}},
					Error: false,
				}, nil
			}
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: "SUPPLIER: Phoenix",
				}},
				Error: false,
			}, nil
		},
	}

	// Create a second test tool
	testTool2 := ai.Tool{
		Name:        "save_memory",
		Description: "A test tool for saving to memory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to save",
				},
			},
			"required": []string{"content"},
		},
		Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
			tool2Called++
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: "memory saved successfully",
				}},
				Error: false,
			}, nil
		},
	}

	// Track tool response events to verify all are properly handled
	var receivedToolMessages []ai.ToolMessage

	agent := Agent{
		Name:        "test-multi-tool-agent",
		Description: "A test agent that makes multiple tool calls including same tool with different inputs",
		AgentTools:  []AgentTool{WrapTool(testTool1), WrapTool(testTool2)},
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++

			// First call: make 4 tool calls (2 lookup_company, 2 save_memory)
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll execute the plan step by step.",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_0",
							Type: "function",
							Name: "lookup_company",
							Args: `{"input": "Look up company 150"}`,
						},
						{
							ID:   "call_1",
							Type: "function",
							Name: "save_memory",
							Args: `{"content": "Company 150: Nexxia"}`,
						},
						{
							ID:   "call_2",
							Type: "function",
							Name: "lookup_company",
							Args: `{"input": "Look up supplier 200"}`,
						},
						{
							ID:   "call_3",
							Type: "function",
							Name: "save_memory",
							Args: `{"content": "Supplier 200: Phoenix"}`,
						},
					},
				}, nil
			}

			// Second call: capture tool messages to verify they match the requests
			for _, msg := range messages {
				if toolMsg, ok := msg.(ai.ToolMessage); ok {
					receivedToolMessages = append(receivedToolMessages, toolMsg)
				}
			}

			// Second call: return final response after tool execution
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "All tools executed successfully. Company: Nexxia, Supplier: Phoenix",
			}, nil
		}),
	}

	result, err := agent.Execute("Execute the plan")

	assert.NoError(t, err)
	assert.Contains(t, result, "All tools executed successfully")

	// Verify that both tools were called the expected number of times
	assert.Equal(t, 2, tool1Called, "lookup_company should have been called twice")
	assert.Equal(t, 2, tool2Called, "save_memory should have been called twice")

	// Most importantly: verify that we received 4 tool response messages that match the original tool calls
	assert.Equal(t, 4, len(receivedToolMessages), "Should have received 4 tool response messages, one for each tool call")

	// Verify that each tool response message has the correct ToolCallID and ToolName that correspond to the original requests
	expectedToolResponses := map[string]struct {
		toolName string
		content  string
	}{
		"call_0": {"lookup_company", "COMPANY: Nexxia"},
		"call_1": {"save_memory", "memory saved successfully"},
		"call_2": {"lookup_company", "SUPPLIER: Phoenix"},
		"call_3": {"save_memory", "memory saved successfully"},
	}

	actualToolResponses := make(map[string]struct {
		toolName string
		content  string
	})
	for _, toolMsg := range receivedToolMessages {
		actualToolResponses[toolMsg.ToolCallID] = struct {
			toolName string
			content  string
		}{toolMsg.ToolName, toolMsg.Content}
	}

	assert.Equal(t, expectedToolResponses, actualToolResponses, "Tool response messages should have correct tool call IDs, tool names, and content that match the original requests")
}
