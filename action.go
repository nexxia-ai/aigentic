package aigentic

type Action interface {
	Target() string
}

type llmCallAction struct {
	Message string
}

func (a *llmCallAction) Target() string { return "" }

type approvalAction struct {
	ApprovalID string
	Approved   bool
}

func (a *approvalAction) Target() string { return "" }

type toolCallAction struct {
	ToolCallID       string
	ToolName         string
	ValidationResult ValidationResult
	Group            *toolCallGroup
}

func (a *toolCallAction) Target() string { return "" }

type toolResponseAction struct {
	request  *toolCallAction
	response string
}

func (a *toolResponseAction) Target() string { return "" }

type stopAction struct {
	Error error
}

func (a *stopAction) Target() string { return "" }

type cancelAction struct {
}

func (a *cancelAction) Target() string { return "" }
