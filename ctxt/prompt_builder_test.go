package ctxt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSystemMsg(t *testing.T) {
	withTempWorkingDirPB(t)

	tests := []struct {
		name                string
		setup               func(*AgentContext) *AgentContext
		tools               []ai.Tool
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name: "empty context",
			setup: func(ac *AgentContext) *AgentContext {
				return ac
			},
			tools: nil,
			expectedContains: []string{
				"You are an autonomous agent",
			},
			expectedNotContains: []string{
				"<description>",
				"<instructions>",
				"<tools>",
				"<document",
			},
		},
		{
			name: "with role",
			setup: func(ac *AgentContext) *AgentContext {
				return ac.SetDescription("Test Agent Role")
			},
			tools: nil,
			expectedContains: []string{
				"<description>",
				"Test Agent Role",
				"</description>",
			},
		},
		{
			name: "with instructions",
			setup: func(ac *AgentContext) *AgentContext {
				return ac.SetInstructions("Follow these instructions carefully")
			},
			tools: nil,
			expectedContains: []string{
				"<instructions>",
				"Follow these instructions carefully",
				"</instructions>",
			},
		},
		{
			name: "with output instructions",
			setup: func(ac *AgentContext) *AgentContext {
				return ac.SetSystemPart(SystemPartKeyOutputInstructions, "Format output as JSON")
			},
			tools: nil,
			expectedContains: []string{
				"<output_instructions>",
				"Format output as JSON",
				"</output_instructions>",
			},
		},
		{
			name: "with tools",
			setup: func(ac *AgentContext) *AgentContext {
				return ac
			},
			tools: []ai.Tool{
				{Name: "tool1", Description: "First tool"},
				{Name: "tool2", Description: "Second tool"},
			},
			expectedContains: []string{
				"<tools>",
				"tool1",
				"First tool",
				"tool2",
				"Second tool",
				"</tools>",
			},
		},
		{
			name: "with system tags",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("", "")
				ac.Turn().InjectSystemTag("tag1", "tag content 1")
				ac.Turn().InjectSystemTag("tag2", "tag content 2")
				return ac
			},
			tools: nil,
			expectedContains: []string{
				"<tag1>tag content 1</tag1>",
				"<tag2>tag content 2</tag2>",
			},
		},
		{
			name: "with all components",
			setup: func(ac *AgentContext) *AgentContext {
				ac.SetDescription("Test Role")
				ac.SetInstructions("Test Instructions")
				ac.SetSystemPart(SystemPartKeyOutputInstructions, "Test Output")
				ac.StartTurn("", "")
				ac.Turn().InjectSystemTag("tag1", "tag content")
				return ac
			},
			tools: []ai.Tool{
				{Name: "tool1", Description: "Tool description"},
			},
			expectedContains: []string{
				"<description>",
				"Test Role",
				"<instructions>",
				"Test Instructions",
				"<output_instructions>",
				"Test Output",
				"<tools>",
				"tool1",
				"<tag1>tag content</tag1>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac, err := New("test-id", "", "", t.TempDir())
			require.NoError(t, err)

			storeName := ac.Workspace().MemoryStoreName()
			t.Cleanup(func() {
				document.UnregisterStore(storeName)
			})

			ac = tt.setup(ac)

			msg, err := createSystemMsg(ac, tt.tools)
			require.NoError(t, err)
			require.NotNil(t, msg)

			sysMsg, ok := msg.(ai.SystemMessage)
			require.True(t, ok)
			content := sysMsg.Content

			for _, expected := range tt.expectedContains {
				assert.Contains(t, content, expected, "System message should contain: %s", expected)
			}

			for _, notExpected := range tt.expectedNotContains {
				assert.NotContains(t, content, notExpected, "System message should not contain: %s", notExpected)
			}
		})
	}
}

