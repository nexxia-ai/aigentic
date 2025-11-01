package aigentic

import (
	"context"
	"time"

	"github.com/google/uuid"
)

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

	Context    context.Context
	cancelFunc context.CancelFunc

	RunHistory []AgentRun
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
