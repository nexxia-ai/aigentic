package memory

import (
	"fmt"
	"strings"

	"github.com/nexxia-ai/aigentic/ai"
)

// SaveMemoryParams represents parameters for save_memory tool
type SaveMemoryParams struct {
	Compartment string            `json:"compartment"`
	Content     string            `json:"content"`
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Priority    int               `json:"priority,omitempty"`
}

// GetMemoryParams represents parameters for get_memory tool
type GetMemoryParams struct {
	Compartment string   `json:"compartment"`
	ID          string   `json:"id,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// ClearMemoryParams represents parameters for clear_memory tool
type ClearMemoryParams struct {
	Compartment string   `json:"compartment"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// MemoryToolExecutor handles memory tool operations
type MemoryToolExecutor struct {
	store *FileStore
}

// NewMemoryToolExecutor creates a new memory tool executor
func NewMemoryToolExecutor(store *FileStore) *MemoryToolExecutor {
	return &MemoryToolExecutor{
		store: store,
	}
}

// executeSaveMemory executes the save_memory tool
func (e *MemoryToolExecutor) executeSaveMemory(params SaveMemoryParams) (*ai.ToolResult, error) {
	// Validate compartment
	compartment := MemoryCompartment(params.Compartment)
	if compartment != RunMemory && compartment != SessionMemory && compartment != PlanMemory {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "Invalid compartment. Must be 'run', 'session', or 'plan'",
			}},
			Error: true,
		}, nil
	}

	// Validate content
	if strings.TrimSpace(params.Content) == "" {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "Content cannot be empty",
			}},
			Error: true,
		}, nil
	}

	// Set default priority if not provided
	priority := params.Priority
	if priority == 0 {
		priority = 5 // Default priority
	}

	// Create memory entry
	entry := NewMemoryEntry(params.Content, params.Category, params.Tags, params.Metadata, priority)

	// Save to store
	if err := e.store.Save(compartment, entry); err != nil {
		errorMsg := fmt.Sprintf("Failed to save memory: %v", err)
		// If it's a size limit error, provide specific guidance
		if strings.Contains(err.Error(), "size limit exceeded") {
			errorMsg = fmt.Sprintf("Memory compartment size limit exceeded (%d characters). Please delete some entries using clear_memory tool before saving new content.", e.store.GetMaxSize())
		}
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: errorMsg,
			}},
			Error: true,
		}, nil
	}

	return &ai.ToolResult{
		Content: []ai.ToolContent{{
			Type:    "text",
			Content: fmt.Sprintf("Memory saved successfully with ID: %s", entry.ID),
		}},
		Error: false,
	}, nil
}

// executeGetMemory executes the get_memory tool
func (e *MemoryToolExecutor) executeGetMemory(params GetMemoryParams) (*ai.ToolResult, error) {
	// Validate compartment
	compartment := MemoryCompartment(params.Compartment)
	if compartment != RunMemory && compartment != SessionMemory && compartment != PlanMemory {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "Invalid compartment. Must be 'run', 'session', or 'plan'",
			}},
			Error: true,
		}, nil
	}

	var entries []*MemoryEntry
	var err error

	if params.ID != "" {
		// Get specific entry by ID
		entry, err := e.store.Get(compartment, params.ID)
		if err != nil {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: fmt.Sprintf("Memory entry not found: %v", err),
				}},
				Error: true,
			}, nil
		}
		entries = []*MemoryEntry{entry}
	} else {
		// Search by category and tags
		entries, err = e.store.Search(compartment, params.Category, params.Tags)
		if err != nil {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: fmt.Sprintf("Failed to search memory: %v", err),
				}},
				Error: true,
			}, nil
		}
	}

	if len(entries) == 0 {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "No memory entries found",
			}},
			Error: false,
		}, nil
	}

	// Format results
	var result strings.Builder
	for i, entry := range entries {
		if i > 0 {
			result.WriteString("\n\n---\n\n")
		}
		result.WriteString(fmt.Sprintf("ID: %s\n", entry.ID))
		result.WriteString(fmt.Sprintf("Content: %s\n", entry.Content))
		if entry.Category != "" {
			result.WriteString(fmt.Sprintf("Category: %s\n", entry.Category))
		}
		if len(entry.Tags) > 0 {
			result.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(entry.Tags, ", ")))
		}
		result.WriteString(fmt.Sprintf("Priority: %d\n", entry.Priority))
		result.WriteString(fmt.Sprintf("Created: %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05")))
		result.WriteString(fmt.Sprintf("Updated: %s\n", entry.UpdatedAt.Format("2006-01-02 15:04:05")))
		result.WriteString(fmt.Sprintf("Access Count: %d", entry.AccessCount))
	}

	return &ai.ToolResult{
		Content: []ai.ToolContent{{
			Type:    "text",
			Content: result.String(),
		}},
		Error: false,
	}, nil
}

