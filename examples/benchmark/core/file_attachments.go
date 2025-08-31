package core

import (
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

func RunFileAttachmentsAgent(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	doc := document.NewInMemoryDocument("", "sample.txt", []byte("This is a test text file with some sample content for analysis. The content includes information about artificial intelligence and machine learning."), nil)

	agent := aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful assistant that analyzes text files and provides insights.",
		Instructions: "When you see a file reference, analyze it and provide a summary. If you cannot access the file, explain why.",
		Documents:    []*document.Document{doc},
		Trace:        aigentic.NewTrace(),
	}

	response, err := agent.Execute("Please analyze the attached file and tell me what it contains. If you are able to analyse the file, start your response with 'SUCCESS:' followed by the analysis.")

	result := CreateBenchResult("FileAttachments", model, start, response, err)

	if err != nil {
		return result, err
	}

	if err := ValidateResponse(response, "SUCCESS:"); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

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
