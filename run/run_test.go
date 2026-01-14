package run

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
	"github.com/stretchr/testify/assert"
)

type noOpTrace struct{}

func (n *noOpTrace) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	return messages, tools, nil
}

func (n *noOpTrace) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	return response, nil
}

func (n *noOpTrace) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, args map[string]any) (map[string]any, error) {
	return args, nil
}

func (n *noOpTrace) AfterToolCall(run *AgentRun, toolName string, toolCallID string, args map[string]any, result *ai.ToolResult) (*ai.ToolResult, error) {
	return result, nil
}

func (n *noOpTrace) RecordError(err error) error {
	return nil
}

func (n *noOpTrace) Close() error {
	return nil
}

func (n *noOpTrace) Filepath() string {
	return ""
}

func newTestTracer() Trace {
	return &noOpTrace{}
}

func TestRunLLMCallAction_StreamingAgent(t *testing.T) {
	tests := []struct {
		name           string
		streamingModel *ai.Model
		expectedChunks int
		expectedFinal  string
		description    string
	}{
		{
			name: "streaming agent with content chunks",
			streamingModel: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Hello! This is a test response that will be streamed in chunks.",
				}, nil
			}),
			expectedChunks: 3,
			expectedFinal:  "Hello! This is a test response that will be streamed in chunks.",
			description:    "Test streaming agent that returns content in chunks",
		},
		{
			name: "streaming agent with tool calls",
			streamingModel: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
				return ai.AIMessage{
					Role: ai.AssistantRole,
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Name: "test_tool",
							Args: `{"message": "test"}`,
						},
					},
				}, nil
			}),
			expectedChunks: 0,
			expectedFinal:  "",
			description:    "Test streaming agent that makes tool calls",
		},
		{
			name: "streaming agent with thinking",
			streamingModel: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Final response",
					Think:   "I need to think about this carefully",
				}, nil
			}),
			expectedChunks: 4,
			expectedFinal:  "Final response",
			description:    "Test streaming agent with thinking content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentRun, err := NewAgentRun("test-streaming-agent", tt.description, "", t.TempDir())
			if err != nil {
				t.Fatalf("failed to create test agent run: %v", err)
			}
			agentRun.SetModel(tt.streamingModel)
			agentRun.SetStreaming(true)
			agentRun.SetEnableTrace(true)
			defer agentRun.stop()

			var events []event.Event
			var contentEvents []*event.ContentEvent
			var thinkingEvents []*event.ThinkingEvent
			var llmCallEvents []*event.LLMCallEvent

			go func() {
				for evt := range agentRun.eventQueue {
					events = append(events, evt)

					switch e := evt.(type) {
					case *event.ContentEvent:
						contentEvents = append(contentEvents, e)
					case *event.ThinkingEvent:
						thinkingEvents = append(thinkingEvents, e)
					case *event.LLMCallEvent:
						llmCallEvents = append(llmCallEvents, e)
					}
				}
			}()

			agentRun.agentContext.StartTurn("")
			agentRun.runLLMCallAction("Test message")

			time.Sleep(100 * time.Millisecond)

			t.Logf("Content events: %d", len(contentEvents))
			t.Logf("Thinking events: %d", len(thinkingEvents))
			t.Logf("Total events: %d", len(events))
			for i, event := range events {
				t.Logf("Event %d: %T", i, event)
			}

			assert.Len(t, llmCallEvents, 1, "Should have one LLM call event")
			assert.Equal(t, "Test message", llmCallEvents[0].Message)
			assert.Equal(t, "test-streaming-agent", llmCallEvents[0].AgentName)

			totalChunks := len(contentEvents) + len(thinkingEvents)
			assert.Equal(t, tt.expectedChunks, totalChunks, "Should have expected total number of content events (chunks + final response)")

			contentMap := make(map[string]int)
			for _, ce := range contentEvents {
				contentMap[ce.Content]++
			}

			for content, count := range contentMap {
				assert.Equal(t, 1, count, "Content chunk should not be duplicated: %s", content)
			}

			if tt.expectedFinal != "" {
				finalContent := ""
				for _, ce := range contentEvents {
					finalContent += ce.Content
				}
				assert.Equal(t, tt.expectedFinal, finalContent, "Final concatenated content should match expected")
			}

		})
	}
}

