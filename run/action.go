package run

import "github.com/nexxia-ai/aigentic/ctxt"

// action is a marker interface for internal agent actions.
// Types implement this interface by defining the unexported isAction method.
type action interface {
	isAction()
}

type llmCallAction struct {
	Message string
}

func (*llmCallAction) isAction() {}

type toolCallAction struct {
	ToolCallID string
	ToolName   string
	Args       map[string]any
	Group      *ToolCallGroup
}

func (*toolCallAction) isAction() {}

type toolResponseAction struct {
	request  *toolCallAction
	response string
	fileRefs []ctxt.FileRefEntry
}

func (*toolResponseAction) isAction() {}

type stopAction struct {
	Error error
}

func (*stopAction) isAction() {}