func TestCreateSystemMsg_ordersGoalBeforeInstructions(t *testing.T) {
	withTempWorkingDirPB(t)
	ac, err := New("test-id", "desc", "inst", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		document.UnregisterStore(ac.Workspace().MemoryStoreName())
	})
	// Append order would put goal after instructions without canonical ordering
	ac.SetSystemPart(SystemPartKeyInstructions, "do the thing")
	ac.SetSystemPart(SystemPartKeyGoal, "Help the user succeed")

	msg, err := createSystemMsg(ac, nil)
	require.NoError(t, err)
	content := msg.(ai.SystemMessage).Content
	iGoal := strings.Index(content, "<goal>")
	iInst := strings.Index(content, "<instructions>")
	require.GreaterOrEqual(t, iGoal, 0)
	require.GreaterOrEqual(t, iInst, 0)
	assert.Less(t, iGoal, iInst, "goal block should appear before instructions")
}

// withTempWorkingDirPB switches the process working directory to a temp dir for the test.
// It restores the original working directory when the test ends.
func withTempWorkingDirPB(t *testing.T) {
	prev, err := os.Getwd()
	require.NoError(t, err)
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
}

func TestCreateSystemMsgWithMemoryFiles(t *testing.T) {
	withTempWorkingDirPB(t)

	tempDir := t.TempDir()
	ac, err := New("test-id", "", "", tempDir)
	require.NoError(t, err)
	require.NoError(t, ac.Workspace().SetMemoryDir(filepath.Join(ac.Workspace().LLMDir, "memory")))

	store := document.NewLocalStore(ac.Workspace().MemoryDir)
	storeID := store.ID()

	// Try to get existing store or register new one
	_, exists := document.GetStore(storeID)
	if !exists {
		if err := document.RegisterStore(store); err != nil {
			t.Fatalf("Failed to register store: %v", err)
		}
	}

	memDoc1Bytes := []byte("Memory file 1 content")
	memDoc2Bytes := []byte("Memory file 2 content")

	memDoc1, err := document.Create(context.Background(), store.ID(), "memory1.txt", strings.NewReader(string(memDoc1Bytes)))
	require.NoError(t, err)
	memDoc2, err := document.Create(context.Background(), store.ID(), "memory2.txt", strings.NewReader(string(memDoc2Bytes)))
	require.NoError(t, err)

	_ = memDoc1
	_ = memDoc2

	msg, err := createSystemMsg(ac, nil)
	require.NoError(t, err)
	require.NotNil(t, msg)

	sysMsg, ok := msg.(ai.SystemMessage)
	require.True(t, ok)
	content := sysMsg.Content

	memoryRelPath := "." + string(filepath.Separator) + filepath.Join("memory", "memory1.txt")
	assert.Contains(t, content, "<document name=\""+memoryRelPath+"\">")
	assert.Contains(t, content, "Memory file 1 content")
	memoryRelPath2 := "." + string(filepath.Separator) + filepath.Join("memory", "memory2.txt")
	assert.Contains(t, content, "<document name=\""+memoryRelPath2+"\">")
	assert.Contains(t, content, "Memory file 2 content")
}

func TestCreateSystemMsgWithEmptyMemoryDir_NoMemoryFilesInPrompt(t *testing.T) {
	withTempWorkingDirPB(t)
	tempDir := t.TempDir()
	ac, err := New("test-id", "", "", tempDir)
	require.NoError(t, err)
	require.Empty(t, ac.Workspace().MemoryDir, "MemoryDir should be empty by default")
	msg, err := createSystemMsg(ac, nil)
	require.NoError(t, err)
	require.NotNil(t, msg)
	sysMsg, ok := msg.(ai.SystemMessage)
	require.True(t, ok)
	assert.NotContains(t, sysMsg.Content, "<document name=", "system message should not contain memory documents when MemoryDir is empty")
}

func TestOpenFileRef_AllowsPathBasedDocumentIDReads(t *testing.T) {
	withTempWorkingDirPB(t)

	llmDir := filepath.Join(t.TempDir(), "llm")
	uploadPath := filepath.Join(llmDir, "uploads", "receipt.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(uploadPath), 0755))
	require.NoError(t, os.WriteFile(uploadPath, []byte("receipt bytes"), 0644))

	ref := FileRef{
		BasePath: llmDir,
		Path:     "uploads/receipt.txt",
		MimeType: "text/plain",
	}

	doc, err := OpenFileRef(ref)
	require.NoError(t, err)

	// GetDocument rewrites the document ID to the llm-relative path before reading bytes.
	doc.SetID(ref.Path)

	data, err := doc.Bytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("receipt bytes"), data)
	assert.Equal(t, ref.Path, doc.ID())
}

