package run

import (
	"path/filepath"
	"testing"

	"github.com/nexxia-ai/aigentic/ctxt"
)

func TestNewChildRunInheritsGoalOutputInstructionsAndSkills(t *testing.T) {
	parent, err := NewAgentRun("parent", "parent desc", "parent inst", t.TempDir())
	if err != nil {
		t.Fatalf("NewAgentRun: %v", err)
	}
	parent.SetGoal("parent goal")
	parent.AgentContext().SetSystemPart(ctxt.SystemPartKeyOutputInstructions, "parent output")
	parent.AgentContext().SetSystemPart(ctxt.SystemPartKeySkills, "parent skills")

	privateDir := filepath.Join(parent.AgentContext().Workspace().RootDir, "_aigentic", "batch", "0")
	child, err := NewChildRun(parent, "child", "child desc", "child inst", privateDir, parent.Model(), nil)
	if err != nil {
		t.Fatalf("NewChildRun: %v", err)
	}

	requireChildPromptPart(t, child, ctxt.SystemPartKeyGoal, "parent goal")
	requireChildPromptPart(t, child, ctxt.SystemPartKeyOutputInstructions, "parent output")
	requireChildPromptPart(t, child, ctxt.SystemPartKeySkills, "parent skills")
}

func TestNewChildRunOverridesGoal(t *testing.T) {
	parent, err := NewAgentRun("parent", "parent desc", "parent inst", t.TempDir())
	if err != nil {
		t.Fatalf("NewAgentRun: %v", err)
	}
	parent.SetGoal("parent goal")

	privateDir := filepath.Join(parent.AgentContext().Workspace().RootDir, "_aigentic", "batch", "0")
	child, err := NewChildRun(parent, "child", "child desc", "child inst", privateDir, parent.Model(), nil, "child goal")
	if err != nil {
		t.Fatalf("NewChildRun: %v", err)
	}

	requireChildPromptPart(t, child, ctxt.SystemPartKeyGoal, "child goal")
}

func requireChildPromptPart(t *testing.T, run *AgentRun, key, want string) {
	t.Helper()
	got, ok := run.AgentContext().PromptPart(key)
	if !ok {
		t.Fatalf("missing prompt part %q", key)
	}
	if got != want {
		t.Fatalf("prompt part %q = %q, want %q", key, got, want)
	}
}
