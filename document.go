package aigentic

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

// DocumentStore interface for any storage backend
type DocumentStore interface {
	Open(ctx context.Context, filePath string) (*Document, error)
	Close(ctx context.Context) error
}

type DocumentProcessor interface {
	Process(doc *Document) ([]*Document, error)
}

// Document is a common type to work with documents. You can load documents using the DocumentStore interface.
// Pass documents to agents and the agents will handle the inclusion of the document in the context.
type Document struct {
	id        string
	Filename  string
	FilePath  string
	FileSize  int64
	MimeType  string
	CreatedAt time.Time

	// Private content field to enable lazy loading
	binary []byte

	Selected bool

	// Chunking metadata - used when this is part of another document
	SourceDoc   *Document
	ChunkIndex  int
	TotalChunks int
	StartChar   int
	EndChar     int
	PageNumber  int

	// loader loads the document from the store
	// it must be implemented by the store provider
	loader func(*Document) ([]byte, error)
}

func (d *Document) ID() string {
	if d.id != "" {
		return d.id
	}
	d.id = fmt.Sprintf("doc_%s", filepath.Base(d.FilePath))
	return d.id
}

// Bytes returns the binary data of the document
func (d *Document) Bytes() ([]byte, error) {
	if d.binary != nil {
		return d.binary, nil
	}

	if d.loader == nil {
		return nil, fmt.Errorf("loader not implemented")
	}

	var err error
	d.binary, err = d.loader(d)
	if err != nil {
		return nil, err
	}

	return d.binary, nil
}

func (d *Document) IsChunk() bool {
	return d.SourceDoc != nil
}

func (d *Document) SetLoader(loader func(*Document) ([]byte, error)) {
	d.loader = loader
}

func NewInMemoryDocument(id, filename string, data []byte, srcDoc *Document) *Document {
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	return &Document{
		id:         id,
		Filename:   filename,
		FilePath:   filename,
		FileSize:   int64(len(data)),
		MimeType:   mimeType,
		binary:     data,
		Selected:   false,
		SourceDoc:  srcDoc,
		ChunkIndex: -1,
	}
}

// deriveTypeFromMime derives the resource type from MIME type
func deriveTypeFromMime(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	case mimeType == "application/pdf":
		return "document"
	default:
		return "document"
	}
}
