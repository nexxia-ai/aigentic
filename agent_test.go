package aigentic

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/memory"
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

	type TestToolInput struct {
		Message string `json:"message" description:"The message to process"`
	}

	testTool := ai.NewTool(
		"test_tool",
		"A test tool that records when it's called",
		func(ctx context.Context, input TestToolInput) (string, error) {
			toolCalled = true
			toolArgs = map[string]interface{}{"message": input.Message}
			return "Tool executed successfully with message: " + input.Message, nil
		},
	)

	agent := Agent{
		Name:        "test-tool-agent",
		Description: "A test agent that uses tools",
		Trace:       NewTrace(),
		AgentTools:  []AgentTool{WrapTool(*testTool)},
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
	type LookupToolInput struct {
		Input string `json:"input" description:"The input to lookup"`
	}

	testTool1 := ai.NewTool(
		"lookup_company",
		"A test tool for looking up companies",
		func(ctx context.Context, input LookupToolInput) (string, error) {
			tool1Called++
			if input.Input == "Look up company 150" {
				return "COMPANY: Nexxia", nil
			}
			return "SUPPLIER: Phoenix", nil
		},
	)

	// Create a second test tool
	type SaveMemoryInput struct {
		Content string `json:"content" description:"The content to save"`
	}

	testTool2 := ai.NewTool(
		"save_memory",
		"A test tool for saving to memory",
		func(ctx context.Context, input SaveMemoryInput) (string, error) {
			tool2Called++
			return "memory saved successfully", nil
		},
	)

	// Track tool response events to verify all are properly handled
	var receivedToolMessages []ai.ToolMessage

	agent := Agent{
		Name:        "test-multi-tool-agent",
		Description: "A test agent that makes multiple tool calls including same tool with different inputs",
		AgentTools:  []AgentTool{WrapTool(*testTool1), WrapTool(*testTool2)},
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

func TestStreamingCoordinatorWithChildAgents(t *testing.T) {
	session := NewSession(context.Background())
	session.Trace = NewTrace()

	callCount := 0

	// Static dummy model with predictable responses based on call count
	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		callCount++

		// First call: coordinator makes a tool call to child agent
		if callCount == 1 {
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "I'll delegate this to my child agent for detailed information.",
				ToolCalls: []ai.ToolCall{
					{
						ID:   "call_child_agent",
						Type: "function",
						Name: "child_agent",
						Args: `{"input": "Tell me about artificial intelligence and its applications"}`,
					},
				},
			}, nil
		}

		// Second call: child agent provides detailed response
		if callCount == 2 {
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Artificial Intelligence (AI) is a transformative technology that enables machines to perform tasks typically requiring human intelligence. AI applications span across multiple domains including healthcare, finance, transportation, and entertainment. Key technologies include machine learning, natural language processing, computer vision, and robotics.",
			}, nil
		}

		// Third call: coordinator provides final summary
		if callCount == 3 {
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "## AI Summary\n\nBased on the detailed analysis from my child agent, Artificial Intelligence represents a revolutionary technology with broad applications across industries. The key areas include machine learning, NLP, computer vision, and robotics, making it a cornerstone of modern technological advancement.",
			}, nil
		}

		// Fallback response
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Processing your request...",
		}, nil
	})

	// Child agent that provides streaming responses
	childAgent := Agent{
		Model:        model,
		Name:         "child_agent",
		Description:  "A child agent that provides detailed information about topics.",
		Instructions: "Provide detailed responses about the requested topic.",
		// Stream:       true, // Should inherit from parent
	}

	// Coordinator agent that calls the child agent
	coordinator := Agent{
		Session:     session,
		Model:       model,
		Name:        "coordinator",
		Description: "A coordinator that delegates tasks to child agents.",
		Instructions: `
		1. Create a brief summary about the topic
		2. Call the sub agent to provide detailed information about the topic
		3. Return the summary and the detailed information in markdown format`,
		Agents: []Agent{childAgent},
		Stream: true,
		Trace:  NewTrace(),
		Memory: memory.NewMemory(),
	}

	message := "Tell me about artificial intelligence and its applications"
	run, err := coordinator.Start(message)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	var coordinatorChunks []string
	var childAgentChunks []string
	var toolCalls []string

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *ContentEvent:
			// Check if this is from the coordinator or child agent
			switch e.AgentName {
			case "coordinator":
				assert.True(t, e.RunID == run.ID(), "Content event have a different RunID from the coordinator")
				coordinatorChunks = append(coordinatorChunks, e.Content)
			case "child_agent":
				assert.True(t, e.RunID != run.ID(), "Content event have a different RunID from the child agent")
				childAgentChunks = append(childAgentChunks, e.Content)
			default:
				// For unknown agent names, we'll still collect the content
				t.Logf("Received content from unknown agent: %s", e.AgentName)
			}
		case *ToolEvent:
			toolCalls = append(toolCalls, e.ToolName)
		case *ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *ErrorEvent:
			t.Fatalf("Agent error: %v", e.Err)
		}
	}

	// Validate that we received streaming chunks from both agents
	assert.Greater(t, len(coordinatorChunks), 1, "Should have received streaming chunks from coordinator")
	assert.Greater(t, len(childAgentChunks), 1, "Should have received streaming chunks from child agent")

	// Validate that child agent was called
	childAgentCalled := false
	for _, toolCall := range toolCalls {
		if toolCall == "child_agent" {
			childAgentCalled = true
			break
		}
	}
	assert.True(t, childAgentCalled, "Child agent should have been called")

	// Validate final content
	finalCoordinatorContent := strings.Join(coordinatorChunks, "")
	finalChildAgentContent := strings.Join(childAgentChunks, "")

	assert.NotEmpty(t, finalCoordinatorContent, "Coordinator should have produced content")
	assert.NotEmpty(t, finalChildAgentContent, "Child agent should have produced content")

	// Validate content quality
	assert.Contains(t, strings.ToLower(finalCoordinatorContent), "delegate", "Coordinator should mention delegation")
	assert.Contains(t, strings.ToLower(finalChildAgentContent), "artificial intelligence", "Child agent should mention AI")

	t.Logf("Coordinator content: %s", finalCoordinatorContent)
	t.Logf("Child agent content: %s", finalChildAgentContent)

}
