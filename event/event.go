package event

import (
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
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
//			case *ThinkingEvent:
//				fmt.Println(ev.Thought)
//			}
//		}
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
}

func (e *ContentEvent) ID() string { return e.RunID }

type ToolResponseEvent struct {
	RunID      string
	AgentName  string
	SessionID  string
	ToolCallID string
	ToolName   string
	Content    string
	Documents  []*document.Document
}

func (e *ToolResponseEvent) ID() string { return e.RunID }

type ToolEvent struct {
	RunID     string
	EventID   string
	AgentName string
	SessionID string
	ToolName  string
	Args      map[string]any
	ToolGroup interface{}
	Result    interface{}
	Error     error
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

type EvalEvent struct {
	RunID     string
	AgentName string
	SessionID string
	Sequence  int
	Timestamp time.Time
	Duration  time.Duration

	Messages []ai.Message
	Tools    []ai.Tool
	Response ai.AIMessage
	Error    error

	ModelName string
	TokensIn  int
	TokensOut int
}

func (e *EvalEvent) ID() string { return e.RunID }
