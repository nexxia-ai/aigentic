package aigentic

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/nexxia-ai/aigentic/ai"
)

// EnhancedSystemContextManager moves all memory to system prompt with improved structure
type EnhancedSystemContextManager struct {
	agent          Agent
	userMsg        string
	msgHistory     []ai.Message
	currentMsg     int
	SystemTemplate *template.Template
	UserTemplate   *template.Template
}

var _ ContextManager = &EnhancedSystemContextManager{}

const EnhancedSystemTemplate = `You are an autonomous agent with the following configuration:

{{if .HasRole}}
## ROLE
{{.Role}}

{{end}}{{if .HasInstructions}}
## INSTRUCTIONS
{{.Instructions}}

{{end}}{{if .HasMemory}}
## PERSISTENT MEMORY
The following information is stored in your persistent memory:
{{.Memory}}

{{end}}{{if .HasTools}}
## AVAILABLE TOOLS
You have access to the following tools:
{{range .Tools}}
- **{{.Name}}**: {{.Description}}
{{end}}

{{end}}{{if .HasToolHistory}}
## RECENT TOOL USAGE
{{.ToolHistory}}

{{end}}## TASK EXECUTION
Analyze the user's request and use the available tools systematically to complete the task. 
Consider your persistent memory and avoid repeating previous actions unless necessary.`

const EnhancedUserTemplate = `{{if .HasMessage}}{{.Message}}{{end}}`

func NewEnhancedSystemContextManager(agent Agent, userMsg string) *EnhancedSystemContextManager {
	cm := &EnhancedSystemContextManager{agent: agent, userMsg: userMsg}
	cm.SetTemplates()
	return cm
}

func (r *EnhancedSystemContextManager) SetTemplates() {
	r.SystemTemplate = template.Must(template.New("enhanced_system").Parse(EnhancedSystemTemplate))
	r.UserTemplate = template.Must(template.New("enhanced_user").Parse(EnhancedUserTemplate))
}

func (r *EnhancedSystemContextManager) BuildPrompt(ctx context.Context, messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	r.currentMsg = len(r.msgHistory)
	r.msgHistory = append(r.msgHistory, messages...)

	// Generate system prompt using template
	systemVars := r.createSystemVariables(tools)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute enhanced system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	// Generate user prompt using template
	userVars := r.createUserVariables(r.userMsg)
	var userBuf bytes.Buffer
	if err := r.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute enhanced user template: %w", err)
	}

	userContent := userBuf.String()
	if userContent != "" {
		msgs = append(msgs, ai.UserMessage{Role: ai.UserRole, Content: userContent})
	}

	msgs = append(msgs, r.addDocuments()...)
	msgs = append(msgs, r.msgHistory...)
	return msgs, nil
}

// UserCentricMemoryContextManager keeps memory in user prompt but improves formatting
type UserCentricMemoryContextManager struct {
	agent          Agent
	userMsg        string
	msgHistory     []ai.Message
	currentMsg     int
	SystemTemplate *template.Template
	UserTemplate   *template.Template
}

var _ ContextManager = &UserCentricMemoryContextManager{}

const UserCentricSystemTemplate = `You are an autonomous agent working to complete tasks efficiently.

{{if .HasRole}}
## YOUR ROLE
{{.Role}}

{{end}}{{if .HasInstructions}}
## OPERATING INSTRUCTIONS
{{.Instructions}}

{{end}}{{if .HasTools}}
## TOOL CAPABILITIES
{{range .Tools}}
• {{.Name}}: {{.Description}}
{{end}}

{{end}}{{if .HasToolHistory}}
## PREVIOUS TOOL USAGE
{{.ToolHistory}}

{{end}}Focus on the user's current request and utilize your tools systematically.`

const UserCentricUserTemplate = `{{if .HasMemory}}## CONTEXT FROM MEMORY
{{.MemoryContent}}

{{end}}{{if .HasMessage}}## CURRENT REQUEST
{{.Message}}{{end}}`

func NewUserCentricMemoryContextManager(agent Agent, userMsg string) *UserCentricMemoryContextManager {
	cm := &UserCentricMemoryContextManager{agent: agent, userMsg: userMsg}
	cm.SetTemplates()
	return cm
}

