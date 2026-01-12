package ctxt

import (
	"testing"

	"github.com/nexxia-ai/aigentic/document"
	"github.com/stretchr/testify/assert"
)

func TestConversationTurnAddDocument(t *testing.T) {
	ac, _ := New("test", "test", "test", "")
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)

	err := turn.AddDocument("tool1", doc)
	assert.NoError(t, err)
	assert.Len(t, turn.Documents, 1)
	assert.Equal(t, doc, turn.Documents[0].Document)
	assert.Equal(t, "tool1", turn.Documents[0].ToolID)
}

func TestConversationTurnAddDocumentNil(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	err := turn.AddDocument("tool1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document cannot be nil")
	assert.Len(t, turn.Documents, 0)
}

func TestConversationTurnAddDocumentEmptyToolID(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)

	err := turn.AddDocument("", doc)
	assert.NoError(t, err)
	assert.Len(t, turn.Documents, 1)
	assert.Equal(t, doc, turn.Documents[0].Document)
	assert.Equal(t, "", turn.Documents[0].ToolID)
}

func TestConversationTurnAddMultipleDocuments(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)
	doc3 := document.NewInMemoryDocument("doc3", "test3.png", []byte("content 3"), nil)

	err := turn.AddDocument("tool1", doc1)
	assert.NoError(t, err)

	err = turn.AddDocument("tool2", doc2)
	assert.NoError(t, err)

	err = turn.AddDocument("tool1", doc3)
	assert.NoError(t, err)

	assert.Len(t, turn.Documents, 3)
	assert.Equal(t, doc1, turn.Documents[0].Document)
	assert.Equal(t, "tool1", turn.Documents[0].ToolID)
	assert.Equal(t, doc2, turn.Documents[1].Document)
	assert.Equal(t, "tool2", turn.Documents[1].ToolID)
	assert.Equal(t, doc3, turn.Documents[2].Document)
	assert.Equal(t, "tool1", turn.Documents[2].ToolID)
}

func TestConversationTurnDeleteDocument(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)
	doc3 := document.NewInMemoryDocument("doc3", "test3.png", []byte("content 3"), nil)

	turn.AddDocument("tool1", doc1)
	turn.AddDocument("tool2", doc2)
	turn.AddDocument("tool1", doc3)

	assert.Len(t, turn.Documents, 3)

	err := turn.DeleteDocument(doc2)
	assert.NoError(t, err)
	assert.Len(t, turn.Documents, 2)
	assert.Equal(t, doc1, turn.Documents[0].Document)
	assert.Equal(t, doc3, turn.Documents[1].Document)
}

func TestConversationTurnDeleteDocumentNil(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)
	turn.AddDocument("tool1", doc)

	err := turn.DeleteDocument(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document cannot be nil")
	assert.Len(t, turn.Documents, 1)
}

func TestConversationTurnDeleteDocumentNotFound(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)

	turn.AddDocument("tool1", doc1)

	err := turn.DeleteDocument(doc2)
	assert.NoError(t, err)
	assert.Len(t, turn.Documents, 1)
	assert.Equal(t, doc1, turn.Documents[0].Document)
}

func TestConversationTurnDeleteDocumentFromEmpty(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)

	err := turn.DeleteDocument(doc)
	assert.NoError(t, err)
	assert.Len(t, turn.Documents, 0)
}

func TestConversationTurnAddAndDeleteDocuments(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)
	doc3 := document.NewInMemoryDocument("doc3", "test3.png", []byte("content 3"), nil)

	turn.AddDocument("tool1", doc1)
	turn.AddDocument("tool2", doc2)
	turn.AddDocument("tool1", doc3)
	assert.Len(t, turn.Documents, 3)

	turn.DeleteDocument(doc1)
	assert.Len(t, turn.Documents, 2)
	assert.Equal(t, doc2, turn.Documents[0].Document)
	assert.Equal(t, doc3, turn.Documents[1].Document)

	turn.DeleteDocument(doc3)
	assert.Len(t, turn.Documents, 1)
	assert.Equal(t, doc2, turn.Documents[0].Document)

	turn.DeleteDocument(doc2)
	assert.Len(t, turn.Documents, 0)
}

func TestConversationTurnDeleteDocumentByID(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)

	turn.AddDocument("tool1", doc1)
	turn.AddDocument("tool2", doc2)

	doc1Copy := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("different content"), nil)

	err := turn.DeleteDocument(doc1Copy)
	assert.NoError(t, err)
	assert.Len(t, turn.Documents, 1)
	assert.Equal(t, doc2, turn.Documents[0].Document)
}

func TestConversationTurnAddDocumentWithDifferentToolIDs(t *testing.T) {
	ac, _ := New("test", "test", "test", t.TempDir())
	turn := NewTurn(ac, "test message", "agent1", "turn-001")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)

	turn.AddDocument("tool1", doc)
	turn.AddDocument("tool2", doc)

	assert.Len(t, turn.Documents, 2)
	assert.Equal(t, doc, turn.Documents[0].Document)
	assert.Equal(t, "tool1", turn.Documents[0].ToolID)
	assert.Equal(t, doc, turn.Documents[1].Document)
	assert.Equal(t, "tool2", turn.Documents[1].ToolID)
}
