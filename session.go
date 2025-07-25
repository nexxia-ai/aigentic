package aigentic

import (
	"context"
	"log/slog"
	"os"
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

	Logger   *slog.Logger
	LogLevel slog.Level
	Context  context.Context
	Trace    *Trace

	RunHistory []AgentRun
}

// NewSession creates a new session with default settings
func NewSession() *Session {
	s := &Session{
		ID:        uuid.New().String(),
		Context:   context.Background(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		State:     make(map[string]interface{}),
		Logger:    slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}
	return s
}
