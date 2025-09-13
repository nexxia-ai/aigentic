package memory

import (
	"time"
)

// MemoryCompartment represents the different types of memory compartments
type MemoryCompartment string

const (
	RunMemory     MemoryCompartment = "run"
	SessionMemory MemoryCompartment = "session"
	PlanMemory    MemoryCompartment = "plan"
)

// MemoryEntry represents a single memory entry with metadata
type MemoryEntry struct {
	ID          string            `json:"id"`
	Content     string            `json:"content"`
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	AccessCount int               `json:"access_count"`
	Priority    int               `json:"priority"` // 1-10, higher = more important
}

// MemoryConfig holds configuration for memory system
type MemoryConfig struct {
	MaxSizePerCompartment int
	StoragePath           string
}

// DefaultMemoryConfig returns default configuration
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		MaxSizePerCompartment: 10000, // 10k characters
		StoragePath:           "memory.json",
	}
}
