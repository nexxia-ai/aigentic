package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

// CreateBenchResult creates a standardized BenchResult with basic fields filled
func CreateBenchResult(testCase string, model *ai.Model, start time.Time, response string, err error) BenchResult {
	duration := time.Since(start)

	result := BenchResult{
		TestCase:     testCase,
		ModelName:    model.ModelName,
		Duration:     duration,
		ResponseSize: len(response),
		Metadata:     make(map[string]interface{}),
	}

	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
	} else {
		result.Success = true
	}

	return result
}

// ValidateResponse checks if response contains expected content (case-insensitive)
func ValidateResponse(response, expectedContent string) error {
	if !strings.Contains(strings.ToLower(response), strings.ToLower(expectedContent)) {
		return fmt.Errorf("expected response to contain '%s', got: %s", expectedContent, truncateString(response, 200))
	}
	return nil
}

// TruncateString truncates a string to maxLen characters with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// truncateString is an internal helper for shorter truncation
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// NewCompanyNameTool creates a test tool for company lookup
func NewCompanyNameTool() aigentic.AgentTool {
	return aigentic.AgentTool{
		Name:        "lookup_company_name",
		Description: "A tool that looks up the name of a company based on a company number",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"company_number": map[string]interface{}{
					"type":        "string",
					"description": "The company number to lookup",
				},
			},
			"required": []string{"company_number"},
		},
		Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Nexxia"}},
				Error:   false,
			}, nil
		},
	}
}

// NewSecretSupplierTool creates a test tool for supplier lookup
func NewSecretSupplierTool() aigentic.AgentTool {
	return aigentic.AgentTool{
		Name:        "lookup_supplier_name",
		Description: "A tool that looks up the name of a supplier based on a supplier number",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supplier_number": map[string]interface{}{
					"type":        "string",
					"description": "The supplier number to lookup",
				},
			},
			"required": []string{"supplier_number"},
		},
		Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			return &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "Phoenix"}},
				Error:   false,
			}, nil
		},
	}
}

// NewLookupCompanyByNameTool creates a tool for looking up company by name
func NewLookupCompanyByNameTool() aigentic.AgentTool {
	return aigentic.AgentTool{
		Name:        "lookup_company_id",
		Description: "Lookup a company ID by its name. Returns 'COMPANY_ID: <id>; NAME: <name>' if found, otherwise 'NOT_FOUND'",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The company name to lookup",
				},
			},
			"required": []string{"name"},
		},
		Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			name, _ := args["name"].(string)
			content := "NOT_FOUND"
			if strings.EqualFold(strings.TrimSpace(name), "Nexxia") {
				content = "COMPANY_ID: COMP-001; NAME: Nexxia"
			}
			return &ai.ToolResult{Content: []ai.ToolContent{{Type: "text", Content: content}}}, nil
		},
	}
}

// NewCreateCompanyTool creates a tool for creating companies
func NewCreateCompanyTool() aigentic.AgentTool {
	return aigentic.AgentTool{
		Name:        "create_company",
		Description: "Create a new company by name. Returns 'COMPANY_ID: <id>; NAME: <name>'",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The company name to create",
				},
			},
			"required": []string{"name"},
		},
		Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			name, _ := args["name"].(string)
			id := "COMP-NEW-001"
			if strings.EqualFold(strings.TrimSpace(name), "Contoso") {
				id = "COMP-CONTOSO-001"
			}
			if strings.EqualFold(strings.TrimSpace(name), "Nexxia") {
				id = "COMP-001"
			}
			content := fmt.Sprintf("COMPANY_ID: %s; NAME: %s", id, strings.TrimSpace(name))
			return &ai.ToolResult{Content: []ai.ToolContent{{Type: "text", Content: content}}}, nil
		},
	}
}

// NewCreateInvoiceTool creates a tool for creating invoices
func NewCreateInvoiceTool() aigentic.AgentTool {
	return aigentic.AgentTool{
		Name:        "create_invoice",
		Description: "Create an invoice for a company. Returns 'INVOICE_ID: <id>; AMOUNT: <amount>'",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"company_id": map[string]interface{}{
					"type":        "string",
					"description": "The company ID to invoice",
				},
				"amount": map[string]interface{}{
					"type":        "number",
					"description": "The invoice amount",
				},
			},
			"required": []string{"company_id", "amount"},
		},
		Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			amountStr := ""
			switch v := args["amount"].(type) {
			case float64:
				amountStr = fmt.Sprintf("%.0f", v)
			case string:
				amountStr = v
			default:
				amountStr = "0"
			}
			content := fmt.Sprintf("INVOICE_ID: INV-1001; AMOUNT: %s", amountStr)
			return &ai.ToolResult{Content: []ai.ToolContent{{Type: "text", Content: content}}}, nil
		},
	}
}
