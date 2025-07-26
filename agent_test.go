package aigentic

import (
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
)

func TestCreateUserMsg(t *testing.T) {
	agent := Agent{
		Attachments: []Attachment{
			{
				Type:     "file",
				Content:  []byte("test content"),
				MimeType: "text/plain",
				Name:     "file-abc123",
			},
			{
				Type:     "image",
				Content:  []byte("image data"),
				MimeType: "image/png",
				Name:     "test.png",
			},
		},
	}

	// Test with message and attachments (no FileID)
	messages := agent.createUserMsg("Hello, please analyze these files")

	assert.Len(t, messages, 3) // 1 main message + 2 attachments

	// Check main message
	mainMsg, ok := messages[0].(ai.UserMessage)
	assert.True(t, ok)
	assert.Equal(t, "Hello, please analyze these files", mainMsg.Content)

	// Check first attachment message (should include content)
	att1Msg, ok := messages[1].(ai.ResourceMessage)
	assert.True(t, ok)
	assert.Contains(t, att1Msg.Name, "file-abc123")
	assert.Contains(t, string(att1Msg.Body.([]byte)), "test content")

	// Check second attachment message (should include content)
	att2Msg, ok := messages[2].(ai.ResourceMessage)
	assert.True(t, ok)
	assert.Contains(t, att2Msg.Name, "test.png")
	assert.Contains(t, string(att2Msg.Body.([]byte)), "image data")

}
