package document

import (
	"context"
	"fmt"
)

// Pipeline is a DocumentProcessor that chains multiple processors together.
// Create a pipeline with NewPipeline(), add stages with AddStage(), then call Run().
// Pipelines support optional backing stores per stage.
type Pipeline struct {
	id         string
	stages     []Stage
	processors []DocumentProcessor
}

// NewPipeline creates a new empty pipeline with the given ID.
func NewPipeline(id string) *Pipeline {
	return &Pipeline{
		id:         id,
		stages:     make([]Stage, 0),
		processors: make([]DocumentProcessor, 0),
	}
}

// AddStage adds a stage with optional backing store (store can be nil) and returns the pipeline for chaining.
func (p *Pipeline) AddStage(name string, processor DocumentProcessor, store Store) *Pipeline {
	if processor != nil {
		p.stages = append(p.stages, Stage{
			Name:      name,
			Processor: processor,
			Store:     store,
		})
	}
	return p
}

// Run executes the pipeline, can be interrupted via context.
func (p *Pipeline) Run(ctx context.Context, docs []*Document) ([]*Document, error) {
	if len(p.stages) == 0 {
		return docs, nil
	}

	for i := 0; i < len(p.stages); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		stage := p.stages[i]

		var nextDocs []*Document
		for _, d := range docs {
			processed, err := stage.Processor.Process(d)
			if err != nil {
				return nil, fmt.Errorf("stage %s failed: %w", stage.Name, err)
			}
			nextDocs = append(nextDocs, processed...)
		}

		if stage.Store != nil {
			for _, doc := range nextDocs {
				if _, err := stage.Store.Save(ctx, doc); err != nil {
					return nil, fmt.Errorf("failed to save document in stage %s: %w", stage.Name, err)
				}
			}
		}

		docs = nextDocs
	}

	return docs, nil
}

// Add adds a processor to the pipeline and returns the pipeline for chaining.
// This is kept for backward compatibility. For new code, use AddStage.
func (p *Pipeline) Add(processor DocumentProcessor) *Pipeline {
	if processor != nil {
		p.processors = append(p.processors, processor)
	}
	return p
}

// Process implements DocumentProcessor interface.
// It applies each processor in sequence to the input document(s).
// Each processor may return multiple documents, which are then processed by the next processor.
// This uses the legacy processors list. For new code, use Run with stages.
func (p *Pipeline) Process(doc *Document) ([]*Document, error) {
	docs := []*Document{doc}

	for _, processor := range p.processors {
		var nextDocs []*Document
		for _, d := range docs {
			processed, err := processor.Process(d)
			if err != nil {
				return nil, err
			}
			nextDocs = append(nextDocs, processed...)
		}
		docs = nextDocs
	}

	return docs, nil
}
