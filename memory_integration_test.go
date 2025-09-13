package aigentic

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/memory"
)

func TestMemoryIntegration(t *testing.T) {
	// Create agent with memory
	callCount := 0
	agent := Agent{
		Name:        "test-agent",
		Description: "Test agent with memory",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				// Simulate LLM response with memory tool calls
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "I'll save this to memory",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "save_memory",
							Args: `{"compartment": "session", "content": "test memory content", "category": "test"}`,
						},
					},
				}, nil
			}
			// Return final response without tool calls
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Memory saved successfully",
			}, nil
		}),
		Memory:      memory.NewMemory(),
		MaxLLMCalls: 5, // Limit LLM calls to prevent infinite loops
	}

	// Start agent run
	run, err := agent.Start("test message")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	// Wait for completion
	content, err := run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}

	if content == "" {
		t.Error("Agent should return content")
	}

	// Verify memory was saved
	memContent := agent.Memory.GetSessionMemoryContent()
	if memContent == "" {
		t.Error("Session memory should contain saved content")
	}
}

func TestMemoryCompartmentLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	config := &memory.MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	callCount := 0
	agent := Agent{
		Name:        "test-agent",
		Description: "Test agent with memory",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Memory saved",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "save_memory",
							Args: `{"compartment": "run", "content": "run memory content", "category": "run"}`,
						},
						{
							ID:   "call-2",
							Name: "save_memory",
							Args: `{"compartment": "session", "content": "session memory content", "category": "session"}`,
						},
					},
				}, nil
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Task completed",
			}, nil
		}),
		Memory:      memory.NewMemoryWithConfig(config),
		MaxLLMCalls: 5,
	}

	// First run
	run1, err := agent.Start("first run")
	if err != nil {
		t.Errorf("Failed to start first run: %v", err)
	}

	_, err = run1.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("First run failed: %v", err)
	}

	// Verify both memories exist
	sessionContent := agent.Memory.GetSessionMemoryContent()

	if sessionContent == "" {
		t.Error("Session memory should contain content after first run")
	}

	// Second run
	run2, err := agent.Start("second run")
	if err != nil {
		t.Errorf("Failed to start second run: %v", err)
	}

	_, err = run2.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Second run failed: %v", err)
	}

	// Verify run memory was cleared but session memory persists
	runContentAfter := agent.Memory.GetRunMemoryContent()
	sessionContentAfter := agent.Memory.GetSessionMemoryContent()

	if runContentAfter != "" {
		t.Error("Run memory should be cleared after second run")
	}
	if sessionContentAfter == "" {
		t.Error("Session memory should persist after second run")
	}
}

func TestMemoryToolsIntegration(t *testing.T) {
	agent := Agent{
		Name:        "test-agent",
		Description: "Test agent with memory tools",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			// Check if memory tools are available
			toolNames := make([]string, len(tools))
			for i, tool := range tools {
				toolNames[i] = tool.Name
			}

			// Verify memory tools are present
			expectedTools := []string{"save_memory", "get_memory", "clear_memory"}
			for _, expected := range expectedTools {
				found := false
				for _, toolName := range toolNames {
					if toolName == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected tool %s not found in tools: %v", expected, toolNames)
				}
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Tools verified",
			}, nil
		}),
		Memory: memory.NewMemory(),
	}

	run, err := agent.Start("test tools")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}
}

func TestMemoryContextIntegration(t *testing.T) {
	agent := Agent{
		Name:        "test-agent",
		Description: "Test agent with memory context",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			// Check if memory content is included in context
			for _, msg := range messages {
				if userMsg, ok := msg.(ai.UserMessage); ok {
					if userMsg.Content == "" {
						t.Error("User message content should not be empty")
					}
				}
			}

			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Context verified",
			}, nil
		}),
		Memory: memory.NewMemory(),
	}

	// Add some memory content by creating a memory entry directly
	entry := memory.NewMemoryEntry("test context content", "test", nil, nil, 5)
	agent.Memory.SaveEntry(memory.SessionMemory, entry)

	run, err := agent.Start("test context")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}
}

func TestMemoryErrorHandling(t *testing.T) {
	callCount := 0
	agent := Agent{
		Name:        "test-agent",
		Description: "Test agent with memory error handling",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Testing error handling",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "save_memory",
							Args: `{"compartment": "invalid", "content": "test"}`,
						},
					},
				}, nil
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Error handled",
			}, nil
		}),
		Memory:      memory.NewMemory(),
		MaxLLMCalls: 5,
	}

	run, err := agent.Start("test error handling")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}

	// The agent should handle the error gracefully and continue
}

func TestMemorySizeLimit(t *testing.T) {
	tempDir := t.TempDir()
	config := &memory.MemoryConfig{
		MaxSizePerCompartment: 100, // Very small limit
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	callCount := 0
	agent := Agent{
		Name:        "test-agent",
		Description: "Test agent with memory size limit",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount++
			if callCount == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Testing size limit",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "save_memory",
							Args: `{"compartment": "run", "content": "This is a very long content that should exceed the size limit and cause an error when trying to save to memory"}`,
						},
					},
				}, nil
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Size limit handled",
			}, nil
		}),
		Memory:      memory.NewMemoryWithConfig(config),
		MaxLLMCalls: 5,
	}

	run, err := agent.Start("test size limit")
	if err != nil {
		t.Errorf("Failed to start agent: %v", err)
	}

	_, err = run.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Agent run failed: %v", err)
	}

	// The agent should handle the size limit error gracefully
}

func TestMemoryPersistenceAcrossSessions(t *testing.T) {
	tempDir := t.TempDir()
	config := &memory.MemoryConfig{
		MaxSizePerCompartment: 1000,
		StoragePath:           filepath.Join(tempDir, "test_memory.json"),
	}

	// First session
	callCount1 := 0
	agent1 := Agent{
		Name:        "test-agent-1",
		Description: "Test agent session 1",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount1++
			if callCount1 == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Session 1 complete",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "save_memory",
							Args: `{"compartment": "session", "content": "persistent session data", "category": "session"}`,
						},
					},
				}, nil
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Session 1 done",
			}, nil
		}),
		Memory:      memory.NewMemoryWithConfig(config),
		MaxLLMCalls: 5,
	}

	run1, err := agent1.Start("session 1")
	if err != nil {
		t.Errorf("Failed to start first session: %v", err)
	}

	_, err = run1.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("First session failed: %v", err)
	}

	// Second session with new agent instance
	callCount2 := 0
	agent2 := Agent{
		Name:        "test-agent-2",
		Description: "Test agent session 2",
		Model: ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
			callCount2++
			if callCount2 == 1 {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "Session 2 complete",
					ToolCalls: []ai.ToolCall{
						{
							ID:   "call-1",
							Name: "get_memory",
							Args: `{"compartment": "session", "category": "session"}`,
						},
					},
				}, nil
			}
			return ai.AIMessage{
				Role:    ai.AssistantRole,
				Content: "Session 2 done",
			}, nil
		}),
		Memory:      memory.NewMemoryWithConfig(config),
		MaxLLMCalls: 5,
	}

	run2, err := agent2.Start("session 2")
	if err != nil {
		t.Errorf("Failed to start second session: %v", err)
	}

	_, err = run2.Wait(5 * time.Second)
	if err != nil {
		t.Errorf("Second session failed: %v", err)
	}

	// Verify session memory persisted
	sessionContent := agent2.Memory.GetSessionMemoryContent()
	if sessionContent == "" {
		t.Error("Session memory should persist across agent instances")
	}
}