func TestCreateDocsMsg(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(*AgentContext) *AgentContext
		expectedContains []string
		shouldBeNil      bool
	}{
		{
			name: "no documents",
			setup: func(ac *AgentContext) *AgentContext {
				return ac
			},
			shouldBeNil: true,
		},
		{
			name: "with documents",
			setup: func(ac *AgentContext) *AgentContext {
				_ = attachTestDocument(ac, "uploads/test1.pdf", []byte("content1"), "", false)
				_ = attachTestDocument(ac, "uploads/test2.txt", []byte("content2"), "", false)
				ac.StartTurn("msg", "")
				return ac
			},
			expectedContains: []string{
				"The following documents are available",
				"test1.pdf",
				"Type: application/pdf",
				"test2.txt",
				"Type: text/plain",
			},
			shouldBeNil: false,
		},
		{
			name: "with multiple uploads",
			setup: func(ac *AgentContext) *AgentContext {
				_ = attachTestDocument(ac, "uploads/test1.pdf", []byte("content1"), "", false)
				_ = attachTestDocument(ac, "uploads/ref1.txt", []byte("content2"), "", false)
				ac.StartTurn("msg", "")
				return ac
			},
			expectedContains: []string{
				"The following documents are available",
				"test1.pdf",
				"ref1.txt",
			},
			shouldBeNil: false,
		},
		{
			name: "with empty filename document",
			setup: func(ac *AgentContext) *AgentContext {
				_ = attachTestDocument(ac, "uploads/doc1", []byte("content"), "", false)
				ac.StartTurn("msg", "")
				return ac
			},
			expectedContains: []string{
				"The following documents are available",
				"doc1",
			},
			shouldBeNil: false,
		},
		{
			name: "with no documents",
			setup: func(ac *AgentContext) *AgentContext {
				return ac
			},
			shouldBeNil: true,
		},
		{
			name: "with unknown mime type",
			setup: func(ac *AgentContext) *AgentContext {
				_ = attachTestDocument(ac, "uploads/test.unknown", []byte("content"), "", false)
				ac.StartTurn("msg", "")
				return ac
			},
			expectedContains: []string{
				"Type: application/octet-stream",
			},
			shouldBeNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac, err := New("test-id", "", "", t.TempDir())
			require.NoError(t, err)

			ac = tt.setup(ac)

			msg, err := createDocsMsg(ac)
			require.NoError(t, err)

			if tt.shouldBeNil {
				assert.Nil(t, msg)
			} else {
				require.NotNil(t, msg)
				userMsg, ok := msg.(ai.UserMessage)
				require.True(t, ok)
				content := userMsg.Content

				for _, expected := range tt.expectedContains {
					assert.Contains(t, content, expected, "Docs message should contain: %s", expected)
				}
			}
		})
	}
}

