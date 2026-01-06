package document

import (
	"context"
	"os"
	"testing"
	"time"
)

type mockProcessor struct {
	result  []*Document
	process func(*Document) ([]*Document, error)
}

func (m *mockProcessor) Process(doc *Document) ([]*Document, error) {
	if m.process != nil {
		return m.process(doc)
	}
	return m.result, nil
}

func TestPipeline_EmptyPipeline(t *testing.T) {
	pipeline := NewPipeline("test-empty", nil)

	doc := NewInMemoryDocument("test1", "test.txt", []byte("content"), nil)

	results, err := pipeline.Process(doc)
	if err != nil {
		t.Fatalf("Pipeline.Process() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0] != doc {
		t.Errorf("expected original document, got different document")
	}
}

func TestPipeline_SingleProcessor(t *testing.T) {
	pipeline := NewPipeline("test-single", nil)

	resultDoc := NewInMemoryDocument("chunk1", "chunk.txt", []byte("chunk content"), nil)
	processor := &mockProcessor{
		result: []*Document{resultDoc},
	}

	pipeline.Add(processor)

	doc := NewInMemoryDocument("test1", "test.txt", []byte("content"), nil)
	results, err := pipeline.Process(doc)

	if err != nil {
		t.Fatalf("Pipeline.Process() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Filename != "chunk.txt" {
		t.Errorf("expected filename 'chunk.txt', got '%s'", results[0].Filename)
	}
}

func TestPipeline_MultipleProcessors(t *testing.T) {
	pipeline := NewPipeline("test-multiple", nil)

	processor1 := &mockProcessor{
		process: func(doc *Document) ([]*Document, error) {
			return []*Document{
				NewInMemoryDocument("intermediate1", "i1.txt", []byte("i1"), doc),
				NewInMemoryDocument("intermediate2", "i2.txt", []byte("i2"), doc),
			}, nil
		},
	}

	processor2 := &mockProcessor{
		process: func(doc *Document) ([]*Document, error) {
			return []*Document{
				NewInMemoryDocument("final", "final.txt", []byte(doc.Filename), doc.SourceDoc),
			}, nil
		},
	}

	pipeline.Add(processor1).Add(processor2)

	doc := NewInMemoryDocument("test1", "test.txt", []byte("content"), nil)
	results, err := pipeline.Process(doc)

	if err != nil {
		t.Fatalf("Pipeline.Process() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestPipeline_IsDocumentProcessor(t *testing.T) {
	var _ DocumentProcessor = &Pipeline{}

	pipeline := NewPipeline("test-processor", nil)
	doc := NewInMemoryDocument("test1", "test.txt", []byte("content"), nil)

	_, err := pipeline.Process(doc)
	if err != nil {
		t.Fatalf("Pipeline.Process() error = %v", err)
	}
}

func TestPipeline_Run_WithStages(t *testing.T) {
	tests := []struct {
		name       string
		pipelineID string
		stages     []struct {
			name      string
			processor DocumentProcessor
			store     Store
		}
		inputDocs      []*Document
		expectedCount  int
		expectedStatus PipelineStatus
	}{
		{
			name:       "single stage without store",
			pipelineID: "test-single-no-store",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "transform",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("transformed", "transformed.txt", []byte("transformed"), nil),
							}, nil
						},
					},
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			expectedCount:  1,
			expectedStatus: StatusCompleted,
		},
		{
			name:       "multiple stages without stores",
			pipelineID: "test-multi-no-store",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "stage1",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
							}, nil
						},
					},
				},
				{
					name: "stage2",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s2", "s2.txt", []byte("stage2"), nil),
							}, nil
						},
					},
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			expectedCount:  1,
			expectedStatus: StatusCompleted,
		},
		{
			name:       "stage with store",
			pipelineID: "test-with-store",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "save",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{doc}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			expectedCount:  1,
			expectedStatus: StatusCompleted,
		},
		{
			name:       "multiple stages with stores",
			pipelineID: "test-multi-store",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "stage1",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
				{
					name: "stage2",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s2", "s2.txt", []byte("stage2"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			expectedCount:  1,
			expectedStatus: StatusCompleted,
		},
		{
			name:       "stage that produces multiple documents",
			pipelineID: "test-multi-output",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "split",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("out1", "out1.txt", []byte("output1"), nil),
								NewInMemoryDocument("out2", "out2.txt", []byte("output2"), nil),
								NewInMemoryDocument("out3", "out3.txt", []byte("output3"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			expectedCount:  3,
			expectedStatus: StatusCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stateStore := NewLocalStore(t.TempDir())
			pipeline := NewPipeline(tt.pipelineID, stateStore)

			for _, stage := range tt.stages {
				pipeline.AddStage(stage.name, stage.processor, stage.store)
			}

			results, err := pipeline.Run(ctx, tt.inputDocs)
			if err != nil {
				t.Fatalf("Pipeline.Run() error = %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			state := pipeline.State()
			if state.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, state.Status)
			}

			if state.CurrentStage != len(tt.stages) {
				t.Errorf("expected current stage %d, got %d", len(tt.stages), state.CurrentStage)
			}

			if len(state.Stages) != len(tt.stages) {
				t.Errorf("expected %d stage states, got %d", len(tt.stages), len(state.Stages))
			}

			for i, stageState := range state.Stages {
				if stageState.Status != StatusCompleted {
					t.Errorf("stage %d expected status %s, got %s", i, StatusCompleted, stageState.Status)
				}
				if stageState.CompletedAt == nil {
					t.Errorf("stage %d should have CompletedAt set", i)
				}
			}
		})
	}
}

