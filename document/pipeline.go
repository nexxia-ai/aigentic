package document

import (
	"context"
	"fmt"
)

// Pipeline is a DocumentProcessor that chains multiple processors together.
// Create a pipeline with NewPipeline(), add processors with Add(), then call Run().
type Pipeline struct {
	id         string
	processors []DocumentProcessor
}

// NewPipeline creates a new empty pipeline with the given ID.
func NewPipeline(id string) *Pipeline {
	return &Pipeline{
		id:         id,
		processors: make([]DocumentProcessor, 0),
	}
}

// Run executes the pipeline, can be interrupted via context.
func (p *Pipeline) Run(ctx context.Context, docs []*Document) ([]*Document, error) {
	if len(p.processors) == 0 {
		return docs, nil
	}

	for _, processor := range p.processors {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var nextDocs []*Document
		for _, d := range docs {
			processed, err := processor.Process(d)
			if err != nil {
				return nil, fmt.Errorf("processor failed: %w", err)
			}
			nextDocs = append(nextDocs, processed...)
		}

		docs = nextDocs
	}

	return docs, nil
}

// Add adds a processor to the pipeline and returns the pipeline for chaining.
func (p *Pipeline) Add(processor DocumentProcessor) *Pipeline {
	if processor != nil {
		p.processors = append(p.processors, processor)
	}
	return p
}

// Process implements DocumentProcessor interface.
// It applies each processor in sequence to the input document.
// Each processor may return multiple documents, which are then processed by the next processor.
func (p *Pipeline) Process(doc *Document) ([]*Document, error) {
	ctx := context.Background()
	return p.Run(ctx, []*Document{doc})
}
