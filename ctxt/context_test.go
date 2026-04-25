package ctxt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func writeTestDocument(ctx *AgentContext, path string, content []byte) (string, error) {
	ws := ctx.Workspace()
	if ws == nil {
		return "", os.ErrInvalid
	}
	fullPath := filepath.Join(ws.LLMDir, filepath.FromSlash(filepath.ToSlash(path)))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return "", err
	}
	return fullPath, nil
}

func attachTestDocument(ctx *AgentContext, path string, content []byte, mimeType string, includeInPrompt bool) error {
	fullPath, err := writeTestDocument(ctx, path, content)
	if err != nil {
		return err
	}
	if mimeType == "" {
		mimeType = document.DetectMimeTypeFromPath(fullPath)
	}
	ref := FileRef{
		Path:            fullPath,
		MimeType:        mimeType,
		IncludeInPrompt: includeInPrompt,
	}
	ref.SetMeta(map[string]string{"visible_to_user": "true"})
	return ctx.AddFile(ref)
}

func TestNew(t *testing.T) {
	ctx := createTestContext(t, "test-id", "test description", "test instructions")

	if ctx.ID() != "test-id" {
		t.Errorf("expected id 'test-id', got '%s'", ctx.ID())
	}
	if desc, ok := ctx.PromptPart(SystemPartKeyDescription); !ok || desc != "test description" {
		t.Errorf("expected description 'test description', got %q (ok=%v)", desc, ok)
	}
	if inst, ok := ctx.PromptPart(SystemPartKeyInstructions); !ok || inst != "test instructions" {
		t.Errorf("expected instructions 'test instructions', got %q (ok=%v)", inst, ok)
	}
	if ctx.conversationHistory == nil {
		t.Error("expected conversation history to be initialized")
	}
}

func TestSetSystemPart_OrderingUpsertRemove(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")

	ctx.SetSystemPart("extra", "v1")
	parts := ctx.SystemParts()
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts (description, instructions, extra), got %d", len(parts))
	}
	if parts[2].Key != "extra" || parts[2].Value != "v1" {
		t.Errorf("expected last part extra=v1, got %s=%s", parts[2].Key, parts[2].Value)
	}

	ctx.SetSystemPart("extra", "v2")
	if v, ok := ctx.PromptPart("extra"); !ok || v != "v2" {
		t.Errorf("upsert: expected extra=v2, got %q (ok=%v)", v, ok)
	}

	ctx.SetSystemPart("extra", "")
	if _, ok := ctx.PromptPart("extra"); ok {
		t.Error("expected extra to be removed when value is empty")
	}
	if len(ctx.SystemParts()) != 2 {
		t.Errorf("expected 2 parts after remove, got %d", len(ctx.SystemParts()))
	}
}

func TestChildContextAddFileRefCarriesIntoTurn(t *testing.T) {
	baseDir := t.TempDir()
	parent, err := New("parent-run", "desc", "inst", baseDir)
	require.NoError(t, err)

	fullPath, err := writeTestDocument(parent, "uploads/test.png", []byte("image-bytes"))
	require.NoError(t, err)

	privateDir := filepath.Join(parent.Workspace().PrivateDir, "batch", "item-0")
	child, err := NewChild("child-run", "child desc", "child inst", privateDir, parent.Workspace().LLMDir, parent.BasePath())
	require.NoError(t, err)

	err = child.AddFileRef(fullPath, true, "image/png")
	require.NoError(t, err)

	turn := child.StartTurn("Process uploads/test.png", "")
	require.Len(t, turn.Files, 1)
	require.Equal(t, fullPath, turn.Files[0].Path)
	require.True(t, turn.Files[0].IncludeInPrompt)
	require.Equal(t, "image/png", turn.Files[0].MimeType)
}

func TestAttachDocumentAndAddFileRefDedupesPendingRef(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")

	fullPath, err := writeTestDocument(ctx, "uploads/test.png", []byte("image-bytes"))
	require.NoError(t, err)
	ref := FileRef{
		Path:            fullPath,
		MimeType:        "image/png",
		IncludeInPrompt: true,
	}
	ref.SetMeta(map[string]string{"visible_to_user": "true"})
	err = ctx.AddFile(ref)
	require.NoError(t, err)
	err = ctx.AddFileRef(fullPath, true, "")
	require.NoError(t, err)

	turn := ctx.StartTurn("Process uploads/test.png", "")
	require.Len(t, turn.Files, 1)
	require.Equal(t, fullPath, turn.Files[0].Path)
	require.True(t, turn.Files[0].IncludeInPrompt)
	require.Equal(t, "image/png", turn.Files[0].MimeType)
	require.Equal(t, "true", turn.Files[0].GetMeta("visible_to_user"))
}

