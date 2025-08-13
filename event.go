package aigentic

import "github.com/nexxia-ai/aigentic/ai"

type Event interface {
	ID() string
}

type LLMCallEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Message   string
	Tools     []ai.Tool
}

func (e *LLMCallEvent) ID() string { return e.RunID }

type ContentEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Content   string
	IsChunk   bool
}

func (e *ContentEvent) ID() string { return e.RunID }

type ToolResponseEvent struct {
	RunID      string
	AgentName  string
	SessionID  string
	ToolCallID string
	ToolName   string
	Content    string
}

func (e *ToolResponseEvent) ID() string { return e.RunID }

type ToolEvent struct {
	RunID            string
	EventID          string
	AgentName        string
	SessionID        string
	ToolName         string
	ValidationResult ValidationResult
	ToolGroup        *toolCallGroup
	Approved         bool
	Result           interface{}
	Error            error
}

func (e *ToolEvent) ID() string { return e.RunID }

type ThinkingEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Thought   string
}

func (e *ThinkingEvent) ID() string { return e.RunID }

type ErrorEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Err       error
}

func (e *ErrorEvent) ID() string { return e.RunID }

type ApprovalEvent struct {
	RunID            string
	ApprovalID       string
	ToolName         string
	ValidationResult ValidationResult
}

func (e *ApprovalEvent) ID() string { return e.RunID }
