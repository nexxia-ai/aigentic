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
	session := aigentic.NewSession(context.Background())
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory tool",
		Session:     session,
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
							Args: `{"memory_id": "test_entry", "memory_description": "Test entry", "memory_content": "test memory content"}`,
						},
					},
				}, nil
			}
			memories := extractMemoriesFromSystemPrompt(messages)
			if !strings.Contains(memories, "test_entry") {
				t.Error("Memory should appear in system prompt")
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
	session := aigentic.NewSession(context.Background())
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory tool",
		Session:     session,
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
							Args: `{"memory_id": "test_entry", "memory_description": "Test entry", "memory_content": "test memory content"}`,
						},
					},
				}, nil
			}
			if callCount == 2 {
				memories := extractMemoriesFromSystemPrompt(messages)
				if !strings.Contains(memories, "test_entry") {
					t.Error("Memory should appear in system prompt")
				}
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Now I'll delete it",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-2",
							Name: "update_memory",
							Args: `{"memory_id": "test_entry", "memory_description": "", "memory_content": ""}`,
						},
					},
				}, nil
			}
			if callCount == 3 {
				memories := extractMemoriesFromSystemPrompt(messages)
				if strings.Contains(memories, "test_entry") {
					t.Error("Memory should be deleted after setting description and content to empty strings")
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
}

func TestMemoryToolSystemPromptInjection(t *testing.T) {
	callCount := 0
	session := aigentic.NewSession(context.Background())
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory context",
		Session:     session,
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
							Args: `{"memory_id": "test_entry", "memory_description": "Test entry", "memory_content": "test memory content"}`,
						},
					},
				}, nil
			}
			memories := extractMemoriesFromSystemPrompt(messages)
			if memories == "" {
				t.Error("Memory content should be injected in system prompt")
			}
			if !strings.Contains(memories, "test_entry") {
				t.Error("Memory content should contain memory ID")
			}
			if !strings.Contains(memories, "Test entry") {
				t.Error("Memory content should contain description")
			}
			if !strings.Contains(memories, "test memory content") {
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
	session := aigentic.NewSession(context.Background())
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with multiple memory entries",
		Session:     session,
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
							Args: `{"memory_id": "entry1", "memory_description": "Entry 1", "memory_content": "content 1"}`,
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
							Args: `{"memory_id": "entry2", "memory_description": "Entry 2", "memory_content": "content 2"}`,
						},
					},
				}, nil
			}
			if callCount == 3 {
				memories := extractMemoriesFromSystemPrompt(messages)
				if !strings.Contains(memories, "entry1") {
					t.Error("Memory should contain entry1")
				}
				if !strings.Contains(memories, "content 1") {
					t.Error("Memory should contain content 1")
				}
				if !strings.Contains(memories, "entry2") {
					t.Error("Memory should contain entry2")
				}
				if !strings.Contains(memories, "content 2") {
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

func TestMemoryToolUpsert(t *testing.T) {
	callCount := 0
	session := aigentic.NewSession(context.Background())
	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory upsert",
		Session:     session,
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, toolList []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Saving entry",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "update_memory",
							Args: `{"memory_id": "entry1", "memory_description": "Original", "memory_content": "original content"}`,
						},
					},
				}, nil
			}
			if callCount == 2 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Updating entry",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-2",
							Name: "update_memory",
							Args: `{"memory_id": "entry1", "memory_description": "Updated", "memory_content": "updated content"}`,
						},
					},
				}, nil
			}
			if callCount == 3 {
				memoriesSection := extractMemoriesSection(messages)
				if !strings.Contains(memoriesSection, "entry1") {
					t.Error("Memory should contain entry1")
				}
				if !strings.Contains(memoriesSection, "Updated") {
					t.Error("Memory should contain updated description")
				}
				if !strings.Contains(memoriesSection, "updated content") {
					t.Error("Memory should contain updated content")
				}
				if strings.Contains(memoriesSection, "Original") {
					t.Error("Memory should not contain original description")
				}
				if strings.Contains(memoriesSection, "original content") {
					t.Error("Memory should not contain original content")
				}
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Upsert verified",
			}, nil
		}),
		AgentTools:  []aigentic.AgentTool{NewMemoryTool()},
		MaxLLMCalls: 10,
	}

	run, err := agent.Start("test upsert")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}
}

func TestMemoryToolSessionPersistence(t *testing.T) {
	session := aigentic.NewSession(context.Background())
	callCount := 0

	agent := aigentic.Agent{
		Name:        "test-agent",
		Description: "Test agent with memory persistence",
		Session:     session,
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, toolList []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Saving entry",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "update_memory",
							Args: `{"memory_id": "persistent", "memory_description": "Persistent entry", "memory_content": "persistent content"}`,
						},
					},
				}, nil
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Done",
			}, nil
		}),
		AgentTools:  []aigentic.AgentTool{NewMemoryTool()},
		MaxLLMCalls: 5,
	}

	run1, err := agent.Start("first run")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}
	_, err = run1.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("First run failed: %v", err)
	}

	callCount = 0
	agent.Model = ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, toolList []ai.Tool) (ai.AIMessage, error) {
		memories := extractMemoriesFromSystemPrompt(messages)
		if !strings.Contains(memories, "persistent") {
			t.Error("Memory should persist across runs")
		}
		if !strings.Contains(memories, "persistent content") {
			t.Error("Memory content should persist across runs")
		}
		return ai.AIMessage{
			Role:    ai.AssistantRole,
			Content: "Memory persisted",
		}, nil
	})

	run2, err := agent.Start("second run")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}
	_, err = run2.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Second run failed: %v", err)
	}
}

func extractMemoriesFromSystemPrompt(messages []ai.Message) string {
	for _, msg := range messages {
		if sysMsg, ok := msg.(ai.SystemMessage); ok {
			content := sysMsg.Content
			if strings.Contains(content, "<memories>") {
				return content
			}
		}
	}
	return ""
}

func extractMemoriesSection(messages []ai.Message) string {
	for _, msg := range messages {
		if sysMsg, ok := msg.(ai.SystemMessage); ok {
			content := sysMsg.Content
			startIdx := strings.Index(content, "<memories>")
			if startIdx == -1 {
				continue
			}
			endIdx := strings.Index(content[startIdx:], "</memories>")
			if endIdx == -1 {
				continue
			}
			return content[startIdx : startIdx+endIdx+len("</memories>")]
		}
	}
	return ""
}
