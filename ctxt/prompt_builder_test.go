package ctxt

import (
	"context"
	"os"
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
				"<role>",
				"<instructions>",
				"<tools>",
				"<memories>",
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
				"<role>",
				"Test Agent Role",
				"</role>",
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
				return ac.SetOutputInstructions("Format output as JSON")
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
			name: "with memories",
			setup: func(ac *AgentContext) *AgentContext {
				ac.AddMemory("mem1", "Memory 1", "Content 1")
				ac.AddMemory("mem2", "Memory 2", "Content 2")
				return ac
			},
			tools: nil,
			expectedContains: []string{
				"<memories>",
				`<memory id="mem1" description="Memory 1">`,
				"Content 1",
				`<memory id="mem2" description="Memory 2">`,
				"Content 2",
				"</memories>",
			},
		},
		{
			name: "with system tags",
			setup: func(ac *AgentContext) *AgentContext {
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
				ac.SetOutputInstructions("Test Output")
				ac.AddMemory("mem1", "Memory 1", "Content 1")
				ac.Turn().InjectSystemTag("tag1", "tag content")
				return ac
			},
			tools: []ai.Tool{
				{Name: "tool1", Description: "Tool description"},
			},
			expectedContains: []string{
				"<role>",
				"Test Role",
				"<instructions>",
				"Test Instructions",
				"<output_instructions>",
				"Test Output",
				"<tools>",
				"tool1",
				"<memories>",
				"mem1",
				"<tag1>tag content</tag1>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac, err := New("test-id", "", "", t.TempDir())
			require.NoError(t, err)

			storeName := ac.ExecutionEnvironment().MemoryStoreName()
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

	store := document.NewLocalStore(ac.ExecutionEnvironment().MemoryDir)
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

	assert.Contains(t, content, "<document name=\"memory1.txt\">")
	assert.Contains(t, content, "Memory file 1 content")
	assert.Contains(t, content, "<document name=\"memory2.txt\">")
	assert.Contains(t, content, "Memory file 2 content")
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
				doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil)
				doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content2"), nil)
				ac.AddDocument(doc1)
				ac.AddDocument(doc2)
				return ac
			},
			expectedContains: []string{
				"The following documents are available",
				"ID: doc1",
				"Filename: test1.pdf",
				"Type: application/pdf",
				"ID: doc2",
				"Filename: test2.txt",
			},
			shouldBeNil: false,
		},
		{
			name: "with document references",
			setup: func(ac *AgentContext) *AgentContext {
				doc1 := document.NewInMemoryDocument("doc1", "ref1.pdf", []byte("content1"), nil)
				ac.AddDocumentReference(doc1)
				return ac
			},
			expectedContains: []string{
				"The following documents are available",
				"ID: doc1",
				"Filename: ref1.pdf",
			},
			shouldBeNil: false,
		},
		{
			name: "with documents and references",
			setup: func(ac *AgentContext) *AgentContext {
				doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil)
				doc2 := document.NewInMemoryDocument("doc2", "ref1.txt", []byte("content2"), nil)
				ac.AddDocument(doc1)
				ac.AddDocumentReference(doc2)
				return ac
			},
			expectedContains: []string{
				"The following documents are available",
				"doc1",
				"test1.pdf",
				"doc2",
				"ref1.txt",
			},
			shouldBeNil: false,
		},
		{
			name: "with empty filename document",
			setup: func(ac *AgentContext) *AgentContext {
				doc := document.NewInMemoryDocument("doc1", "", []byte("content"), nil)
				ac.AddDocument(doc)
				return ac
			},
			shouldBeNil: true,
		},
		{
			name: "with nil document",
			setup: func(ac *AgentContext) *AgentContext {
				ac.AddDocument(nil)
				return ac
			},
			shouldBeNil: true,
		},
		{
			name: "with unknown mime type",
			setup: func(ac *AgentContext) *AgentContext {
				doc := document.NewInMemoryDocument("doc1", "test.unknown", []byte("content"), nil)
				doc.MimeType = ""
				ac.AddDocument(doc)
				return ac
			},
			expectedContains: []string{
				"Type: unknown",
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
				return ac
			},
			message: "What is the weather?",
			expectedContains: []string{
				"Please answer the following request",
				"What is the weather?",
			},
		},
		{
			name: "with user tags",
			setup: func(ac *AgentContext) *AgentContext {
				ac.Turn().InjectUserTag("context", "additional context")
				ac.Turn().InjectUserTag("priority", "high")
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
				ac.Turn().InjectUserTag("context", "test context")
				return ac
			},
			message: "Test message",
			expectedContains: []string{
				"Please answer the following request",
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

			msg, err := createUserMsg(ac, tt.message)
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
				doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("content"), nil)
				ac.AddDocument(doc)
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
				ac.StartTurn("First message")
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
				ac.StartTurn("First message")
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
				doc := document.NewInMemoryDocument("doc1", "turn.pdf", []byte("turn content"), nil)
				ac.Turn().AddDocument("tool1", doc)
				return ac
			},
			tools:            nil,
			includeHistory:   false,
			userMessage:      "Process",
			expectedMsgCount: 3,
			validate: func(t *testing.T, msgs []ai.Message) {
				resourceFound := false
				for _, msg := range msgs {
					if rm, ok := msg.(ai.ResourceMessage); ok {
						resourceFound = true
						assert.Equal(t, "turn.pdf", rm.Name)
						assert.Equal(t, ai.UserRole, rm.Role)
					}
				}
				assert.True(t, resourceFound, "Should contain resource message")
			},
		},
		{
			name: "with tool messages",
			setup: func(ac *AgentContext) *AgentContext {
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
				ac.AddMemory("mem1", "Memory 1", "Content 1")

				doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil)
				ac.AddDocument(doc1)

				doc2 := document.NewInMemoryDocument("doc2", "turn.pdf", []byte("turn content"), nil)
				ac.Turn().AddDocument("tool1", doc2)

				ac.StartTurn("Previous message")
				ac.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Previous response"})

				return ac
			},
			tools: []ai.Tool{
				{Name: "tool1", Description: "Tool description"},
			},
			includeHistory:   true,
			userMessage:      "Current message",
			expectedMsgCount: 0,
			validate: func(t *testing.T, msgs []ai.Message) {
				assert.GreaterOrEqual(t, len(msgs), 3, "Should have at least system, docs, and user messages")
				sysMsg := msgs[0].(ai.SystemMessage)
				assert.Contains(t, sysMsg.Content, "Test Role")
				assert.Contains(t, sysMsg.Content, "Test Instructions")
				assert.Contains(t, sysMsg.Content, "mem1")
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
			ac.StartTurn(tt.userMessage)

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
		case ai.ResourceMessage:
			types[i] = "resource"
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
	ac.Turn().InjectSystemTag("tag1", "system tag content")

	ac.StartTurn("Test message")
	ac.Turn().InjectUserTag("tag2", "user tag content")

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

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil)
	ac.AddDocument(doc1)

	ac.StartTurn("Previous")
	ac.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "Response"})

	ac.StartTurn("Current message")

	doc2 := document.NewInMemoryDocument("doc2", "turn.pdf", []byte("turn content"), nil)
	ac.Turn().AddDocument("tool1", doc2)

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
	resourceMsgs := []ai.ResourceMessage{}
	toolMsgFound := false

	for _, msg := range msgs {
		if msg == nil {
			continue
		}

		if um, ok := msg.(ai.UserMessage); ok {
			if strings.Contains(um.Content, "The following documents are available") {
				docsMsgFound = true
				assert.Contains(t, um.Content, "test1.pdf")
			} else if strings.Contains(um.Content, "Previous") {
				historyUserFound = true
			} else if strings.Contains(um.Content, "Current message") {
				currentUserFound = true
			}
		}

		if am, ok := msg.(ai.AIMessage); ok && strings.Contains(am.Content, "Response") {
			historyAssistantFound = true
		}

		if rm, ok := msg.(ai.ResourceMessage); ok {
			resourceMsgs = append(resourceMsgs, rm)
		}

		if tm, ok := msg.(ai.ToolMessage); ok && tm.Content == "Tool result" {
			toolMsgFound = true
		}
	}

	assert.True(t, docsMsgFound, "Should have docs message")
	assert.True(t, historyUserFound, "Should have history user message")
	assert.True(t, historyAssistantFound, "Should have history assistant message")
	assert.True(t, currentUserFound, "Should have current user message")
	assert.GreaterOrEqual(t, len(resourceMsgs), 1, "Should have at least one resource message")
	assert.True(t, toolMsgFound, "Should have tool message")
}