func (r *UserCentricMemoryContextManager) SetTemplates() {
	r.SystemTemplate = template.Must(template.New("user_centric_system").Parse(UserCentricSystemTemplate))
	r.UserTemplate = template.Must(template.New("user_centric_user").Parse(UserCentricUserTemplate))
}

func (r *UserCentricMemoryContextManager) BuildPrompt(ctx context.Context, messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	r.currentMsg = len(r.msgHistory)
	r.msgHistory = append(r.msgHistory, messages...)

	systemVars := r.createSystemVariables(tools)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute user centric system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	userVars := r.createUserVariables(r.userMsg)
	var userBuf bytes.Buffer
	if err := r.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute user centric user template: %w", err)
	}

	userContent := userBuf.String()
	if userContent != "" {
		msgs = append(msgs, ai.UserMessage{Role: ai.UserRole, Content: userContent})
	}

	msgs = append(msgs, r.addDocuments()...)
	msgs = append(msgs, r.msgHistory...)
	return msgs, nil
}

// MinimalSystemContextManager provides lightweight system prompt with rich user context
type MinimalSystemContextManager struct {
	agent          Agent
	userMsg        string
	msgHistory     []ai.Message
	currentMsg     int
	SystemTemplate *template.Template
	UserTemplate   *template.Template
}

var _ ContextManager = &MinimalSystemContextManager{}

const MinimalSystemTemplate = `You are {{.Role}}

{{if .HasInstructions}}{{.Instructions}}{{end}}

{{if .HasTools}}Available tools: {{range .Tools}}{{.Name}}{{end}}{{end}}`

const MinimalUserTemplate = `{{if .HasMemory}}### Memory
{{.MemoryContent}}

{{end}}{{if .HasToolHistory}}### Recent Actions
{{.ToolHistory}}

{{end}}{{if .HasTools}}### Tools
{{range .Tools}}**{{.Name}}**: {{.Description}}
{{end}}

{{end}}{{if .HasMessage}}### Task
{{.Message}}{{end}}`

func NewMinimalSystemContextManager(agent Agent, userMsg string) *MinimalSystemContextManager {
	cm := &MinimalSystemContextManager{agent: agent, userMsg: userMsg}
	cm.SetTemplates()
	return cm
}

func (r *MinimalSystemContextManager) SetTemplates() {
	r.SystemTemplate = template.Must(template.New("minimal_system").Parse(MinimalSystemTemplate))
	r.UserTemplate = template.Must(template.New("minimal_user").Parse(MinimalUserTemplate))
}

func (r *MinimalSystemContextManager) BuildPrompt(ctx context.Context, messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	r.currentMsg = len(r.msgHistory)
	r.msgHistory = append(r.msgHistory, messages...)

	systemVars := r.createSystemVariables(tools)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute minimal system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	userVars := r.createUserVariables(r.userMsg)
	var userBuf bytes.Buffer
	if err := r.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute minimal user template: %w", err)
	}

	userContent := userBuf.String()
	if userContent != "" {
		msgs = append(msgs, ai.UserMessage{Role: ai.UserRole, Content: userContent})
	}

	msgs = append(msgs, r.addDocuments()...)
	msgs = append(msgs, r.msgHistory...)
	return msgs, nil
}

// HierarchicalContextManager structures information with clear priority hierarchy
type HierarchicalContextManager struct {
	agent          Agent
	userMsg        string
	msgHistory     []ai.Message
	currentMsg     int
	SystemTemplate *template.Template
	UserTemplate   *template.Template
}

var _ ContextManager = &HierarchicalContextManager{}

const HierarchicalSystemTemplate = `# AGENT CONFIGURATION

## CORE IDENTITY
{{if .HasRole}}{{.Role}}{{end}}

## PRIMARY DIRECTIVES
{{if .HasInstructions}}{{.Instructions}}{{end}}

## KNOWLEDGE BASE
{{if .HasMemory}}{{.Memory}}{{end}}

## CAPABILITIES
{{if .HasTools}}
{{range .Tools}}
### {{.Name}}
{{.Description}}
{{end}}
{{end}}

## EXECUTION CONTEXT
{{if .HasToolHistory}}
Recent tool usage:
{{.ToolHistory}}
{{end}}

Execute tasks systematically following your directives and utilizing available capabilities.`

