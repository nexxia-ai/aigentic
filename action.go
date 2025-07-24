package aigentic

type Action interface {
	Target() string
}

type runAction struct {
	Message string
}

func (a *runAction) Target() string { return "" }

type approvalAction struct {
	EventID  string
	Approved bool
}

func (a *approvalAction) Target() string { return a.EventID }

type toolExecutionAction struct {
	EventID    string
	ToolCallID string
	ToolName   string
	ToolArgs   map[string]interface{}
	Group      *toolCallGroup
}

func (a *toolExecutionAction) Target() string { return a.EventID }

type stopAction struct {
	EventID string
}

func (a *stopAction) Target() string { return a.EventID }

type cancelAction struct {
	EventID string
}

func (a *cancelAction) Target() string { return a.EventID }
