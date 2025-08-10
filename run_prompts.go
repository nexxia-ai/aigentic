package aigentic

import (
	"fmt"

	"github.com/nexxia-ai/aigentic/ai"
)

func (r *AgentRun) createToolPrompt() string {
	if len(r.tools) > 0 {
		msg := ""
		for _, tool := range r.tools {
			msg += fmt.Sprintf("<tool>\n%s\n%s\n</tool>\n", tool.Name, tool.Description)
		}
		return msg
	}
	return ""
}

func (r *AgentRun) createSystemPrompt() string {
	sysMsg := ""
	if r.parentRun == nil {
		sysMsg += "You are an autonomous agent working to complete a task.\n"
	} else {
		sysMsg += "You are an autonomous sub-agent working to complete a task on behalf of a coordinator agent.\n"
	}
	sysMsg += "You have to consider all the information you were given and reason about the next step to take.\n"

	if len(r.tools) > 0 {
		sysMsg += "You have access to one or more tools to complete the task. Use these tools as required to complete the task.\n"
	}
	sysMsg += "\n"

	if r.agent.Description != "" {
		sysMsg += "The user provided the following description of your role:\n"
		sysMsg += "<role>\n" + r.agent.Description + "\n</role>\n\n"
	}

	if r.agent.Instructions != "" {
		sysMsg += "\n <instructions>\n" + r.agent.Instructions + "\n</instructions>\n\n"
	}

	if s := r.memory.SystemPrompt(); s != "" {
		sysMsg += "\n<memory>\n" + s + "\n</memory>\n\n"
	}

	if s := r.createToolPrompt(); s != "" {
		sysMsg += "You have access to the following tools:\n"
		sysMsg += "\n<tools>\n" + s + "\n</tools>\n\n"
	}
	return sysMsg
}

// createUserMsg returns a list of Messages, with each attachment as a separate Resource message
func (r *AgentRun) createUserMsg(message string) []ai.Message {
	userMsg := ""
	if s := r.memory.Content(); s != "" {
		userMsg += "This is the content of the memory file (called ContextMemory.md):\n"
		userMsg += "<ContextMemory.md>\n" + s + "\n</ContextMemory.md>\n\n"
	}

	if message != "" {
		userMsg += "Please answer the following request or task:\n"
		userMsg += message + " \n\n"
	}

	var messages []ai.Message
	messages = append(messages, ai.UserMessage{Role: ai.UserRole, Content: userMsg})

	// Add each attachment as a separate Resource message with actual content
	for _, doc := range r.agent.Documents {
		content, err := doc.Bytes()
		if err != nil {
			continue // skip
		}

		attachmentMsg := ai.ResourceMessage{
			Role: ai.UserRole,
			URI:  "",
			Name: doc.Filename,
			Body: content,
			Type: deriveTypeFromMime(doc.MimeType),
		}
		messages = append(messages, attachmentMsg)
	}

	// Add attachment references as Resource messages with file:// URI
	for _, docRef := range r.agent.DocumentReferences {
		// Use the document ID as the file reference
		fileID := docRef.ID()

		refMsg := ai.ResourceMessage{
			Role: ai.UserRole,
			URI:  fmt.Sprintf("file://%s", fileID),
			Name: docRef.Filename,
			Body: nil,                                 // No body for file references
			Type: deriveTypeFromMime(docRef.MimeType), // Use actual MIME type
		}
		messages = append(messages, refMsg)
	}

	return messages
}
