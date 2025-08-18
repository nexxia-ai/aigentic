package core

import (
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

// NewFileAttachmentsAgent creates an agent that can analyze file attachments
func NewFileAttachmentsAgent(model *ai.Model) aigentic.Agent {
	// Create a sample text document for testing
	doc := aigentic.NewInMemoryDocument("", "sample.txt", []byte("This is a test text file with some sample content for analysis. The content includes information about artificial intelligence and machine learning."), nil)

	return aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that analyzes text files and provides insights.",
		Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
		Documents:    []*aigentic.Document{doc},
	}
}

// RunFileAttachmentsAgent executes the file attachments example and returns benchmark results
func RunFileAttachmentsAgent(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	agent := NewFileAttachmentsAgent(model)
	agent.Trace = aigentic.NewTrace()

	response, err := agent.Execute("Please analyze the attached file and tell me what it contains. If you are able to analyse the file, start your response with 'SUCCESS:' followed by the analysis.")

	result := CreateBenchResult("FileAttachments", model, start, response, err)

	if err != nil {
		return result, err
	}

	// Validate that the analysis was successful
	if err := ValidateResponse(response, "SUCCESS:"); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Also check that it mentions some content from the file
	contentChecks := []string{"artificial intelligence", "machine learning", "sample content"}
	contentFound := false
	for _, check := range contentChecks {
		if err := ValidateResponse(response, check); err == nil {
			contentFound = true
			break
		}
	}

	if !contentFound {
		result.Success = false
		result.ErrorMessage = "Response does not contain expected file content analysis"
		return result, nil
	}

	result.Metadata["expected_prefix"] = "SUCCESS:"
	result.Metadata["content_checks"] = contentChecks
	result.Metadata["response_preview"] = TruncateString(response, 150)

	return result, nil
}
