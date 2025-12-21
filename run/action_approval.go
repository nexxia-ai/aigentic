package run

import (
	"fmt"
	"time"
)

func (r *AgentRun) runApprovalAction(act *approvalAction) {
	approval, ok := r.pendingApprovals[act.ApprovalID]
	if !ok {
		r.Logger.Error("invalid approval ID", "approvalID", act.ApprovalID)
		return
	}
	delete(r.pendingApprovals, act.ApprovalID)

	if act.Approved {
		r.queueAction(&toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group})
		return
	}
	r.queueAction(&toolResponseAction{
		request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group},
		response: fmt.Sprintf("approval denied for tool: %s", approval.Tool.Name),
	})
}

func (r *AgentRun) checkApprovalTimeouts() {
	now := time.Now()
	for approvalID, approval := range r.pendingApprovals {
		if now.After(approval.deadline) {
			r.Logger.Error("approval timed out", "approvalID", approvalID, "deadline", approval.deadline)
			delete(r.pendingApprovals, approvalID)
			r.queueAction(
				&toolResponseAction{
					request:  &toolCallAction{ToolCallID: approval.ToolCallID, ToolName: approval.Tool.Name, ValidationResult: approval.ValidationResult, Group: approval.Group},
					response: fmt.Sprintf("approval timed out for tool: %s", approval.Tool.Name),
				})
		}
	}
}
