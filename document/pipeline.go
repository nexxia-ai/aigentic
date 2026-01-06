package document

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Pipeline is a DocumentProcessor that chains multiple processors together.
// Create a pipeline with NewPipeline(id, store), add stages with AddStage(), then call Run().
// Pipelines support pause/resume functionality and optional backing stores per stage.
type Pipeline struct {
	id         string
	stages     []Stage
	state      *PipelineState
	stateStore Store
	processors []DocumentProcessor
}

// NewPipeline creates a new empty pipeline with the given ID and state store.
func NewPipeline(id string, store Store) *Pipeline {
	return &Pipeline{
		id:     id,
		stages: make([]Stage, 0),
		state: &PipelineState{
			ID:           id,
			Status:       StatusPending,
			CurrentStage: 0,
			Stages:       make([]StageState, 0),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		stateStore: store,
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
		p.state.Stages = append(p.state.Stages, StageState{
			Name:         name,
			Status:       StatusPending,
			OutputDocIDs: make([]string, 0),
		})
	}
	return p
}

// State returns the current pipeline state.
func (p *Pipeline) State() *PipelineState {
	return p.state
}

// SaveState persists current state to stateStore.
func (p *Pipeline) SaveState(ctx context.Context) error {
	if p.stateStore == nil {
		return nil
	}

	p.state.UpdatedAt = time.Now()

	stateJSON, err := json.Marshal(p.state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	stateDoc := NewInMemoryDocument(p.id+"-state", p.id+".state.json", stateJSON, nil)
	if _, err := p.stateStore.Save(ctx, stateDoc); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// LoadState restores state from stateStore.
func (p *Pipeline) LoadState(ctx context.Context) error {
	if p.stateStore == nil {
		return fmt.Errorf("state store not set")
	}

	stateDoc, err := p.stateStore.Load(ctx, p.id+"-state")
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	stateData, err := stateDoc.Bytes()
	if err != nil {
		return fmt.Errorf("failed to get state bytes: %w", err)
	}

	var state PipelineState
	if err := json.Unmarshal(stateData, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	p.state = &state
	return nil
}

// Run executes the pipeline, can be interrupted via context.
func (p *Pipeline) Run(ctx context.Context, docs []*Document) ([]*Document, error) {
	if len(p.stages) == 0 {
		return docs, nil
	}

	p.state.Status = StatusRunning
	p.state.CurrentStage = 0

	if err := p.SaveState(ctx); err != nil {
		return nil, fmt.Errorf("failed to save initial state: %w", err)
	}

	return p.runFromStage(ctx, 0, docs)
}

// Resume continues from last completed stage.
func (p *Pipeline) Resume(ctx context.Context) ([]*Document, error) {
	if err := p.LoadState(ctx); err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if p.state.Status != StatusPaused {
		return nil, fmt.Errorf("pipeline is not paused, status: %s", p.state.Status)
	}

	var docs []*Document
	for i := p.state.CurrentStage - 1; i >= 0; i-- {
		if i < len(p.stages) {
			stage := p.stages[i]
			if stage.Store != nil && i < len(p.state.Stages) && p.state.Stages[i].Status == StatusCompleted {
				loadedDocs, err := stage.Store.List(ctx)
				if err == nil && len(loadedDocs) > 0 {
					docs = loadedDocs
					break
				}
			}
		}
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents found to resume from")
	}

	p.state.Status = StatusRunning
	if err := p.SaveState(ctx); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	return p.runFromStage(ctx, p.state.CurrentStage, docs)
}

func (p *Pipeline) runFromStage(ctx context.Context, startStage int, docs []*Document) ([]*Document, error) {
	for i := startStage; i < len(p.stages); i++ {
		select {
		case <-ctx.Done():
			p.state.Status = StatusPaused
			p.state.CurrentStage = i
			p.SaveState(ctx)
			return nil, ctx.Err()
		default:
		}

		stage := p.stages[i]
		if i < len(p.state.Stages) {
			p.state.Stages[i].Status = StatusRunning
		}

		var nextDocs []*Document
		for _, d := range docs {
			processed, err := stage.Processor.Process(d)
			if err != nil {
				if i < len(p.state.Stages) {
					p.state.Stages[i].Status = StatusFailed
					p.state.Stages[i].Error = err.Error()
				}
				p.state.Status = StatusFailed
				p.state.CurrentStage = i
				p.SaveState(ctx)
				return nil, fmt.Errorf("stage %s failed: %w", stage.Name, err)
			}
			nextDocs = append(nextDocs, processed...)
		}

		if stage.Store != nil {
			docIDs := make([]string, 0, len(nextDocs))
			for _, doc := range nextDocs {
				if _, err := stage.Store.Save(ctx, doc); err != nil {
					if i < len(p.state.Stages) {
						p.state.Stages[i].Status = StatusFailed
						p.state.Stages[i].Error = err.Error()
					}
					p.state.Status = StatusFailed
					p.state.CurrentStage = i
					p.SaveState(ctx)
					return nil, fmt.Errorf("failed to save document in stage %s: %w", stage.Name, err)
				}
				docIDs = append(docIDs, doc.ID())
			}

			if i < len(p.state.Stages) {
				p.state.Stages[i].OutputDocIDs = docIDs
			}
		}

		now := time.Now()
		if i < len(p.state.Stages) {
			p.state.Stages[i].Status = StatusCompleted
			p.state.Stages[i].CompletedAt = &now
		}

		docs = nextDocs
		p.state.CurrentStage = i + 1

		if err := p.SaveState(ctx); err != nil {
			return nil, fmt.Errorf("failed to save state: %w", err)
		}
	}

	p.state.Status = StatusCompleted
	p.state.UpdatedAt = time.Now()
	p.SaveState(ctx)

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
