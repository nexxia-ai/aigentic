package aigentic

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

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
	ToolCallID string
	ToolName   string
	ToolArgs   map[string]interface{}
	Group      *toolCallGroup
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

func (r *AgentRun) queueToolCallAction(tcName string, tcArgs map[string]interface{}, toolCallID string, group *toolCallGroup) {
	tool := r.findTool(tcName)
	if tool == nil {
		r.queueAction(&toolResponseAction{
			request:  &toolCallAction{ToolName: tcName, ToolArgs: tcArgs, Group: group},
			response: fmt.Sprintf("tool not found: %s", tcName),
		})
		return
	}

	if tool.RequireApproval {
		approvalID := uuid.New().String()
		r.pendingApprovals[approvalID] = pendingApproval{
			ApprovalID: approvalID,
			Tool:       tool,
			ToolCallID: toolCallID,
			ToolArgs:   tcArgs,
			Group:      group,
			deadline:   time.Now().Add(r.approvalTimeout),
		}
		approvalEvent := &ApprovalEvent{
			RunID:      r.id,
			ApprovalID: approvalID,
			Content:    fmt.Sprintf("Approval required for tool: %s", tcName),
		}
		r.queueEvent(approvalEvent)
		return
	}

	r.queueAction(&toolCallAction{ToolCallID: toolCallID, ToolName: tcName, ToolArgs: tcArgs, Group: group})
}
