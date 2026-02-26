package run

import (
	"fmt"

	"github.com/nexxia-ai/aigentic/ctxt"
)

type Skill = ctxt.Skill

func (r *AgentRun) AddSkill(skill Skill) error {
	if r == nil || r.agentContext == nil {
		return fmt.Errorf("agent context is not set")
	}
	return r.agentContext.AddSkill(skill)
}

func (r *AgentRun) RemoveSkill(id string) bool {
	if r == nil || r.agentContext == nil {
		return false
	}
	return r.agentContext.RemoveSkill(id)
}

func (r *AgentRun) Skills() []Skill {
	if r == nil || r.agentContext == nil {
		return nil
	}
	return r.agentContext.Skills()
}