func TestCreateUserMsg(t *testing.T) {
	tests := []struct {
		name                string
		setup               func(*AgentContext) *AgentContext
		message             string
		expectedContains    []string
		expectedNotContains []string
	}{
		{
			name: "empty message",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("", "")
				return ac
			},
			message: "",
			expectedNotContains: []string{
				"Please answer the following request",
			},
		},
		{
			name: "with message",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("What is the weather?", "")
				return ac
			},
			message: "What is the weather?",
			expectedContains: []string{
				"What is the weather?",
			},
		},
		{
			name: "with user tags",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("Process this", "")
				ac.Turn().InjectTurnTag("context", "additional context")
				ac.Turn().InjectTurnTag("priority", "high")
				return ac
			},
			message: "Process this",
			expectedContains: []string{
				"<context>additional context</context>",
				"<priority>high</priority>",
			},
		},
		{
			name: "with message and user tags",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("Test message", "")
				ac.Turn().InjectTurnTag("context", "test context")
				return ac
			},
			message: "Test message",
			expectedContains: []string{
				"Test message",
				"<context>test context</context>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac, err := New("test-id", "", "", t.TempDir())
			require.NoError(t, err)

			ac = tt.setup(ac)

			msg, err := createUserMsg(ac)
			require.NoError(t, err)
			require.NotNil(t, msg)

			userMsg, ok := msg.(ai.UserMessage)
			require.True(t, ok)
			content := userMsg.Content

			for _, expected := range tt.expectedContains {
				assert.Contains(t, content, expected, "User message should contain: %s", expected)
			}

			for _, notExpected := range tt.expectedNotContains {
				assert.NotContains(t, content, notExpected, "User message should not contain: %s", notExpected)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name             string
		setup            func(*AgentContext) *AgentContext
		tools            []ai.Tool
		includeHistory   bool
		userMessage      string
		skipStartTurn    bool
		expectedMsgCount int
		expectedOrder    []string
		validate         func(*testing.T, []ai.Message)
	}{
		{
			name: "minimal prompt",
			setup: func(ac *AgentContext) *AgentContext {
				return ac
			},
			tools:            nil,
			includeHistory:   false,
			userMessage:      "Hello",
			expectedMsgCount: 2,
			expectedOrder:    []string{"system", "user"},
		},
		{
			name: "with documents",
			setup: func(ac *AgentContext) *AgentContext {
				_ = attachTestDocument(ac, "uploads/test.pdf", []byte("content"), "", false)
				return ac
			},
			tools:            nil,
			includeHistory:   false,
			userMessage:      "Process document",
			expectedMsgCount: 3,
			expectedOrder:    []string{"system", "user", "user"},
		},
		{
			name: "with history",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("First message", "")
				ac.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "First response"})
				return ac
			},
			tools:            nil,
			includeHistory:   true,
			userMessage:      "Second message",
			expectedMsgCount: 0,
			expectedOrder:    []string{"system"},
			validate: func(t *testing.T, msgs []ai.Message) {
				assert.GreaterOrEqual(t, len(msgs), 3, "Should have at least system, history user, history assistant, and current user")
				historyUserFound := false
				historyAssistantFound := false
				currentUserFound := false
				for _, msg := range msgs {
					if um, ok := msg.(ai.UserMessage); ok {
						if strings.Contains(um.Content, "First message") {
							historyUserFound = true
						}
						if strings.Contains(um.Content, "Second message") {
							currentUserFound = true
						}
					}
					if am, ok := msg.(ai.AIMessage); ok && strings.Contains(am.Content, "First response") {
						historyAssistantFound = true
					}
				}
				assert.True(t, historyUserFound, "Should have history user message")
				assert.True(t, historyAssistantFound, "Should have history assistant message")
				assert.True(t, currentUserFound, "Should have current user message")
			},
		},
		{
			name: "without history",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("First message", "")
				ac.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "First response"})
				return ac
			},
			tools:            nil,
			includeHistory:   false,
			userMessage:      "Second message",
			expectedMsgCount: 2,
			expectedOrder:    []string{"system", "user"},
		},
		{
			name: "with turn documents",
			setup: func(ac *AgentContext) *AgentContext {
				_ = attachTestDocument(ac, "uploads/turn.pdf", []byte("turn content"), "", true)
				ac.StartTurn("Process", "")
				return ac
			},
			tools:            nil,
			includeHistory:   false,
			userMessage:      "Process",
			skipStartTurn:    true,
			expectedMsgCount: 4,
			validate: func(t *testing.T, msgs []ai.Message) {
				resourceFound := false
				for _, msg := range msgs {
					if um, ok := msg.(ai.UserMessage); ok && len(um.Parts) > 0 {
						for _, part := range um.Parts {
							if part.Name == "turn.pdf" {
								resourceFound = true
								assert.Equal(t, ai.UserRole, um.Role)
								break
							}
						}
					}
				}
				assert.True(t, resourceFound, "Should contain resource message with content part")
			},
		},
		{
			name: "with tool messages",
			setup: func(ac *AgentContext) *AgentContext {
				ac.StartTurn("Test", "")
				toolMsg := ai.ToolMessage{
					Role:       ai.ToolRole,
					Content:    "Tool result",
					ToolCallID: "call1",
					ToolName:   "test_tool",
				}
				ac.Turn().AddMessage(toolMsg)
				return ac
			},
			tools:            nil,
			includeHistory:   false,
			userMessage:      "Test",
			skipStartTurn:    true,
			expectedMsgCount: 3,
			validate: func(t *testing.T, msgs []ai.Message) {
				toolFound := false
				for _, msg := range msgs {
					if tm, ok := msg.(ai.ToolMessage); ok {
						toolFound = true
						assert.Equal(t, "Tool result", tm.Content)
						assert.Equal(t, "test_tool", tm.ToolName)
					}
				}
				assert.True(t, toolFound, "Should contain tool message")
			},
		},
		{
			name: "full prompt with all components",
			setup: func(ac *AgentContext) *AgentContext {
				ac.SetDescription("Test Role")
				ac.SetInstructions("Test Instructions")

				_ = attachTestDocument(ac, "uploads/test1.pdf", []byte("content1"), "", false)

				ac.StartTurn("Previous message", "")
				ac.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Previous response"})

				_ = attachTestDocument(ac, "uploads/turn.pdf", []byte("turn content"), "", true)
				ac.StartTurn("Current message", "")

				return ac
			},
			tools: []ai.Tool{
				{Name: "tool1", Description: "Tool description"},
			},
			includeHistory:   true,
			userMessage:      "Current message",
			skipStartTurn:    true,
			expectedMsgCount: 0,
			validate: func(t *testing.T, msgs []ai.Message) {
				assert.GreaterOrEqual(t, len(msgs), 3, "Should have at least system, docs, and user messages")
				sysMsg := msgs[0].(ai.SystemMessage)
				assert.Contains(t, sysMsg.Content, "Test Role")
				assert.Contains(t, sysMsg.Content, "Test Instructions")
				assert.Contains(t, sysMsg.Content, "tool1")

				var userMsgFound bool
				for _, msg := range msgs {
					if um, ok := msg.(ai.UserMessage); ok && strings.Contains(um.Content, "Current message") {
						userMsgFound = true
						break
					}
				}
				assert.True(t, userMsgFound, "Should find user message with current message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac, err := New("test-id", "", "", t.TempDir())
			require.NoError(t, err)

			ac = tt.setup(ac)
			if !tt.skipStartTurn {
				ac.StartTurn(tt.userMessage, "")
			}

			msgs, err := ac.BuildPrompt(tt.tools, tt.includeHistory)
			require.NoError(t, err)
			require.NotNil(t, msgs)

			if tt.expectedMsgCount > 0 {
				assert.Equal(t, tt.expectedMsgCount, len(msgs), "Message count mismatch. Got %d messages: %v", len(msgs), getMessageTypes(msgs))
			}

			if len(tt.expectedOrder) > 0 {
				for i, expectedRole := range tt.expectedOrder {
					if i < len(msgs) && msgs[i] != nil {
						role, _ := msgs[i].Value()
						assert.Equal(t, ai.MessageRole(expectedRole), role, "Message %d should be %s", i, expectedRole)
					}
				}
			}

			if tt.validate != nil {
				tt.validate(t, msgs)
			}
		})
	}
}

