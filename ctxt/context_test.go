package ctxt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

// withTempWorkingDir switches to a temp working directory for the test and restores it afterward.
func withTempWorkingDir(t *testing.T) {
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func createTestContext(t *testing.T, id, description, instructions string) *AgentContext {
	ctx, err := New(id, description, instructions, t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test context: %v", err)
	}
	return ctx
}

func TestNew(t *testing.T) {
	ctx := createTestContext(t, "test-id", "test description", "test instructions")

	if ctx.id != "test-id" {
		t.Errorf("expected id 'test-id', got '%s'", ctx.id)
	}
	if ctx.description != "test description" {
		t.Errorf("expected description 'test description', got '%s'", ctx.description)
	}
	if ctx.instructions != "test instructions" {
		t.Errorf("expected instructions 'test instructions', got '%s'", ctx.instructions)
	}
	if ctx.conversationHistory == nil {
		t.Error("expected conversation history to be initialized")
	}
}

func TestAddDocument(t *testing.T) {
	tests := []struct {
		name      string
		docs      []*document.Document
		wantCount int
	}{
		{
			name:      "add single document",
			docs:      []*document.Document{document.NewInMemoryDocument("doc1", "test.pdf", []byte("content"), nil)},
			wantCount: 1,
		},
		{
			name: "add multiple documents",
			docs: []*document.Document{
				document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil),
				document.NewInMemoryDocument("doc2", "test2.txt", []byte("content2"), nil),
				document.NewInMemoryDocument("doc3", "test3.png", []byte("content3"), nil),
			},
			wantCount: 3,
		},
		{
			name:      "add nil document",
			docs:      []*document.Document{nil},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createTestContext(t, "id", "desc", "inst")
			for _, doc := range tt.docs {
				ctx.AddDocument(doc)
			}

			got := len(ctx.GetDocuments())
			if got != tt.wantCount {
				t.Errorf("expected %d documents, got %d", tt.wantCount, got)
			}
		})
	}
}

func TestRemoveDocument(t *testing.T) {
	tests := []struct {
		name      string
		setup     []*document.Document
		remove    *document.Document
		wantErr   bool
		wantCount int
	}{
		{
			name: "remove existing document",
			setup: []*document.Document{
				document.NewInMemoryDocument("doc1", "test.pdf", []byte("content"), nil),
			},
			remove:    document.NewInMemoryDocument("doc1", "test.pdf", []byte("content"), nil),
			wantErr:   false,
			wantCount: 0,
		},
		{
			name: "remove non-existing document",
			setup: []*document.Document{
				document.NewInMemoryDocument("doc1", "test.pdf", []byte("content"), nil),
			},
			remove:    document.NewInMemoryDocument("doc2", "other.pdf", []byte("content"), nil),
			wantErr:   true,
			wantCount: 1,
		},
		{
			name:      "remove nil document",
			setup:     []*document.Document{},
			remove:    nil,
			wantErr:   true,
			wantCount: 0,
		},
		{
			name: "remove from middle",
			setup: []*document.Document{
				document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil),
				document.NewInMemoryDocument("doc2", "test2.pdf", []byte("content2"), nil),
				document.NewInMemoryDocument("doc3", "test3.pdf", []byte("content3"), nil),
			},
			remove:    document.NewInMemoryDocument("doc2", "test2.pdf", []byte("content2"), nil),
			wantErr:   false,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createTestContext(t, "id", "desc", "inst")
			for _, doc := range tt.setup {
				ctx.AddDocument(doc)
			}

			err := ctx.RemoveDocument(tt.remove)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveDocument() error = %v, wantErr %v", err, tt.wantErr)
			}

			got := len(ctx.GetDocuments())
			if got != tt.wantCount {
				t.Errorf("expected %d documents, got %d", tt.wantCount, got)
			}
		})
	}
}

func TestRemoveDocumentByID(t *testing.T) {
	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.pdf", []byte("content2"), nil)

	ctx := createTestContext(t, "id", "desc", "inst").
		AddDocument(doc1).
		AddDocument(doc2)

	err := ctx.RemoveDocumentByID("test1.pdf")
	if err != nil {
		t.Errorf("RemoveDocumentByID() error = %v", err)
	}

	docs := ctx.GetDocuments()
	if len(docs) != 1 {
		t.Errorf("expected 1 document, got %d", len(docs))
	}
	if len(docs) > 0 && docs[0].ID() != "uploads/test2.pdf" {
		t.Errorf("expected remaining document to be uploads/test2.pdf, got %s", docs[0].ID())
	}

	err = ctx.RemoveDocumentByID("nonexistent")
	if err == nil {
		t.Error("expected error when removing non-existent document")
	}
}

func TestChainableMethods(t *testing.T) {
	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.pdf", []byte("content2"), nil)

	ctx := createTestContext(t, "id", "desc", "inst").
		SetOutputInstructions("Use JSON").
		AddDocument(doc1).
		AddDocument(doc2)

	if ctx.outputInstructions != "Use JSON" {
		t.Errorf("expected output instructions 'Use JSON', got '%s'", ctx.outputInstructions)
	}

	docCount := len(ctx.GetDocuments())
	if docCount != 2 {
		t.Errorf("expected 2 documents, got %d", docCount)
	}
}