func TestPipeline_PauseAndResume(t *testing.T) {
	tests := []struct {
		name       string
		pipelineID string
		stages     []struct {
			name      string
			processor DocumentProcessor
			store     Store
		}
		inputDocs      []*Document
		pauseAtStage   int
		expectedResume bool
		expectedCount  int
	}{
		{
			name:       "pause at first stage with store",
			pipelineID: "test-pause-first",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "stage1",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							time.Sleep(100 * time.Millisecond)
							return []*Document{
								NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
				{
					name: "stage2",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s2", "s2.txt", []byte("stage2"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			pauseAtStage:   0,
			expectedResume: true,
			expectedCount:  1,
		},
		{
			name:       "pause at middle stage with store",
			pipelineID: "test-pause-middle",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "stage1",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
				{
					name: "stage2",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							time.Sleep(100 * time.Millisecond)
							return []*Document{
								NewInMemoryDocument("s2", "s2.txt", []byte("stage2"), nil),
							}, nil
						},
					},
					store: nil,
				},
				{
					name: "stage3",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s3", "s3.txt", []byte("stage3"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			pauseAtStage:   1,
			expectedResume: true,
			expectedCount:  1,
		},
		{
			name:       "pause at stage without store resumes from previous",
			pipelineID: "test-pause-no-store",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "stage1",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
				{
					name: "stage2",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							time.Sleep(100 * time.Millisecond)
							return []*Document{
								NewInMemoryDocument("s2", "s2.txt", []byte("stage2"), nil),
							}, nil
						},
					},
					store: nil,
				},
				{
					name: "stage3",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{
								NewInMemoryDocument("s3", "s3.txt", []byte("stage3"), nil),
							}, nil
						},
					},
					store: func() Store {
						dir := t.TempDir()
						return NewLocalStore(dir)
					}(),
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			pauseAtStage:   1,
			expectedResume: true,
			expectedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir := t.TempDir()
			stateStore := NewLocalStore(stateDir)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			pipeline := NewPipeline(tt.pipelineID, stateStore)
			for _, stage := range tt.stages {
				pipeline.AddStage(stage.name, stage.processor, stage.store)
			}

			go func() {
				time.Sleep(30 * time.Millisecond)
				cancel()
			}()

			_, err := pipeline.Run(ctx, tt.inputDocs)
			if err == nil {
				t.Fatal("expected error from cancelled context")
			}

			state := pipeline.State()
			if state.Status != StatusPaused {
				t.Errorf("expected status %s, got %s", StatusPaused, state.Status)
			}

			if !tt.expectedResume {
				return
			}

			newPipeline := NewPipeline(tt.pipelineID, stateStore)
			for _, stage := range tt.stages {
				newPipeline.AddStage(stage.name, stage.processor, stage.store)
			}

			resumeCtx := context.Background()
			results, err := newPipeline.Resume(resumeCtx)
			if err != nil {
				t.Fatalf("Pipeline.Resume() error = %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results after resume, got %d", tt.expectedCount, len(results))
			}

			finalState := newPipeline.State()
			if finalState.Status != StatusCompleted {
				t.Errorf("expected status %s after resume, got %s", StatusCompleted, finalState.Status)
			}
		})
	}
}

