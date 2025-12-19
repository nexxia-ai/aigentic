package aigentic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/document"
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

	documents []*document.Document

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
		documents:  make([]*document.Document, 0),
	}
	return s
}

func (h *Session) Cancel() {
	h.cancelFunc()
}
