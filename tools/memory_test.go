package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func TestMemoryToolBasicUsage(t *testing.T) {
	callCount := 0
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory tool",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, toolList []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll save this to memory",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "update_memory",
							Args: `{"memory_name": "test_entry", "memory_content": "test memory content"}`,
						},
					},
				}, nil
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Memory saved successfully",
			}, nil
		}),
		AgentTools:  []aigentic.AgentTool{NewMemoryTool()},
		MaxLLMCalls: 5,
	}

	run, err := agent.Start("test message")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	content, err := run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}

	if content == "" {
		t.Error("Agent should return content")
	}
}

func TestMemoryToolDelete(t *testing.T) {
	callCount := 0
	var savedMemory string
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory tool",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, toolList []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll save this to memory",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "update_memory",
							Args: `{"memory_name": "test_entry", "memory_content": "test memory content"}`,
						},
					},
				}, nil
			}
			if callCount == 2 {
				savedMemory = extractMemoryFromMessages(messages)
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Now I'll delete it",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-2",
							Name: "update_memory",
							Args: `{"memory_name": "test_entry", "memory_content": ""}`,
						},
					},
				}, nil
			}
			if callCount == 3 {
				deletedMemory := extractMemoryFromMessages(messages)
				if deletedMemory != "" {
					t.Error("Memory should be deleted after setting content to empty string")
				}
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Delete complete",
			}, nil
		}),
		AgentTools:  []aigentic.AgentTool{NewMemoryTool()},
		MaxLLMCalls: 10,
	}

	run, err := agent.Start("test message")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}

	if savedMemory == "" {
		t.Error("Memory should have been saved")
	}
}

func TestMemoryToolContextInjection(t *testing.T) {
	callCount := 0
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory context",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, toolList []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll save this to memory",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "update_memory",
							Args: `{"memory_name": "test_entry", "memory_content": "test memory content"}`,
						},
					},
				}, nil
			}
			memoryContent := extractMemoryFromMessages(messages)
			if memoryContent == "" {
				t.Error("Memory content should be injected via ContextFunction")
			}
			if !strings.Contains(memoryContent, "Memory: test_entry") {
				t.Error("Memory content should contain proper header")
			}
			if !strings.Contains(memoryContent, "test memory content") {
				t.Error("Memory content should contain the saved data")
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Context verified",
			}, nil
		}),
		AgentTools:  []aigentic.AgentTool{NewMemoryTool()},
		MaxLLMCalls: 5,
	}

	run, err := agent.Start("test context")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}
}

func TestMemoryToolMultipleEntries(t *testing.T) {
	callCount := 0
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with multiple memory entries",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, toolList []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Saving first entry",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "update_memory",
							Args: `{"memory_name": "entry1", "memory_content": "content 1"}`,
						},
					},
				}, nil
			}
			if callCount == 2 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Saving second entry",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-2",
							Name: "update_memory",
							Args: `{"memory_name": "entry2", "memory_content": "content 2"}`,
						},
					},
				}, nil
			}
			if callCount == 3 {
				memoryContent := extractMemoryFromMessages(messages)
				if !strings.Contains(memoryContent, "Memory: entry1") {
					t.Error("Memory should contain entry1")
				}
				if !strings.Contains(memoryContent, "content 1") {
					t.Error("Memory should contain content 1")
				}
				if !strings.Contains(memoryContent, "Memory: entry2") {
					t.Error("Memory should contain entry2")
				}
				if !strings.Contains(memoryContent, "content 2") {
					t.Error("Memory should contain content 2")
				}
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Multiple entries verified",
			}, nil
		}),
		AgentTools:  []aigentic.AgentTool{NewMemoryTool()},
		MaxLLMCalls: 10,
	}

	run, err := agent.Start("test multiple entries")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}
}

func extractMemoryFromMessages(messages []ai.Message) string {
	for _, msg := range messages {
		if userMsg, ok := msg.(ai.UserMessage); ok {
			content := userMsg.Content
			if strings.Contains(content, "Memory:") {
				return content
			}
		}
	}
	return ""
}