// executeClearMemory executes the clear_memory tool
func (e *MemoryToolExecutor) executeClearMemory(params ClearMemoryParams) (*ai.ToolResult, error) {
	// Validate compartment
	compartment := MemoryCompartment(params.Compartment)
	if compartment != RunMemory && compartment != SessionMemory && compartment != PlanMemory {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: "Invalid compartment. Must be 'run', 'session', or 'plan'",
			}},
			Error: true,
		}, nil
	}

	// Clear memory entries
	if err := e.store.Clear(compartment, params.Category, params.Tags); err != nil {
		return &ai.ToolResult{
			Content: []ai.ToolContent{{
				Type:    "text",
				Content: fmt.Sprintf("Failed to clear memory: %v", err),
			}},
			Error: true,
		}, nil
	}

	message := fmt.Sprintf("Memory cleared successfully for compartment '%s'", params.Compartment)
	if params.Category != "" {
		message += fmt.Sprintf(" with category '%s'", params.Category)
	}
	if len(params.Tags) > 0 {
		message += fmt.Sprintf(" with tags: %s", strings.Join(params.Tags, ", "))
	}

	return &ai.ToolResult{
		Content: []ai.ToolContent{{
			Type:    "text",
			Content: message,
		}},
		Error: false,
	}, nil
}