func getMessageTypes(msgs []ai.Message) []string {
	types := make([]string, len(msgs))
	for i, msg := range msgs {
		if msg == nil {
			types[i] = "nil"
			continue
		}
		switch msg.(type) {
		case ai.SystemMessage:
			types[i] = "system"
		case ai.UserMessage:
			types[i] = "user"
		case ai.AIMessage:
			types[i] = "assistant"
		case ai.ToolMessage:
			types[i] = "tool"
		default:
			role, _ := msg.Value()
			types[i] = string(role) + "?"
		}
	}
	return types
}

func TestBuildPromptWithSystemAndUserTags(t *testing.T) {
	ac, err := New("test-id", "", "", t.TempDir())
	require.NoError(t, err)

	ac.SetDescription("Test Role")
	ac.StartTurn("Test message", "")
	ac.Turn().InjectSystemTag("tag1", "system tag content")
	ac.Turn().InjectTurnTag("tag2", "user tag content")

	msgs, err := ac.BuildPrompt(nil, false)
	require.NoError(t, err)

	sysMsg := msgs[0].(ai.SystemMessage)
	assert.Contains(t, sysMsg.Content, "<tag1>system tag content</tag1>")

	var userMsgFound bool
	for _, msg := range msgs {
		if um, ok := msg.(ai.UserMessage); ok && strings.Contains(um.Content, "Test message") {
			userMsgFound = true
			assert.Contains(t, um.Content, "<tag2>user tag content</tag2>")
			break
		}
	}
	assert.True(t, userMsgFound, "Should find user message with tags")
}