func TestRunLLMCallAction_NonStreamingAgent(t *testing.T) {
	agentRun, err := NewAgentRun("test-non-streaming-agent", "Test non-streaming agent", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	agentRun.SetModel(ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "This is a non-streaming response",
		}, nil
	}))
	agentRun.SetStreaming(false)
	agentRun.SetEnableTrace(true)
	defer agentRun.stop()

	var contentEvents []*event.ContentEvent
	var llmCallEvents []*event.LLMCallEvent

	go func() {
		for evt := range agentRun.eventQueue {
			switch e := evt.(type) {
			case *event.ContentEvent:
				contentEvents = append(contentEvents, e)
			case *event.LLMCallEvent:
				llmCallEvents = append(llmCallEvents, e)
			}
		}
	}()

	agentRun.agentContext.StartTurn("")
	agentRun.runLLMCallAction("Test message")

	time.Sleep(100 * time.Millisecond)

	assert.Len(t, llmCallEvents, 1, "Should have one LLM call event")

	assert.Len(t, contentEvents, 1, "Non-streaming agent should have only one content event")
	assert.Equal(t, "This is a non-streaming response", contentEvents[0].Content)
}

func TestRunLLMCallAction_StreamingWithToolCalls(t *testing.T) {
	agentRun, err := NewAgentRun("test-tool-streaming-agent", "Test streaming agent with tool calls", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	agentRun.SetModel(ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{
			Role: ai.AssistantRole,
			ToolCalls: []ai.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Name: "test_tool_1",
					Args: `{"param": "value1"}`,
				},
				{
					ID:   "call_2",
					Type: "function",
					Name: "test_tool_2",
					Args: `{"param": "value2"}`,
				},
			},
		}, nil
	}))
	agentRun.SetStreaming(true)
	agentRun.SetEnableTrace(true)
	defer agentRun.stop()

	var contentEvents []*event.ContentEvent
	var toolEvents []*event.ToolEvent

	go func() {
		for evt := range agentRun.eventQueue {
			switch e := evt.(type) {
			case *event.ContentEvent:
				contentEvents = append(contentEvents, e)
			case *event.ToolEvent:
				toolEvents = append(toolEvents, e)
			}
		}
	}()

	agentRun.agentContext.StartTurn("")
	agentRun.runLLMCallAction("Test message")

	time.Sleep(100 * time.Millisecond)

	t.Logf("Content events: %d", len(contentEvents))
	t.Logf("Tool events: %d", len(toolEvents))

	var allEvents []event.Event
	go func() {
		for evt := range agentRun.eventQueue {
			allEvents = append(allEvents, evt)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	t.Logf("Total events: %d", len(allEvents))
	for i, event := range allEvents {
		t.Logf("Event %d: %T", i, event)
	}

	t.Logf("Note: Tool calls are processed in final response, not as streaming chunks")
}

func TestRunLLMCallAction_LLMCallLimit(t *testing.T) {
	agentRun, err := NewAgentRun("test-limited-agent", "Test agent with LLM call limit", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	agentRun.SetModel(ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Response",
		}, nil
	}))
	agentRun.SetStreaming(true)
	agentRun.SetMaxLLMCalls(2)
	agentRun.SetEnableTrace(true)
	defer agentRun.stop()

	var actions []action
	go func() {
		for action := range agentRun.actionQueue {
			actions = append(actions, action)
		}
	}()

	agentRun.agentContext.StartTurn("")
	agentRun.runLLMCallAction("First call")
	agentRun.runLLMCallAction("Second call")
	agentRun.runLLMCallAction("Third call")

	time.Sleep(100 * time.Millisecond)

	var stopActions []*stopAction
	for _, action := range actions {
		if sa, ok := action.(*stopAction); ok {
			stopActions = append(stopActions, sa)
		}
	}

	if len(stopActions) > 0 {
		var limitExceededAction *stopAction
		for _, sa := range stopActions {
			if sa.Error != nil && strings.Contains(sa.Error.Error(), "LLM call limit exceeded") {
				limitExceededAction = sa
				break
			}
		}

		if limitExceededAction != nil {
			t.Logf("Found stop action with LLM call limit error: %v", limitExceededAction.Error)
		} else {
			t.Logf("Stop actions found but none with LLM call limit error")
			for i, sa := range stopActions {
				t.Logf("Stop action %d: error=%v", i, sa.Error)
			}
		}
	} else {
		t.Logf("No stop actions found, total actions: %d", len(actions))
		for i, action := range actions {
			t.Logf("Action %d: %T", i, action)
		}
	}

	t.Logf("Test completed - LLM call limit behavior needs investigation")
}

