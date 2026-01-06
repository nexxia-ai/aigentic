package document

import (
	"context"
)

// Store provides read/write access to documents
type Store interface {
	Save(ctx context.Context, doc *Document) (*Document, error)
	Load(ctx context.Context, id string) (*Document, error)
	List(ctx context.Context) ([]*Document, error)
	Delete(ctx context.Context, id string) error
}