func TestBuildPromptMessageOrder(t *testing.T) {
	ac, err := New("test-id", "", "", t.TempDir())
	require.NoError(t, err)

	_ = attachTestDocument(ac, "uploads/test1.pdf", []byte("content1"), "", false)

	ac.StartTurn("Previous", "")
	ac.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Response"})

	_ = attachTestDocument(ac, "uploads/turn.pdf", []byte("turn content"), "", true)
	ac.StartTurn("Current message", "")

	toolMsg := ai.ToolMessage{
		Role:       ai.ToolRole,
		Content:    "Tool result",
		ToolCallID: "call1",
		ToolName:   "test_tool",
	}
	ac.Turn().AddMessage(toolMsg)

	msgs, err := ac.BuildPrompt(nil, true)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(msgs), 5, "Should have at least 5 messages")

	sysMsg := msgs[0]
	role, _ := sysMsg.Value()
	assert.Equal(t, ai.SystemRole, role, "First message should be system")

	docsMsgFound := false
	historyUserFound := false
	historyAssistantFound := false
	currentUserFound := false
	resourcePartsFound := 0
	toolMsgFound := false

	for _, msg := range msgs {
		if msg == nil {
			continue
		}

		if um, ok := msg.(ai.UserMessage); ok {
			if strings.Contains(um.Content, "The following documents are available") {
				docsMsgFound = true
				assert.Contains(t, um.Content, "turn.pdf")
			} else if strings.Contains(um.Content, "Previous") {
				historyUserFound = true
			} else if strings.Contains(um.Content, "Current message") {
				currentUserFound = true
			}
			if len(um.Parts) > 0 {
				resourcePartsFound += len(um.Parts)
			}
		}

		if am, ok := msg.(ai.AIMessage); ok && strings.Contains(am.Content, "Response") {
			historyAssistantFound = true
		}

		if tm, ok := msg.(ai.ToolMessage); ok && tm.Content == "Tool result" {
			toolMsgFound = true
		}
	}

	assert.True(t, docsMsgFound, "Should have docs message")
	assert.True(t, historyUserFound, "Should have history user message")
	assert.True(t, historyAssistantFound, "Should have history assistant message")
	assert.True(t, currentUserFound, "Should have current user message")
	assert.GreaterOrEqual(t, resourcePartsFound, 1, "Should have at least one resource content part")
	assert.True(t, toolMsgFound, "Should have tool message")
}

