package document

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStore concrete implementation of FileStore
type LocalStore struct {
	dir string
}

var _ Store = &LocalStore{}

// NewLocalStore creates a new LocalStore
func NewLocalStore(dir string) *LocalStore {
	// remove the trailing slash
	dir = strings.TrimSuffix(dir, string(os.PathSeparator))
	return &LocalStore{dir: dir}
}

// Open implements FileStore interface - just creates Document with metadata
func (ls *LocalStore) OpenOLD(ctx context.Context, id string) (*Document, error) {
	if !strings.HasPrefix(id, ls.dir) {
		id = filepath.Join(ls.dir, id)
	}

	stat, err := os.Stat(id)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Determine MIME type and document type
	mimeType := mime.TypeByExtension(filepath.Ext(id))

	// Create document with empty content for lazy loading
	doc := NewInMemoryDocument(
		filepath.Base(id),
		filepath.Base(id),
		nil, // Empty content for lazy loading
		nil,
	)

	// Store relative path for lazy loading
	relativePath := strings.TrimPrefix(id, ls.dir+string(os.PathSeparator))
	doc.FilePath = relativePath
	doc.FileSize = stat.Size()
	doc.MimeType = mimeType

	// Set the loader for lazy loading
	doc.SetLoader(ls.load)

	return doc, nil
}

// Close implements FileStore interface
func (ls *LocalStore) Close(ctx context.Context) error {
	// No cleanup needed for local files
	return nil
}

// Add copies a file to the store location
func (ls *LocalStore) Add(ctx context.Context, filePath string) (*Document, error) {
	// For now, just return the document without copying
	// In a full implementation, this would copy the file to basePath
	return ls.Load(ctx, filePath)
}

// Delete removes a document from the store by ID (implements Store interface)
func (ls *LocalStore) Delete(ctx context.Context, id string) error {
	contentPath := filepath.Join(ls.dir, id+".content")
	metaPath := filepath.Join(ls.dir, id+".meta.json")

	if err := os.Remove(contentPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete content: %w", err)
	}

	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}

// Save saves a document to the store
func (ls *LocalStore) Save(ctx context.Context, doc *Document) (*Document, error) {
	if err := os.MkdirAll(ls.dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	docID := doc.ID()
	contentPath := filepath.Join(ls.dir, docID)
	metaPath := filepath.Join(ls.dir, docID+".meta.json")

	data, err := doc.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get document bytes: %w", err)
	}

	if err := os.WriteFile(contentPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write content: %w", err)
	}

	meta := DocumentMetadata{
		ID:          docID,
		Filename:    doc.Filename,
		FilePath:    docID,
		MimeType:    doc.MimeType,
		FileSize:    int64(len(data)),
		CreatedAt:   doc.CreatedAt,
		SourceDocID: "",
		ChunkIndex:  doc.ChunkIndex,
		TotalChunks: doc.TotalChunks,
	}

	if doc.SourceDoc != nil {
		meta.SourceDocID = doc.SourceDoc.ID()
	}

	if doc.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	return doc, nil
}

// Load loads a document from the store by ID
func (ls *LocalStore) Load(ctx context.Context, id string) (*Document, error) {
	if strings.HasPrefix(id, ls.dir) {
		id = strings.TrimPrefix(id, ls.dir+string(os.PathSeparator))
	}

	metaPath := filepath.Join(ls.dir, id+".meta.json")
	contentPath := filepath.Join(ls.dir, id)

	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		var meta DocumentMetadata
		if err := json.Unmarshal(metaData, &meta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		content, err := os.ReadFile(contentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read content: %w", err)
		}

		doc := NewInMemoryDocument(meta.ID, meta.Filename, content, nil)
		doc.FilePath = meta.FilePath
		doc.FileSize = meta.FileSize
		doc.MimeType = meta.MimeType
		doc.CreatedAt = meta.CreatedAt
		doc.ChunkIndex = meta.ChunkIndex
		doc.TotalChunks = meta.TotalChunks

		if meta.SourceDocID != "" {
			sourceDoc, err := ls.Load(ctx, meta.SourceDocID)
			if err == nil {
				doc.SourceDoc = sourceDoc
			}
		}

		doc.SetLoader(ls.load)
		return doc, nil
	}

	filePath := filepath.Join(ls.dir, id)
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find document: %w", err)
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("document ID %s is a directory", id)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	doc := NewInMemoryDocument(id, filepath.Base(id), content, nil)
	doc.FilePath = filepath.Base(id)
	doc.FileSize = stat.Size()
	doc.MimeType = mime.TypeByExtension(filepath.Ext(id))
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = stat.ModTime()
	}

	doc.SetLoader(ls.load)
	return doc, nil
}