func TestPipeline_ErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		pipelineID string
		stages     []struct {
			name      string
			processor DocumentProcessor
			store     Store
		}
		inputDocs      []*Document
		expectError    bool
		expectedStatus PipelineStatus
		errorStage     int
	}{
		{
			name:       "processor error",
			pipelineID: "test-processor-error",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "error-stage",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return nil, os.ErrNotExist
						},
					},
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			expectError:    true,
			expectedStatus: StatusFailed,
			errorStage:     0,
		},
		{
			name:       "store save error",
			pipelineID: "test-store-error",
			stages: []struct {
				name      string
				processor DocumentProcessor
				store     Store
			}{
				{
					name: "save-stage",
					processor: &mockProcessor{
						process: func(doc *Document) ([]*Document, error) {
							return []*Document{doc}, nil
						},
					},
					store: &errorStore{},
				},
			},
			inputDocs: []*Document{
				NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
			},
			expectError:    true,
			expectedStatus: StatusFailed,
			errorStage:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stateStore := NewLocalStore(t.TempDir())
			pipeline := NewPipeline(tt.pipelineID, stateStore)

			for _, stage := range tt.stages {
				pipeline.AddStage(stage.name, stage.processor, stage.store)
			}

			_, err := pipeline.Run(ctx, tt.inputDocs)

			if tt.expectError && err == nil {
				t.Fatal("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			state := pipeline.State()
			if state.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, state.Status)
			}

			if tt.expectError && tt.errorStage < len(state.Stages) {
				if state.Stages[tt.errorStage].Status != StatusFailed {
					t.Errorf("expected stage %d to have status %s, got %s", tt.errorStage, StatusFailed, state.Stages[tt.errorStage].Status)
				}
				if state.Stages[tt.errorStage].Error == "" {
					t.Errorf("expected stage %d to have error message", tt.errorStage)
				}
			}
		})
	}
}

func TestPipeline_StatePersistence(t *testing.T) {
	stateDir := t.TempDir()
	stateStore := NewLocalStore(stateDir)

	pipelineID := "test-state-persistence"
	ctx := context.Background()

	pipeline1 := NewPipeline(pipelineID, stateStore)
	pipeline1.AddStage("stage1", &mockProcessor{
		process: func(doc *Document) ([]*Document, error) {
			return []*Document{
				NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
			}, nil
		},
	}, NewLocalStore(t.TempDir()))

	inputDocs := []*Document{
		NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
	}

	_, err := pipeline1.Run(ctx, inputDocs)
	if err != nil {
		t.Fatalf("Pipeline.Run() error = %v", err)
	}

	state1 := pipeline1.State()
	if state1.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, state1.Status)
	}

	pipeline2 := NewPipeline(pipelineID, stateStore)
	pipeline2.AddStage("stage1", &mockProcessor{
		process: func(doc *Document) ([]*Document, error) {
			return []*Document{
				NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
			}, nil
		},
	}, NewLocalStore(t.TempDir()))

	err = pipeline2.LoadState(ctx)
	if err != nil {
		t.Fatalf("Pipeline.LoadState() error = %v", err)
	}

	state2 := pipeline2.State()
	if state2.ID != state1.ID {
		t.Errorf("expected ID %s, got %s", state1.ID, state2.ID)
	}
	if state2.Status != state1.Status {
		t.Errorf("expected status %s, got %s", state1.Status, state2.Status)
	}
	if state2.CurrentStage != state1.CurrentStage {
		t.Errorf("expected current stage %d, got %d", state1.CurrentStage, state2.CurrentStage)
	}
	if len(state2.Stages) != len(state1.Stages) {
		t.Errorf("expected %d stages, got %d", len(state1.Stages), len(state2.Stages))
	}
}

func TestPipeline_Resume_WithoutPause(t *testing.T) {
	stateDir := t.TempDir()
	stateStore := NewLocalStore(stateDir)

	pipelineID := "test-resume-no-pause"
	ctx := context.Background()

	pipeline := NewPipeline(pipelineID, stateStore)
	pipeline.AddStage("stage1", &mockProcessor{
		process: func(doc *Document) ([]*Document, error) {
			return []*Document{
				NewInMemoryDocument("s1", "s1.txt", []byte("stage1"), nil),
			}, nil
		},
	}, NewLocalStore(t.TempDir()))

	inputDocs := []*Document{
		NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
	}

	_, err := pipeline.Run(ctx, inputDocs)
	if err != nil {
		t.Fatalf("Pipeline.Run() error = %v", err)
	}

	_, err = pipeline.Resume(ctx)
	if err == nil {
		t.Fatal("expected error when resuming non-paused pipeline")
	}
}

type errorStore struct{}

func (e *errorStore) Save(ctx context.Context, doc *Document) (*Document, error) {
	return nil, os.ErrPermission
}

func (e *errorStore) Load(ctx context.Context, id string) (*Document, error) {
	return nil, os.ErrNotExist
}

func (e *errorStore) List(ctx context.Context) ([]*Document, error) {
	return nil, os.ErrNotExist
}

func (e *errorStore) Delete(ctx context.Context, id string) error {
	return os.ErrPermission
}
