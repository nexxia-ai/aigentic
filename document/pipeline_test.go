package document

import (
	"testing"
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
	pipeline := NewPipeline()

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
	pipeline := NewPipeline()

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
	pipeline := NewPipeline()

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

	pipeline := NewPipeline()
	doc := NewInMemoryDocument("test1", "test.txt", []byte("content"), nil)

	_, err := pipeline.Process(doc)
	if err != nil {
		t.Fatalf("Pipeline.Process() error = %v", err)
	}
}

