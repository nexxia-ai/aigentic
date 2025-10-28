package document

// Pipeline is a DocumentProcessor that chains multiple processors together.
// Create a pipeline with NewPipeline(), add processors with Add(), then call Process().
// Pipelines are reusable and can be passed as a DocumentProcessor.
type Pipeline struct {
	processors []DocumentProcessor
}

// NewPipeline creates a new empty pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{
		processors: make([]DocumentProcessor, 0),
	}
}

// Add adds a processor to the pipeline and returns the pipeline for chaining.
func (p *Pipeline) Add(processor DocumentProcessor) *Pipeline {
	if processor != nil {
		p.processors = append(p.processors, processor)
	}
	return p
}

// Process implements DocumentProcessor interface.
// It applies each processor in sequence to the input document(s).
// Each processor may return multiple documents, which are then processed by the next processor.
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

