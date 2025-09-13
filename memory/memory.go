package memory

import (
	"fmt"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
)

// Memory represents the enhanced memory system with compartments
type Memory struct {
	store    *FileStore
	executor *MemoryToolExecutor
	config   *MemoryConfig
}

// NewMemory creates a new memory instance with default configuration
func NewMemory() *Memory {
	config := DefaultMemoryConfig()
	store := NewFileStore(config)
	executor := NewMemoryToolExecutor(store)

	return &Memory{
		store:    store,
		executor: executor,
		config:   config,
	}
}

// NewMemoryWithConfig creates a new memory instance with custom configuration
func NewMemoryWithConfig(config *MemoryConfig) *Memory {
	store := NewFileStore(config)
	executor := NewMemoryToolExecutor(store)

	return &Memory{
		store:    store,
		executor: executor,
		config:   config,
	}
}

// SystemPrompt returns the system prompt for memory tools
func (m *Memory) SystemPrompt() string {
	return `
The agent has access to a compartmentalized memory system with three types of memory:

1. **Run Memory**: Available in every LLM call during a single agent run
   - Use for: Current task state, progress tracking, keep state between tool calls, intermediate results
   - Scope: Single agent run (not shared with sub-agents)
   - Persistence: Cleared at the end of each agent run
   - Access: Automatically included in context (no tool needed)

2. **Session Memory**: Shared across multiple agent calls within a session
   - Use for: User preferences, session context, shared information between agents
   - Scope: Entire session (shared with sub-agents)
   - Persistence: Persists across agent runs within the same session
   - Access: Must be requested using get_memory tool

3. **Plan Memory**: For storing and tracking complex multi-step plans
   - Use for: Multi-step task plans, progress tracking, plan modifications
   - Scope: Can be run-level or session-level depending on plan scope
   - Persistence: Configurable (run-level or session-level)
   - Access: Must be requested using get_memory tool

You have access to three memory tools:
- **save_memory**: Save memory entries to specified compartment
- **get_memory**: Retrieve memory entries from specified compartment
- **clear_memory**: Clear memory entries from specified compartment

Note: Only Run Memory is automatically included in your context. To access Session or Plan memory, you must use the get_memory tool.
`
}

// GetRunMemoryContent returns formatted content from run memory for context
func (m *Memory) GetRunMemoryContent() string {
	entries, err := m.store.GetAll(RunMemory)
	if err != nil || len(entries) == 0 {
		return ""
	}

	return m.formatMemoryContent("Run Memory", entries)
}

// GetSessionMemoryContent returns formatted content from session memory for context
func (m *Memory) GetSessionMemoryContent() string {
	entries, err := m.store.GetAll(SessionMemory)
	if err != nil || len(entries) == 0 {
		return ""
	}

	return m.formatMemoryContent("Session Memory", entries)
}

// GetPlanMemoryContent returns formatted content from plan memory for context
func (m *Memory) GetPlanMemoryContent() string {
	entries, err := m.store.GetAll(PlanMemory)
	if err != nil || len(entries) == 0 {
		return ""
	}

	return m.formatMemoryContent("Plan Memory", entries)
}

// GetAllMemoryContent returns formatted content from all memory compartments
func (m *Memory) GetAllMemoryContent() string {
	var sections []string

	runContent := m.GetRunMemoryContent()
	if runContent != "" {
		sections = append(sections, runContent)
	}

	sessionContent := m.GetSessionMemoryContent()
	if sessionContent != "" {
		sections = append(sections, sessionContent)
	}

	planContent := m.GetPlanMemoryContent()
	if planContent != "" {
		sections = append(sections, planContent)
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}

// formatMemoryContent formats memory entries for display
func (m *Memory) formatMemoryContent(title string, entries []*MemoryEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("## %s\n", title))

	for i, entry := range entries {
		if i > 0 {
			content.WriteString("\n")
		}
		content.WriteString(fmt.Sprintf("**%s**", entry.Content))
		if entry.Category != "" {
			content.WriteString(fmt.Sprintf(" (Category: %s)", entry.Category))
		}
		if len(entry.Tags) > 0 {
			content.WriteString(fmt.Sprintf(" [Tags: %s]", strings.Join(entry.Tags, ", ")))
		}
		content.WriteString(fmt.Sprintf(" - Priority: %d", entry.Priority))
	}

	return content.String()
}

// ClearRunMemory clears all run memory (called at end of agent run)
func (m *Memory) ClearRunMemory() error {
	return m.store.Clear(RunMemory, "", nil)
}

// GetTools returns the memory tools for the agent
func (m *Memory) GetTools() []ai.Tool {
	return GetMemoryTools(m.executor)
}

// SaveEntry saves a memory entry directly (for testing purposes)
func (m *Memory) SaveEntry(compartment MemoryCompartment, entry *MemoryEntry) error {
	return m.store.Save(compartment, entry)
}