func TestChainableMethods(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")
	ctx.SetSystemPart(SystemPartKeyOutputInstructions, "Use JSON")
	if err := attachTestDocument(ctx, "uploads/test1.pdf", []byte("content1"), "", false); err != nil {
		t.Fatalf("attachTestDocument: %v", err)
	}
	if err := attachTestDocument(ctx, "uploads/test2.pdf", []byte("content2"), "", false); err != nil {
		t.Fatalf("attachTestDocument: %v", err)
	}

	if out, ok := ctx.PromptPart(SystemPartKeyOutputInstructions); !ok || out != "Use JSON" {
		t.Errorf("expected output instructions 'Use JSON', got %q (ok=%v)", out, ok)
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
			name: "clear history",
			setup: func(ctx *AgentContext) {
				appendTestTurns(ctx, 2)
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
	ctx := createTestContext(t, "id", "desc", "inst")
	appendTestTurns(ctx, 1)

	ctx.ClearAll()

	if ctx.GetHistory().Len() != 0 {
		t.Errorf("expected 0 history turns after ClearAll, got %d", ctx.GetHistory().Len())
	}
}

func TestSetMethods(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")

	ctx.SetDescription("new description").
		SetInstructions("new instructions").
		SetSystemPart(SystemPartKeyOutputInstructions, "new output instructions")

	if desc, ok := ctx.PromptPart(SystemPartKeyDescription); !ok || desc != "new description" {
		t.Errorf("expected description 'new description', got %q (ok=%v)", desc, ok)
	}
	if inst, ok := ctx.PromptPart(SystemPartKeyInstructions); !ok || inst != "new instructions" {
		t.Errorf("expected instructions 'new instructions', got %q (ok=%v)", inst, ok)
	}
	if out, ok := ctx.PromptPart(SystemPartKeyOutputInstructions); !ok || out != "new output instructions" {
		t.Errorf("expected output instructions 'new output instructions', got %q (ok=%v)", out, ok)
	}
}

func appendTestTurns(ctx *AgentContext, n int) {
	ledger := ctx.Ledger()
	if ledger == nil {
		return
	}
	for i := 0; i < n; i++ {
		turnID, _, err := ledger.PrepareTurn(time.Now())
		if err != nil {
			return
		}
		ctx.GetHistory().appendTurn(Turn{TurnID: turnID})
	}
}

func TestHistoryQuery(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")
	ledger := ctx.Ledger()
	require.NotNil(t, ledger)

	turn1 := Turn{AgentName: "agent1", Hidden: false}
	turn2 := Turn{AgentName: "agent2", Hidden: false}
	turn3 := Turn{AgentName: "agent1", Hidden: true}
	turn4 := Turn{AgentName: "agent1", Hidden: false}
	for _, turn := range []Turn{turn1, turn2, turn3, turn4} {
		turnID, _, err := ledger.PrepareTurn(time.Now())
		if err != nil {
			t.Fatalf("PrepareTurn: %v", err)
		}
		turn.TurnID = turnID
		ctx.GetHistory().appendTurn(turn)
	}

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
	if err := ctx.Workspace().SetMemoryDir(filepath.Join(ctx.Workspace().LLMDir, "memory")); err != nil {
		t.Fatalf("SetMemoryDir: %v", err)
	}

	memoryFileContent := "memory file content for testing"
	memoryFileName := "memory_test.txt"
	memoryFilePath := filepath.Join(ctx.Workspace().MemoryDir, memoryFileName)

	err = os.WriteFile(memoryFilePath, []byte(memoryFileContent), 0644)
	if err != nil {
		t.Fatalf("failed to create memory file: %v", err)
	}
	ctx.StartTurn("test user message", "")

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

	if strings.Contains(sysMsg.Content, memoryFileContent) {
		t.Errorf("system message should not contain memory file content %q, got: %s", memoryFileContent, sysMsg.Content)
	}

	foundContextMap := false
	for _, msg := range msgs {
		userMsg, ok := msg.(ai.UserMessage)
		if !ok || !strings.Contains(userMsg.Content, "<context_map>") {
			continue
		}
		foundContextMap = true
		if !strings.Contains(userMsg.Content, memoryFileName) {
			t.Errorf("context map should contain memory file name %q, got: %s", memoryFileName, userMsg.Content)
		}
	}
	if !foundContextMap {
		t.Fatal("expected context map message for memory files")
	}

	storeName := ctx.Workspace().MemoryStoreName()
	t.Cleanup(func() {
		document.UnregisterStore(storeName)
	})
}

func TestEndTurnPersistsStartTurnRequestSnapshot(t *testing.T) {
	ctx := createTestContext(t, "id", "desc", "inst")
	require.NoError(t, ctx.AddFile(FileRef{
		Path:            "uploads/original.txt",
		MimeType:        "text/plain",
		IncludeInPrompt: false,
		Role:            FileRoleUserUpload,
	}))

	ctx.StartTurn("hello", "")
	ctx.Turn().AddFile(FileRef{
		Path:            "output/tool-artifact.txt",
		MimeType:        "text/plain",
		IncludeInPrompt: false,
		Role:            FileRoleToolArtifact,
	})
	ctx.Turn().AddFile(FileRef{
		Path:            "output/tool-artifact-2.txt",
		MimeType:        "text/plain",
		IncludeInPrompt: false,
		Role:            FileRoleToolArtifact,
	})
	ctx.EndTurn(ai.AIMessage{Role: ai.AssistantRole, Content: "done"})

	turns := ctx.GetHistory().GetTurns()
	require.Len(t, turns, 1)
	req, ok := turns[0].Request.(ai.UserMessage)
	require.True(t, ok)
	assert.Contains(t, req.Content, "uploads/original.txt")
	assert.NotContains(t, req.Content, "output/tool-artifact.txt")
	assert.NotContains(t, req.Content, "output/tool-artifact-2.txt")
}
