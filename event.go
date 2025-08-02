package aigentic

import "github.com/nexxia-ai/aigentic/ai"

type Event interface {
	ID() string
}

type LLMCallEvent struct {
	EventID   string
	AgentName string
	SessionID string
	Message   string
	Tools     []ai.Tool
}

func (e *LLMCallEvent) ID() string { return e.EventID }

type ContentEvent struct {
	EventID   string
	AgentName string
	SessionID string
	Content   string
	IsChunk   bool
}

func (e *ContentEvent) ID() string { return e.EventID }

type ToolResponseEvent struct {
	EventID    string
	AgentName  string
	SessionID  string
	ToolCallID string
	ToolName   string
	Content    string
}

func (e *ToolResponseEvent) ID() string { return e.EventID }

type ToolEvent struct {
	EventID         string
	AgentName       string
	SessionID       string
	ToolName        string
	ToolArgs        map[string]interface{}
	ToolGroup       *toolCallGroup
	RequireApproval bool
	Approved        bool
	Result          interface{}
	Error           error
}

func (e *ToolEvent) ID() string { return e.EventID }

type ThinkingEvent struct {
	EventID   string
	AgentName string
	SessionID string
	Thought   string
}

func (e *ThinkingEvent) ID() string { return e.EventID }

type ErrorEvent struct {
	EventID   string
	AgentName string
	SessionID string
	Err       error
}

func (e *ErrorEvent) ID() string { return e.EventID }