func TestBuildPromptWithMemoryFiles(t *testing.T) {
	withTempWorkingDirPB(t)

	tempDir := t.TempDir()
	ac, err := New("test-id", "", "", tempDir)
	require.NoError(t, err)
	require.NoError(t, ac.Workspace().SetMemoryDir(filepath.Join(ac.Workspace().LLMDir, "memory")))

	store := document.NewLocalStore(ac.Workspace().MemoryDir)
	storeID := store.ID()

	// Try to get existing store or register new one
	_, exists := document.GetStore(storeID)
	if !exists {
		if err := document.RegisterStore(store); err != nil {
			t.Fatalf("Failed to register store: %v", err)
		}
	}

	memDocBytes := []byte("Memory content")
	memDoc, err := document.Create(context.Background(), store.ID(), "memory.txt", strings.NewReader(string(memDocBytes)))
	require.NoError(t, err)

	_ = memDoc

	ac.StartTurn("Test message", "")

	msgs, err := ac.BuildPrompt(nil, false)
	require.NoError(t, err)

	sysMsg := msgs[0].(ai.SystemMessage)
	memoryRelPath := "." + string(filepath.Separator) + filepath.Join("memory", "memory.txt")
	assert.Contains(t, sysMsg.Content, "<document name=\""+memoryRelPath+"\">")
	assert.Contains(t, sysMsg.Content, "Memory content")
}

func TestBuildPromptUploadedDocuments(t *testing.T) {
	ac, err := New("test-id", "", "", t.TempDir())
	require.NoError(t, err)

	_ = attachTestDocument(ac, "uploads/ref1.pdf", []byte("content1"), "", false)
	_ = attachTestDocument(ac, "uploads/ref2.txt", []byte("content2"), "", false)

	ac.StartTurn("Process documents", "")

	msgs, err := ac.BuildPrompt(nil, false)
	require.NoError(t, err)

	docsMsgFound := false
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if um, ok := msg.(ai.UserMessage); ok && strings.Contains(um.Content, "The following documents are available") {
			docsMsgFound = true
			assert.Contains(t, um.Content, "ref1.pdf")
			assert.Contains(t, um.Content, "ref2.txt")
			break
		}
	}
	assert.True(t, docsMsgFound, "Should have documents message listing uploaded ref1.pdf and ref2.txt")
}

func TestBuildPromptLimitsHistoryToLatest100Turns(t *testing.T) {
	ac, err := New("test-id", "", "", t.TempDir())
	require.NoError(t, err)

	for i := 0; i < 105; i++ {
		ac.StartTurn(fmt.Sprintf("history-%03d", i), "")
		ac.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: fmt.Sprintf("reply-%03d", i)})
	}
	ac.StartTurn("current", "")

	msgs, err := ac.BuildPrompt(nil, true)
	require.NoError(t, err)

	historyUsers := 0
	historyReplies := 0
	for _, msg := range msgs {
		switch m := msg.(type) {
		case ai.UserMessage:
			if strings.Contains(m.Content, "history-") {
				historyUsers++
				assert.NotContains(t, m.Content, "history-000")
				assert.NotContains(t, m.Content, "history-001")
				assert.NotContains(t, m.Content, "history-002")
				assert.NotContains(t, m.Content, "history-003")
				assert.NotContains(t, m.Content, "history-004")
			}
		case ai.AIMessage:
			if strings.Contains(m.Content, "reply-") {
				historyReplies++
				assert.NotContains(t, m.Content, "reply-000")
				assert.NotContains(t, m.Content, "reply-001")
				assert.NotContains(t, m.Content, "reply-002")
				assert.NotContains(t, m.Content, "reply-003")
				assert.NotContains(t, m.Content, "reply-004")
			}
		}
	}

	assert.Equal(t, 100, historyUsers)
	assert.Equal(t, 100, historyReplies)
	assert.Equal(t, 105, len(ac.GetHistory().GetTurns()))
	assert.Equal(t, 210, len(ac.GetHistory().GetMessages(ac)))
}

func TestBuildPromptEmptyUserMessage(t *testing.T) {
	ac, err := New("test-id", "", "", t.TempDir())
	require.NoError(t, err)

	ac.StartTurn("", "")

	msgs, err := ac.BuildPrompt(nil, false)
	require.NoError(t, err)

	userMsgFound := false
	for _, msg := range msgs {
		if um, ok := msg.(ai.UserMessage); ok {
			userMsgFound = true
			assert.NotContains(t, um.Content, "Please answer the following request")
		}
	}
	assert.True(t, userMsgFound, "Should have user message even with empty content")
}