// GetMemoryTools returns the memory tools as ai.Tool instances
func GetMemoryTools(executor *MemoryToolExecutor) []ai.Tool {
	return []ai.Tool{
		{
			Name: "save_memory",
			Description: `
This tool saves memory entries to specified compartments for persistent memory across multi-step tasks.

Compartments:
- **run**: Memory available in every LLM call during a single agent run with multiple tool calls (cleared at run end)
- **session**: Memory shared across multiple agent calls within a session (persists across runs)
- **plan**: Memory for storing and tracking complex multi-step plans

When to use:
- Save current task state and progress
- Store intermediate results and calculations
- Record user preferences and settings
- Store multi-step plans and strategies
- Save important context for future agent calls

Parameters:
- compartment: "run", "session", or "plan"
- content: The information to save
- category: Optional category for organization
- tags: Optional tags for flexible organization
- metadata: Optional key-value metadata
- priority: Priority level 1-10 (default: 5)
`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"compartment": map[string]interface{}{
						"type":        "string",
						"description": "Memory compartment: 'run', 'session', or 'plan'",
						"enum":        []string{"run", "session", "plan"},
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to save to memory",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Optional category for organization",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "Optional tags for flexible organization",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Optional key-value metadata",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "Priority level 1-10 (default: 5)",
						"minimum":     1,
						"maximum":     10,
					},
				},
				"required": []string{"compartment", "content"},
			},
			Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
				params := SaveMemoryParams{}

				if compartment, ok := args["compartment"].(string); ok {
					params.Compartment = compartment
				} else {
					return &ai.ToolResult{
						Content: []ai.ToolContent{{
							Type:    "text",
							Content: "compartment is required",
						}},
						Error: true,
					}, nil
				}

				if content, ok := args["content"].(string); ok {
					params.Content = content
				} else {
					return &ai.ToolResult{
						Content: []ai.ToolContent{{
							Type:    "text",
							Content: "content is required",
						}},
						Error: true,
					}, nil
				}

				if category, ok := args["category"].(string); ok {
					params.Category = category
				}

				if tags, ok := args["tags"].([]interface{}); ok {
					params.Tags = make([]string, len(tags))
					for i, tag := range tags {
						if tagStr, ok := tag.(string); ok {
							params.Tags[i] = tagStr
						}
					}
				}

				if metadata, ok := args["metadata"].(map[string]interface{}); ok {
					params.Metadata = make(map[string]string)
					for k, v := range metadata {
						if vStr, ok := v.(string); ok {
							params.Metadata[k] = vStr
						}
					}
				}

				if priority, ok := args["priority"].(float64); ok {
					params.Priority = int(priority)
				}

				return executor.executeSaveMemory(params)
			},
		},
		{
			Name: "get_memory",
			Description: `
This tool retrieves memory entries from specified compartments.

Compartments:
- **run**: Memory available in every LLM call during a single agent run with multiple tool calls
- **session**: Memory shared across multiple agent calls within a session
- **plan**: Memory for storing and tracking complex multi-step plans

You can retrieve:
- All entries from a compartment
- Specific entry by ID
- Entries matching a category
- Entries with specific tags

Parameters:
- compartment: "run", "session", or "plan"
- id: Optional specific entry ID
- category: Optional category filter
- tags: Optional tags filter
`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"compartment": map[string]interface{}{
						"type":        "string",
						"description": "Memory compartment: 'run', 'session', or 'plan'",
						"enum":        []string{"run", "session", "plan"},
					},
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Optional specific entry ID",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Optional category filter",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "Optional tags filter",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"compartment"},
			},
			Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
				params := GetMemoryParams{}

				if compartment, ok := args["compartment"].(string); ok {
					params.Compartment = compartment
				} else {
					return &ai.ToolResult{
						Content: []ai.ToolContent{{
							Type:    "text",
							Content: "compartment is required",
						}},
						Error: true,
					}, nil
				}

				if id, ok := args["id"].(string); ok {
					params.ID = id
				}

				if category, ok := args["category"].(string); ok {
					params.Category = category
				}

				if tags, ok := args["tags"].([]interface{}); ok {
					params.Tags = make([]string, len(tags))
					for i, tag := range tags {
						if tagStr, ok := tag.(string); ok {
							params.Tags[i] = tagStr
						}
					}
				}

				return executor.executeGetMemory(params)
			},
		},
		{
			Name: "clear_memory",
			Description: `
This tool clears memory entries from specified compartments.

Compartments:
- **run**: Memory available in every LLM call during a single agent with multiple tool calls
- **session**: Memory shared across multiple agent calls within a session
- **plan**: Memory for storing and tracking complex multi-step plans

You can clear:
- All entries from a compartment
- Entries matching a category
- Entries with specific tags

Use with caution as this action cannot be undone.

Parameters:
- compartment: "run", "session", or "plan"
- category: Optional category filter
- tags: Optional tags filter
`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"compartment": map[string]interface{}{
						"type":        "string",
						"description": "Memory compartment: 'run', 'session', or 'plan'",
						"enum":        []string{"run", "session", "plan"},
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Optional category filter",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "Optional tags filter",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"compartment"},
			},
			Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
				params := ClearMemoryParams{}

				if compartment, ok := args["compartment"].(string); ok {
					params.Compartment = compartment
				} else {
					return &ai.ToolResult{
						Content: []ai.ToolContent{{
							Type:    "text",
							Content: "compartment is required",
						}},
						Error: true,
					}, nil
				}

				if category, ok := args["category"].(string); ok {
					params.Category = category
				}

				if tags, ok := args["tags"].([]interface{}); ok {
					params.Tags = make([]string, len(tags))
					for i, tag := range tags {
						if tagStr, ok := tag.(string); ok {
							params.Tags[i] = tagStr
						}
					}
				}

				return executor.executeClearMemory(params)
			},
		},
	}
}
