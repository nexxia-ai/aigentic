package ctxt

import (
	"testing"

	"github.com/nexxia-ai/aigentic/document"
	"github.com/stretchr/testify/assert"
)

func TestAddDocument(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)

	err := ctx.AddDocument(doc)
	assert.NoError(t, err)
	assert.Len(t, ctx.documents, 1)
	assert.Equal(t, doc, ctx.documents[0])
}

func TestAddDocumentNil(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	err := ctx.AddDocument(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document cannot be nil")
	assert.Len(t, ctx.documents, 0)
}

func TestAddMultipleDocuments(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)
	doc3 := document.NewInMemoryDocument("doc3", "test3.png", []byte("content 3"), nil)

	err := ctx.AddDocument(doc1)
	assert.NoError(t, err)

	err = ctx.AddDocument(doc2)
	assert.NoError(t, err)

	err = ctx.AddDocument(doc3)
	assert.NoError(t, err)

	assert.Len(t, ctx.documents, 3)
	assert.Equal(t, doc1, ctx.documents[0])
	assert.Equal(t, doc2, ctx.documents[1])
	assert.Equal(t, doc3, ctx.documents[2])
}

func TestDeleteDocument(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)
	doc3 := document.NewInMemoryDocument("doc3", "test3.png", []byte("content 3"), nil)

	ctx.AddDocument(doc1)
	ctx.AddDocument(doc2)
	ctx.AddDocument(doc3)

	assert.Len(t, ctx.documents, 3)

	err := ctx.DeleteDocument(doc2)
	assert.NoError(t, err)
	assert.Len(t, ctx.documents, 2)
	assert.Equal(t, doc1, ctx.documents[0])
	assert.Equal(t, doc3, ctx.documents[1])
}

func TestDeleteDocumentNil(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)
	ctx.AddDocument(doc)

	err := ctx.DeleteDocument(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document cannot be nil")
	assert.Len(t, ctx.documents, 1)
}

func TestDeleteDocumentNotFound(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)

	ctx.AddDocument(doc1)

	err := ctx.DeleteDocument(doc2)
	assert.NoError(t, err)
	assert.Len(t, ctx.documents, 1)
	assert.Equal(t, doc1, ctx.documents[0])
}

func TestDeleteDocumentFromEmpty(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc := document.NewInMemoryDocument("doc1", "test.pdf", []byte("test content"), nil)

	err := ctx.DeleteDocument(doc)
	assert.NoError(t, err)
	assert.Len(t, ctx.documents, 0)
}

func TestAddAndDeleteDocuments(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)
	doc3 := document.NewInMemoryDocument("doc3", "test3.png", []byte("content 3"), nil)

	ctx.AddDocument(doc1)
	ctx.AddDocument(doc2)
	ctx.AddDocument(doc3)
	assert.Len(t, ctx.documents, 3)

	ctx.DeleteDocument(doc1)
	assert.Len(t, ctx.documents, 2)
	assert.Equal(t, doc2, ctx.documents[0])
	assert.Equal(t, doc3, ctx.documents[1])

	ctx.DeleteDocument(doc3)
	assert.Len(t, ctx.documents, 1)
	assert.Equal(t, doc2, ctx.documents[0])

	ctx.DeleteDocument(doc2)
	assert.Len(t, ctx.documents, 0)
}

func TestDeleteDocumentByID(t *testing.T) {
	ctx := NewAgentContext("test-id", "test description", "test instructions")

	doc1 := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("content 1"), nil)
	doc2 := document.NewInMemoryDocument("doc2", "test2.txt", []byte("content 2"), nil)

	ctx.AddDocument(doc1)
	ctx.AddDocument(doc2)

	doc1Copy := document.NewInMemoryDocument("doc1", "test1.pdf", []byte("different content"), nil)

	err := ctx.DeleteDocument(doc1Copy)
	assert.NoError(t, err)
	assert.Len(t, ctx.documents, 1)
	assert.Equal(t, doc2, ctx.documents[0])
}

