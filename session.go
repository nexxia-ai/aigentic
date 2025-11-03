package aigentic

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryEntry represents a single memory entry
type MemoryEntry struct {
	ID          string
	Description string
	Content     string
	Scope       string
	RunID       string
	Timestamp   time.Time
}

// Session represents a shared session between agents and teams
type Session struct {
	// Core session identifiers
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Session metadata
	Description string
	Tags        []string

	// Session state and memory
	State map[string]interface{}

	// Thread-safe memory storage
	mutex    sync.RWMutex
	memories []MemoryEntry

	Context    context.Context
	cancelFunc context.CancelFunc
}

// NewSession creates a new session with default settings
func NewSession(ctx context.Context) *Session {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{
		ID:         uuid.New().String(),
		Context:    ctx,
		cancelFunc: cancel,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		State:      make(map[string]interface{}),
	}
	return s
}

func (h *Session) Cancel() {
	h.cancelFunc()
}

// AddMemory adds a new memory entry or updates an existing one by ID
func (s *Session) AddMemory(id, description, content, scope, runID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	for i := range s.memories {
		if s.memories[i].ID == id {
			s.memories[i].Description = description
			s.memories[i].Content = content
			s.memories[i].Scope = scope
			s.memories[i].RunID = runID
			s.memories[i].Timestamp = now
			return nil
		}
	}

	s.memories = append(s.memories, MemoryEntry{
		ID:          id,
		Description: description,
		Content:     content,
		Scope:       scope,
		RunID:       runID,
		Timestamp:   now,
	})
	return nil
}

// DeleteMemory removes a memory entry by ID
func (s *Session) DeleteMemory(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i := range s.memories {
		if s.memories[i].ID == id {
			s.memories = append(s.memories[:i], s.memories[i+1:]...)
			return nil
		}
	}
	return nil
}

// GetMemories returns all memories in insertion order
func (s *Session) GetMemories() []MemoryEntry {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]MemoryEntry, len(s.memories))
	copy(result, s.memories)
	return result
}
