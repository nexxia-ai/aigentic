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
	pipeline := NewPipeline("test-empty")

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
	pipeline := NewPipeline("test-single")

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
	pipeline := NewPipeline("test-multiple")

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

	pipeline := NewPipeline("test-processor")
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
		inputDocs     []*Document
		expectedCount int
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
			expectedCount: 1,
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
			expectedCount: 1,
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
			expectedCount: 1,
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
			expectedCount: 1,
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
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pipeline := NewPipeline(tt.pipelineID)

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
		})
	}
}

func TestPipeline_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipeline := NewPipeline("test-cancel")
	pipeline.AddStage("stage1", &mockProcessor{
		process: func(doc *Document) ([]*Document, error) {
			time.Sleep(200 * time.Millisecond)
			return []*Document{doc}, nil
		},
	}, nil)
	pipeline.AddStage("stage2", &mockProcessor{
		process: func(doc *Document) ([]*Document, error) {
			return []*Document{doc}, nil
		},
	}, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	inputDocs := []*Document{
		NewInMemoryDocument("input1", "input.txt", []byte("input"), nil),
	}

	_, err := pipeline.Run(ctx, inputDocs)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if err != ctx.Err() {
		t.Errorf("expected context error, got %v", err)
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
		inputDocs   []*Document
		expectError bool
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
			expectError: true,
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
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pipeline := NewPipeline(tt.pipelineID)

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
		})
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



