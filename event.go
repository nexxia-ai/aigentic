package aigentic

import (
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

// Event interface identify types that can be sent to the event channel
// Events are used to notify the caller of the execution of the agent's actions.
// For example, when the agent calls a tool, a ToolEvent is sent to the event channel.
//
// The caller will typically use a switch statement to handle the event type.
// For example:
//
//	 for event := range run.Next() {
//			switch ev := event.(type) {
//			case *LLMCallEvent:
//				fmt.Println(ev.Message)
//			case *ContentEvent:
//				fmt.Println(ev.Content)
//			case *ToolEvent:
//				fmt.Println(ev.ToolName)
//			case *ToolResponseEvent:
//				fmt.Println(ev.Content)
//			case *ErrorEvent:
//				fmt.Println(ev.Err)
//			case *ApprovalEvent:
//				fmt.Println(ev.ToolName)
//			case *ThinkingEvent:
//				fmt.Println(ev.Thought)
//			}
//		}
type Event interface {
	ID() string
}

// LLMCallEvent is sent when the agent calls an LLM.
type LLMCallEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Message   string
	Tools     []ai.Tool
}

func (e *LLMCallEvent) ID() string { return e.RunID }

// ContentEvent is sent when the agent receives a response from the LLM.
// When streaming is enabled, the agent will receive a ContentEvent for each chunk of the response.
type ContentEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Content   string
}

func (e *ContentEvent) ID() string { return e.RunID }

// ToolResponseEvent is sent when the agent receives a response from a tool.
type ToolResponseEvent struct {
	RunID      string
	AgentName  string
	SessionID  string
	ToolCallID string
	ToolName   string
	Content    string
}

func (e *ToolResponseEvent) ID() string { return e.RunID }

// ToolEvent is sent when the agent calls a tool.
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

// ThinkingEvent is sent when the agent is receiving thinking output from the LLM.
// When streaming is enabled, the agent will receive a ThinkingEvent for each chunk of the thinking.
type ThinkingEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Thought   string
}

func (e *ThinkingEvent) ID() string { return e.RunID }

// ErrorEvent is sent when the agent encounters an error.
type ErrorEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Err       error
}

func (e *ErrorEvent) ID() string { return e.RunID }

// ApprovalEvent is sent when the agent needs to approve an action.
type ApprovalEvent struct {
	RunID            string
	ApprovalID       string
	ToolName         string
	ValidationResult ValidationResult
}

func (e *ApprovalEvent) ID() string { return e.RunID }

// EvalEvent is sent when the agent receives a response from the LLM.
// It contains the raw ai.Messages sent and received from the LLM.
// This can be used to evaluate the agent's prompt performance using the eval package.
type EvalEvent struct {
	RunID     string
	EventID   string
	AgentName string
	SessionID string
	CallID    string
	Sequence  int
	Timestamp time.Time
	Duration  time.Duration

	// LLM Call Data
	Messages []ai.Message
	Tools    []ai.Tool
	Response ai.AIMessage
	Error    error

	// Metadata
	ModelName string
	TokensIn  int
	TokensOut int
}

func (e *EvalEvent) ID() string { return e.RunID }