// List returns all documents in the store (implements Store interface)
func (ls *LocalStore) List(ctx context.Context) ([]*Document, error) {
	var documents []*Document
	processed := make(map[string]bool)

	entries, err := os.ReadDir(ls.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	metaFiles := make(map[string]string)
	sourceFiles := make(map[string]os.DirEntry)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".meta.json") {
			docID := strings.TrimSuffix(name, ".meta.json")
			metaFiles[docID] = name
		} else {
			sourceFiles[name] = entry
		}
	}

	for docID, metaFileName := range metaFiles {
		metaPath := filepath.Join(ls.dir, metaFileName)
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var meta DocumentMetadata
		if err := json.Unmarshal(metaData, &meta); err != nil {
			continue
		}

		contentPath := filepath.Join(ls.dir, docID)
		content, err := os.ReadFile(contentPath)
		if err != nil {
			if os.IsNotExist(err) {
				doc := NewInMemoryDocument(meta.ID, meta.Filename, nil, nil)
				doc.FilePath = meta.FilePath
				doc.FileSize = meta.FileSize
				doc.MimeType = meta.MimeType
				doc.CreatedAt = meta.CreatedAt
				doc.ChunkIndex = meta.ChunkIndex
				doc.TotalChunks = meta.TotalChunks
				doc.SetLoader(ls.load)
				documents = append(documents, doc)
				processed[docID] = true
				processed[metaFileName] = true
			}
			continue
		}

		doc := NewInMemoryDocument(meta.ID, meta.Filename, content, nil)
		doc.FilePath = meta.FilePath
		doc.FileSize = meta.FileSize
		doc.MimeType = meta.MimeType
		doc.CreatedAt = meta.CreatedAt
		doc.ChunkIndex = meta.ChunkIndex
		doc.TotalChunks = meta.TotalChunks

		if meta.SourceDocID != "" {
			sourceDoc, err := ls.Load(ctx, meta.SourceDocID)
			if err == nil {
				doc.SourceDoc = sourceDoc
			}
		}

		doc.SetLoader(ls.load)
		documents = append(documents, doc)
		processed[docID] = true
		processed[metaFileName] = true
	}

	for name, entry := range sourceFiles {
		if processed[name] {
			continue
		}

		filePath := filepath.Join(ls.dir, name)
		stat, err := entry.Info()
		if err != nil {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		mimeType := mime.TypeByExtension(filepath.Ext(name))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		doc := NewInMemoryDocument(name, name, content, nil)
		doc.FilePath = name
		doc.FileSize = stat.Size()
		doc.MimeType = mimeType
		doc.CreatedAt = stat.ModTime()
		doc.SetLoader(ls.load)

		documents = append(documents, doc)
		processed[name] = true
	}

	return documents, nil
}

// Private method for lazy loading
func (ls *LocalStore) load(d *Document) ([]byte, error) {
	// Read file from filesystem
	if d.FilePath != "" {
		// Construct full path by joining base directory with relative file path
		fullPath := filepath.Join(ls.dir, d.FilePath)
		if data, err := os.ReadFile(fullPath); err == nil {
			return data, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}