func TestBuildPromptWithMemoryFiles(t *testing.T) {
	withTempWorkingDirPB(t)

	tempDir := t.TempDir()
	ac, err := New("test-id", "", "", tempDir)
	require.NoError(t, err)

	store := document.NewLocalStore(ac.ExecutionEnvironment().MemoryDir)
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

	ac.AddMemory("mem1", "Test Memory", "Memory content")
	ac.StartTurn("Test message")

	msgs, err := ac.BuildPrompt(nil, false)
	require.NoError(t, err)

	sysMsg := msgs[0].(ai.SystemMessage)
	assert.Contains(t, sysMsg.Content, "<document name=\"memory.txt\">")
	assert.Contains(t, sysMsg.Content, "Memory content")
	assert.Contains(t, sysMsg.Content, "<memories>")
	assert.Contains(t, sysMsg.Content, "mem1")
}

func TestBuildPromptDocumentReferences(t *testing.T) {
	ac, err := New("test-id", "", "", t.TempDir())
	require.NoError(t, err)

	doc1 := document.NewInMemoryDocument("doc1", "ref1.pdf", []byte("content1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "ref2.txt", []byte("content2"), nil)

	ac.AddDocument(doc1)
	ac.AddDocumentReference(doc2)

	ac.StartTurn("Process documents")

	msgs, err := ac.BuildPrompt(nil, false)
	require.NoError(t, err)

	docsMsgFound := false
	resourceMsgs := []ai.ResourceMessage{}

	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if um, ok := msg.(ai.UserMessage); ok && strings.Contains(um.Content, "The following documents are available") {
			docsMsgFound = true
			assert.Contains(t, um.Content, "ref1.pdf")
			assert.Contains(t, um.Content, "ref2.txt")
		}
		if rm, ok := msg.(ai.ResourceMessage); ok {
			resourceMsgs = append(resourceMsgs, rm)
		}
	}

	assert.True(t, docsMsgFound, "Should have documents message")
	assert.GreaterOrEqual(t, len(resourceMsgs), 1, "Should have at least one resource message (document reference)")

	refFound := false
	for _, rm := range resourceMsgs {
		if rm.URI != "" {
			refFound = true
			assert.Contains(t, rm.URI, "file://")
		}
	}
	assert.True(t, refFound, "Should have at least one document reference with URI")
}

func TestBuildPromptEmptyUserMessage(t *testing.T) {
	ac, err := New("test-id", "", "", t.TempDir())
	require.NoError(t, err)

	ac.StartTurn("")

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
