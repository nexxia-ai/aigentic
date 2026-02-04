package document

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

// Document is a common type to work with documents. You can load documents using the DocumentStore interface.
// Pass documents to agents and the agents will handle the inclusion of the document in the context.
type Document struct {
	id        string
	Filename  string    `json:"filename"`
	FilePath  string    `json:"file_path"`
	URL       string    `json:"url,omitempty"`
	FileSize  int64     `json:"file_size"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`

	Selected bool `json:"-"`

	// Chunking metadata - used when this is part of another document
	SourceDoc   *Document `json:"-"`
	SourceDocID string    `json:"source_doc_id,omitempty"`
	ChunkIndex  int       `json:"chunk_index,omitempty"`
	TotalChunks int       `json:"total_chunks,omitempty"`
	StartChar   int       `json:"-"`
	EndChar     int       `json:"-"`
	PageNumber  int       `json:"-"`

	// store is the backing store for this document
	store Store `json:"-"`
}

func (d *Document) ID() string {
	if d.id != "" {
		return d.id
	}
	d.id = fmt.Sprintf("doc_%s", filepath.Base(d.FilePath))
	return d.id
}

func (d *Document) SetID(id string) {
	d.id = id
}

func (d *Document) Reader() (io.ReadCloser, error) {
	if d.store == nil {
		return nil, fmt.Errorf("document has no backing store")
	}

	reader, err := d.store.Open(context.Background(), d.ID())
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// Bytes returns the binary data of the document
func (d *Document) Bytes() ([]byte, error) {
	reader, err := d.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func (d *Document) Text() string {
	b, err := d.Bytes()
	if err != nil {
		slog.Error("failed to get document bytes", "error", err)
	}
	return string(b)
}

func (d *Document) IsChunk() bool {
	return d.SourceDoc != nil
}

func (d *Document) Store() Store {
	return d.store
}

func NewInMemoryDocument(id, filename string, data []byte, srcDoc *Document) *Document {
	return NewDocument(defaultMemoryStore, id, filename, data, srcDoc)
}

func NewDocument(store Store, id, filename string, data []byte, srcDoc *Document) *Document {
	mimeType := DetectMimeTypeFromPath(filename)

	docID := id
	if docID == "" {
		docID = fmt.Sprintf("doc_%s", filepath.Base(filename))
	}

	store.Create(context.Background(), docID, bytes.NewReader(data))

	doc := &Document{
		id:         docID,
		Filename:   filename,
		FilePath:   filename,
		FileSize:   int64(len(data)),
		MimeType:   mimeType,
		Selected:   false,
		SourceDoc:  srcDoc,
		ChunkIndex: -1,
		store:      store,
		CreatedAt:  time.Now(),
	}
	return doc
}

// DeriveTypeFromMime derives the resource type from MIME type
func DeriveTypeFromMime(mimeType string) string {
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

// prepareForSerialization prepares the document for JSON serialization by setting SourceDocID
func (d *Document) prepareForSerialization() {
	if d.SourceDoc != nil && d.SourceDocID == "" {
		d.SourceDocID = d.SourceDoc.ID()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	if d.id == "" {
		d.id = fmt.Sprintf("doc_%s", filepath.Base(d.FilePath))
	}
}

// MarshalJSON customizes JSON serialization to include ID and prepare document
func (d *Document) MarshalJSON() ([]byte, error) {
	type Alias Document
	d.prepareForSerialization()
	return json.Marshal(&struct {
		ID string `json:"id"`
		*Alias
	}{
		ID:    d.ID(),
		Alias: (*Alias)(d),
	})
}

// UnmarshalJSON customizes JSON deserialization to handle ID field
func (d *Document) UnmarshalJSON(data []byte) error {
	type Alias Document
	aux := &struct {
		ID string `json:"id"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ID != "" {
		d.id = aux.ID
	}
	return nil
}
