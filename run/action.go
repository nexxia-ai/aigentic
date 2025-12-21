package run

import "github.com/nexxia-ai/aigentic/event"

// action is a marker interface for internal agent actions.
// Types implement this interface by defining the unexported isAction method.
type action interface {
	isAction()
}

type llmCallAction struct {
	Message string
}

func (*llmCallAction) isAction() {}

type approvalAction struct {
	ApprovalID string
	Approved   bool
}

func (*approvalAction) isAction() {}

type toolCallAction struct {
	ToolCallID       string
	ToolName         string
	ValidationResult event.ValidationResult
	Group            *ToolCallGroup
}

func (*toolCallAction) isAction() {}

type toolResponseAction struct {
	request  *toolCallAction
	response string
}

func (*toolResponseAction) isAction() {}

type stopAction struct {
	Error error
}

func (*stopAction) isAction() {}