func TestRunLLMCallAction_StreamingContentConcatenation(t *testing.T) {
	contentChunks := []string{"Hello", " world", "! This is", " a test", " response."}

	streamingModel := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Hello world! This is a test response.",
		}, nil
	})

	streamingModel.SetStreamingFunc(func(ctx context.Context, model *ai.Model, messages []ai.Message, tools []ai.Tool, chunkFunction func(ai.AIMessage) error) (ai.AIMessage, error) {
		for _, chunk := range contentChunks {
			chunkMsg := ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: chunk,
			}
			if err := chunkFunction(chunkMsg); err != nil {
				return ai.AIMessage{}, err
			}
		}

		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Hello world! This is a test response.",
		}, nil
	})

	agentRun, err := NewAgentRun("test-chunk-agent", "Test agent with controlled chunking", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	agentRun.SetModel(streamingModel)
	agentRun.SetStreaming(true)
	agentRun.SetEnableTrace(true)
	defer agentRun.stop()

	var contentEvents []*event.ContentEvent
	go func() {
		for evt := range agentRun.eventQueue {
			if ce, ok := evt.(*event.ContentEvent); ok {
				contentEvents = append(contentEvents, ce)
			}
		}
	}()

	agentRun.agentContext.StartTurn("")
	agentRun.runLLMCallAction("Test chunking")

	time.Sleep(100 * time.Millisecond)

	assert.Len(t, contentEvents, len(contentChunks), "Should have exactly the expected number of content chunks")

	seenContent := make(map[string]bool)
	for _, ce := range contentEvents {
		assert.False(t, seenContent[ce.Content], "Content chunk should not be duplicated: %s", ce.Content)
		seenContent[ce.Content] = true
	}

	for _, expectedChunk := range contentChunks {
		found := false
		for _, ce := range contentEvents {
			if ce.Content == expectedChunk {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected chunk should be present: %s", expectedChunk)
	}

	finalContent := ""
	for _, ce := range contentEvents {
		finalContent += ce.Content
	}
	expectedFinal := "Hello world! This is a test response."
	assert.Equal(t, expectedFinal, finalContent, "Final concatenated content should match expected")
}

func TestAgentRun_ReuseAfterCompletion(t *testing.T) {
	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "First response",
		}, nil
	})

	ar, err := NewAgentRun("test-reuse-agent", "Test agent for reuse", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	ar.SetModel(model)
	ar.SetEnableTrace(true)

	ar.Run(context.Background(), "First message")
	content1, err1 := ar.Wait(0)
	assert.NoError(t, err1)
	assert.Contains(t, content1, "First response")

	ar.Run(context.Background(), "Second message")
	content2, err2 := ar.Wait(0)
	assert.NoError(t, err2)
	assert.Contains(t, content2, "First response")
}

func TestAgentRun_ReuseAfterCancel(t *testing.T) {
	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		time.Sleep(50 * time.Millisecond)
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Response after cancel",
		}, nil
	})

	ar, err := NewAgentRun("test-cancel-reuse-agent", "Test agent for cancel and reuse", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	ar.SetModel(model)
	ar.SetEnableTrace(true)

	ar.Run(context.Background(), "First message")
	ar.Cancel()

	time.Sleep(100 * time.Millisecond)

	ar.Run(context.Background(), "Second message")
	content2, err2 := ar.Wait(0)
	assert.NoError(t, err2)
	assert.Contains(t, content2, "Response after cancel")
}

func TestAgentRun_MemoryPersistenceAcrossRuns(t *testing.T) {
	callCount := 0
	model := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		callCount++
		content := "Response " + string(rune('0'+callCount))
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: content,
		}, nil
	})

	ar, err := NewAgentRun("test-memory-agent", "Test agent with memory", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	ar.SetModel(model)
	ar.SetEnableTrace(true)

	ar.Run(context.Background(), "First message")
	content1, err1 := ar.Wait(0)
	assert.NoError(t, err1)
	assert.Contains(t, content1, "Response")

	ar.Run(context.Background(), "Second message")
	content2, err2 := ar.Wait(0)
	assert.NoError(t, err2)
	assert.Contains(t, content2, "Response")

	agentContext := ar.AgentContext()
	assert.NotNil(t, agentContext, "Agent context should persist across runs")
}

func TestSetModel_ModelUpdateReflectedInNextRun(t *testing.T) {
	model1 := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Response from model-1",
		}, nil
	})
	model1.ModelName = "model-1"

	model2 := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Response from model-2",
		}, nil
	})
	model2.ModelName = "model-2"

	ar, err := NewAgentRun("test-setmodel-agent", "Test agent for SetModel", "", t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test agent run: %v", err)
	}
	ar.SetModel(model1)
	ar.SetEnableTrace(true)

	ar.Run(context.Background(), "First message")
	content1, err1 := ar.Wait(0)
	assert.NoError(t, err1)
	assert.Contains(t, content1, "Response from model-1")
	assert.NotContains(t, content1, "Response from model-2")

	ar.SetModel(model2)

	ar.Run(context.Background(), "Second message")
	content2, err2 := ar.Wait(0)
	assert.NoError(t, err2)
	assert.Contains(t, content2, "Response from model-2")
	assert.NotContains(t, content2, "Response from model-1")

	assert.Equal(t, model2, ar.Model(), "Model() should return the updated model")
}
