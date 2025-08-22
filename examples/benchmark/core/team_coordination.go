package core

import (
	"context"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

// NewTeamCoordinationAgent creates a coordinator agent with subagents
func NewTeamCoordinationAgent(model *ai.Model) aigentic.Agent {
	// Subagents
	lookup := aigentic.Agent{
		Model:        model,
		Name:         "agent_lookup_company_by_name",
		Description:  "Lookup company details by name. Return either 'COMPANY_ID: <id>; NAME: <name>' or 'NOT_FOUND' only.",
		Instructions: "Use tools to perform the lookup and return the canonical format only.",
		AgentTools:   []aigentic.AgentTool{NewLookupCompanyByNameTool()},
	}

	companyCreator := aigentic.Agent{
		Model:        model,
		Name:         "agent_create_company",
		Description:  "Create a new company by name and return 'COMPANY_ID: <id>; NAME: <name>' only.",
		Instructions: "Use tools to create the company and return the canonical format only.",
		AgentTools:   []aigentic.AgentTool{NewCreateCompanyTool()},
	}

	invoiceCreator := aigentic.Agent{
		Model:        model,
		Name:         "agent_create_invoice",
		Description:  "Create an invoice for a given company_id and amount. Return 'INVOICE_ID: <id>; AMOUNT: <amount>' only.",
		Instructions: "Use tools to create the invoice and return the canonical format only.",
		AgentTools:   []aigentic.AgentTool{NewCreateInvoiceTool()},
	}

	coordinator := aigentic.Agent{
		Model: model,
		Name:  "coordinator",
		Description: "Coordinate a workflow to ensure an invoice exists for the requested company name and amount. " +
			"Steps: 1) Call 'lookup' subagent with the company name. 2) If NOT_FOUND, call 'company_creator' to create it. " +
			"3) Call 'invoice_creator' with the resolved company_id and the requested amount. " +
			"Finally, return exactly: 'COMPANY_ID: <id>; NAME: <name>; INVOICE_ID: <invoice>; AMOUNT: <amount>'.",
		Instructions: "Call exactly one tool at a time and wait for the response before the next call. " +
			"Use the save_memory tool to persist important context between tool calls, especially after getting company information and getting invoice information. " +
			"Do not add commentary.",
		Agents: []aigentic.Agent{lookup, companyCreator, invoiceCreator},
		Trace:  aigentic.NewTrace(),
		Memory: aigentic.NewMemory(),
		// LogLevel: slog.LevelDebug,
	}

	return coordinator
}

// RunTeamCoordination executes the team coordination example and returns benchmark results
func RunTeamCoordination(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	session := aigentic.NewSession(context.Background())

	coordinator := NewTeamCoordinationAgent(model)
	coordinator.Session = session

	run, err := coordinator.Start("Create an invoice for company 'Nexxia' for the amount 100. Return the final canonical line only.")
	if err != nil {
		result := CreateBenchResult("TeamCoordination", model, start, "", err)
		return result, err
	}

	var chunks []string
	toolCalls := []string{}

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *aigentic.ContentEvent:
			chunks = append(chunks, e.Content)
		case *aigentic.ToolEvent:
			toolCalls = append(toolCalls, e.ToolName)
		case *aigentic.ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *aigentic.ErrorEvent:
			result := CreateBenchResult("TeamCoordination", model, start, "", e.Err)
			return result, e.Err
		}
	}

	response := strings.Join(chunks, "")
	result := CreateBenchResult("TeamCoordination", model, start, response, nil)

	// Validate final content contains expected elements
	expectedElements := []string{"COMPANY_ID:", "NAME:", "INVOICE_ID:", "AMOUNT:", "Nexxia", "100"}
	for _, element := range expectedElements {
		if !strings.Contains(response, element) {
			err := ValidateResponse(response, element)
			result.Success = false
			result.ErrorMessage = err.Error()
			return result, err
		}
	}

	// Check that lookup subagent was called
	lookupCalled := false
	for _, toolCall := range toolCalls {
		if toolCall == "lookup" {
			lookupCalled = true
			break
		}
	}

	if !lookupCalled {
		result.Success = false
		result.ErrorMessage = "Expected lookup subagent to be called"
		return result, nil
	}

	result.Metadata["tool_calls"] = toolCalls
	result.Metadata["expected_elements"] = expectedElements
	result.Metadata["response_preview"] = TruncateString(response, 100)

	return result, nil
}