const HierarchicalUserTemplate = `# CURRENT TASK

{{if .HasMessage}}{{.Message}}{{end}}

{{if .HasMemory}}
# RELEVANT CONTEXT
{{.MemoryContent}}
{{end}}`

func NewHierarchicalContextManager(agent Agent, userMsg string) *HierarchicalContextManager {
	cm := &HierarchicalContextManager{agent: agent, userMsg: userMsg}
	cm.SetTemplates()
	return cm
}

func (r *HierarchicalContextManager) SetTemplates() {
	r.SystemTemplate = template.Must(template.New("hierarchical_system").Parse(HierarchicalSystemTemplate))
	r.UserTemplate = template.Must(template.New("hierarchical_user").Parse(HierarchicalUserTemplate))
}

func (r *HierarchicalContextManager) BuildPrompt(ctx context.Context, messages []ai.Message, tools []ai.Tool) ([]ai.Message, error) {
	r.currentMsg = len(r.msgHistory)
	r.msgHistory = append(r.msgHistory, messages...)

	systemVars := r.createSystemVariables(tools)
	var systemBuf bytes.Buffer
	if err := r.SystemTemplate.Execute(&systemBuf, systemVars); err != nil {
		return nil, fmt.Errorf("failed to execute hierarchical system template: %w", err)
	}

	msgs := []ai.Message{
		ai.SystemMessage{Role: ai.SystemRole, Content: systemBuf.String()},
	}

	userVars := r.createUserVariables(r.userMsg)
	var userBuf bytes.Buffer
	if err := r.UserTemplate.Execute(&userBuf, userVars); err != nil {
		return nil, fmt.Errorf("failed to execute hierarchical user template: %w", err)
	}

	userContent := userBuf.String()
	if userContent != "" {
		msgs = append(msgs, ai.UserMessage{Role: ai.UserRole, Content: userContent})
	}

	msgs = append(msgs, r.addDocuments()...)
	msgs = append(msgs, r.msgHistory...)
	return msgs, nil
}

// Shared helper methods for all context managers
func (r *EnhancedSystemContextManager) createSystemVariables(tools []ai.Tool) map[string]interface{} {
	return createSystemVariables(r.agent, tools, r.createToolHistory())
}

func (r *EnhancedSystemContextManager) createUserVariables(message string) map[string]interface{} {
	return createUserVariables(r.agent, message)
}

func (r *EnhancedSystemContextManager) createToolHistory() string {
	return createToolHistory(r.msgHistory, r.currentMsg)
}

func (r *EnhancedSystemContextManager) addDocuments() []ai.Message {
	return addDocuments(r.agent)
}

func (r *UserCentricMemoryContextManager) createSystemVariables(tools []ai.Tool) map[string]interface{} {
	return createSystemVariables(r.agent, tools, r.createToolHistory())
}

func (r *UserCentricMemoryContextManager) createUserVariables(message string) map[string]interface{} {
	return createUserVariables(r.agent, message)
}

func (r *UserCentricMemoryContextManager) createToolHistory() string {
	return createToolHistory(r.msgHistory, r.currentMsg)
}

func (r *UserCentricMemoryContextManager) addDocuments() []ai.Message {
	return addDocuments(r.agent)
}

func (r *MinimalSystemContextManager) createSystemVariables(tools []ai.Tool) map[string]interface{} {
	return createSystemVariables(r.agent, tools, r.createToolHistory())
}

func (r *MinimalSystemContextManager) createUserVariables(message string) map[string]interface{} {
	return createUserVariables(r.agent, message)
}

func (r *MinimalSystemContextManager) createToolHistory() string {
	return createToolHistory(r.msgHistory, r.currentMsg)
}

func (r *MinimalSystemContextManager) addDocuments() []ai.Message {
	return addDocuments(r.agent)
}

func (r *HierarchicalContextManager) createSystemVariables(tools []ai.Tool) map[string]interface{} {
	return createSystemVariables(r.agent, tools, r.createToolHistory())
}

func (r *HierarchicalContextManager) createUserVariables(message string) map[string]interface{} {
	return createUserVariables(r.agent, message)
}

func (r *HierarchicalContextManager) createToolHistory() string {
	return createToolHistory(r.msgHistory, r.currentMsg)
}

func (r *HierarchicalContextManager) addDocuments() []ai.Message {
	return addDocuments(r.agent)
}