func TestClearMethods(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*AgentContext)
		clear     func(*AgentContext)
		check     func(*AgentContext) int
		wantCount int
	}{
		{
			name: "clear documents",
			setup: func(ctx *AgentContext) {
				ctx.AddDocument(document.NewInMemoryDocument("doc1", "test1.pdf", []byte("c"), nil))
				ctx.AddDocument(document.NewInMemoryDocument("doc2", "test2.pdf", []byte("c"), nil))
			},
			clear:     func(ctx *AgentContext) { ctx.ClearDocuments() },
			check:     func(ctx *AgentContext) int { return len(ctx.GetDocuments()) },
			wantCount: 0,
		},
		{
			name: "clear history",
			setup: func(ctx *AgentContext) {
				ctx.GetHistory().appendTurn(Turn{})
				ctx.GetHistory().appendTurn(Turn{})
			},
			clear:     func(ctx *AgentContext) { ctx.ClearHistory() },
			check:     func(ctx *AgentContext) int { return ctx.GetHistory().Len() },
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createTestContext(t, "id", "desc", "inst")
			tt.setup(ctx)
			tt.clear(ctx)
			got := tt.check(ctx)
			if got != tt.wantCount {
				t.Errorf("expected %d items after clear, got %d", tt.wantCount, got)
			}
		})
	}
}

func TestClearAll(t *testing.T) {
	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("content"), nil)

	ctx := createTestContext(t, "id", "desc", "inst").
		AddDocument(doc)

	ctx.GetHistory().appendTurn(Turn{})

	ctx.ClearAll()

	if len(ctx.GetDocuments()) != 0 {
		t.Errorf("expected 0 documents after ClearAll, got %d", len(ctx.GetDocuments()))
	}
	if ctx.GetHistory().Len() != 0 {
		t.Errorf("expected 0 history turns after ClearAll, got %d", ctx.GetHistory().Len())
	}
}

func TestSetMethods(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")

	ctx.SetDescription("new description").
		SetInstructions("new instructions").
		SetOutputInstructions("new output instructions")

	if ctx.description != "new description" {
		t.Errorf("expected description 'new description', got '%s'", ctx.description)
	}
	if ctx.instructions != "new instructions" {
		t.Errorf("expected instructions 'new instructions', got '%s'", ctx.instructions)
	}
	if ctx.outputInstructions != "new output instructions" {
		t.Errorf("expected output instructions 'new output instructions', got '%s'", ctx.outputInstructions)
	}
}

func TestHistoryQuery(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")

	turn1 := Turn{AgentName: "agent1", Hidden: false}
	turn2 := Turn{AgentName: "agent2", Hidden: false}
	turn3 := Turn{AgentName: "agent1", Hidden: true}
	turn4 := Turn{AgentName: "agent1", Hidden: false}

	ctx.GetHistory().appendTurn(turn1)
	ctx.GetHistory().appendTurn(turn2)
	ctx.GetHistory().appendTurn(turn3)
	ctx.GetHistory().appendTurn(turn4)

	tests := []struct {
		name      string
		query     func() []Turn
		wantCount int
	}{
		{
			name:      "get all turns",
			query:     func() []Turn { return ctx.GetHistory().GetTurns() },
			wantCount: 4,
		},
		{
			name:      "last 2 turns",
			query:     func() []Turn { return ctx.GetHistory().Last(2) },
			wantCount: 2,
		},
		{
			name:      "filter by agent1",
			query:     func() []Turn { return ctx.GetHistory().FilterByAgent("agent1") },
			wantCount: 3,
		},
		{
			name:      "exclude hidden",
			query:     func() []Turn { return ctx.GetHistory().ExcludeHidden() },
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.query()
			if len(result) != tt.wantCount {
				t.Errorf("expected %d turns, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestBuildPromptIncludesMemoryFiles(t *testing.T) {
	withTempWorkingDir(t)

	baseDir := t.TempDir()
	ctx, err := New("test-id", "test description", "test instructions", baseDir)
	if err != nil {
		t.Fatalf("failed to create test context: %v", err)
	}
	if err := ctx.ExecutionEnvironment().SetMemoryDir(filepath.Join(ctx.ExecutionEnvironment().LLMDir, "memory")); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}

	memoryFileContent := "memory file content for testing"
	memoryFileName := "memory_test.txt"
	memoryFilePath := filepath.Join(ctx.ExecutionEnvironment().MemoryDir, memoryFileName)

	err = os.WriteFile(memoryFilePath, []byte(memoryFileContent), 0644)
	if err != nil {
		t.Fatalf("failed to create memory file: %v", err)
	}
	ctx.StartTurn("test user message")

	msgs, err := ctx.BuildPrompt([]ai.Tool{}, false)
	if err != nil {
		t.Fatalf("BuildPrompt failed: %v", err)
	}

	if len(msgs) == 0 {
		t.Fatal("expected at least one message")
	}

	sysMsg, ok := msgs[0].(ai.SystemMessage)
	if !ok {
		t.Fatalf("expected first message to be SystemMessage, got %T", msgs[0])
	}

	if !strings.Contains(sysMsg.Content, memoryFileContent) {
		t.Errorf("system message should contain memory file content %q, got: %s", memoryFileContent, sysMsg.Content)
	}

	if !strings.Contains(sysMsg.Content, memoryFileName) {
		t.Errorf("system message should contain memory file name %q, got: %s", memoryFileName, sysMsg.Content)
	}

	storeName := ctx.ExecutionEnvironment().MemoryStoreName()
	t.Cleanup(func() {
		document.UnregisterStore(storeName)
	})
}
