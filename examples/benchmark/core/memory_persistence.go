package core

import (
	"context"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

// NewMemoryPersistenceAgent creates a coordinator agent that uses memory
func NewMemoryPersistenceAgent(model *ai.Model) aigentic.Agent {
	// Sub-agents
	lookupCompany := aigentic.Agent{
		Model:        model,
		Name:         "lookup_company",
		Description:  "This agent allows you to look up a company name by company number. Please provide the request as 'lookup the company name for xxx'",
		Instructions: "Use tools to look up the company name. Return exactly 'COMPANY: <name>' and nothing else.",
		AgentTools:   []aigentic.AgentTool{NewCompanyNameTool()},
	}

	lookupSupplier := aigentic.Agent{
		Model:        model,
		Name:         "lookup_company_supplier",
		Description:  "This agent allows you to look up a supplier name by supplier number. The request should be in the format 'lookup the supplier name for xxx'",
		Instructions: "Use tools to look up the supplier name. Return exactly 'SUPPLIER: <name>' and nothing else.",
		AgentTools:   []aigentic.AgentTool{NewSecretSupplierTool()},
	}

	// Coordinator executes the plan, saves each result to memory, then replies with full memory content
	coordinator := aigentic.Agent{
		Model:       model,
		Name:        "coordinator",
		Description: "You are a coordinator that executes a plan and saves the results to memory. ",
		Instructions: "1) First analyse the plan and identify tasks" +
			"2) Execute the plan by executing each task in the order specified. " +
			"3) Keep track of the tasks you have already executed to avoid repeating the same task. Save the tasks you have executed to memory." +
			"4) When saving memory, include the current memory content and append the new result so both are present. " +
			"5) Return only the memory content (no commentary). " +
			"Do not make up information. You must use the tools to get the information.",
		Agents: []aigentic.Agent{lookupCompany, lookupSupplier},
		Trace:  aigentic.NewTrace(),
		Memory: aigentic.NewMemory(), // this is important to save the plan
	}

	return coordinator
}

// Run executes the memory persistence example and returns benchmark results
func RunMemoryPersistenceAgent(model *ai.Model) (BenchResult, error) {
	start := time.Now()

	session := aigentic.NewSession(context.Background())

	coordinator := NewMemoryPersistenceAgent(model)
	coordinator.Session = session

	run, err := coordinator.Start(
		"Execute the following plan: " +
			"1) Call 'lookup_company' with input 'Look up company 150'. " +
			"2) Save the result to memory using save_memory. " +
			"3) Call 'lookup_company_supplier' with input 'Look up supplier 200'. " +
			"4) Save the result to memory again, including previous memory content. " +
			"5) When you have the company and the supplier details, then respond with exactly the full content of ContextMemory.md (no extra text).",
	)
	if err != nil {
		result := CreateBenchResult("MemoryPersistence", model, start, "", err)
		return result, err
	}

	var toolOrder []string
	var saveCount int
	var chunks []string

	for ev := range run.Next() {
		switch e := ev.(type) {
		case *aigentic.ContentEvent:
			chunks = append(chunks, e.Content)
		case *aigentic.ToolEvent:
			toolOrder = append(toolOrder, e.ToolName)
			if e.ToolName == "save_memory" {
				saveCount++
			}
		case *aigentic.ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *aigentic.ErrorEvent:
			result := CreateBenchResult("MemoryPersistence", model, start, "", e.Err)
			return result, e.Err
		}
	}

	finalContent := strings.Join(chunks, "")
	result := CreateBenchResult("MemoryPersistence", model, start, finalContent, nil)

	// Validate memory contains both company and supplier results
	if err := ValidateResponse(finalContent, "nexxia"); err != nil {
		result.Success = false
		result.ErrorMessage = "Memory should include company result (Nexxia)"
		return result, err
	}

	if err := ValidateResponse(finalContent, "phoenix"); err != nil {
		result.Success = false
		result.ErrorMessage = "Memory should include supplier result (Phoenix)"
		return result, err
	}

	// Ensure orchestration used subagents and memory saves
	companyIdx := -1
	supplierIdx := -1
	for i, tool := range toolOrder {
		if tool == "lookup_company" && companyIdx == -1 {
			companyIdx = i
		}
		if tool == "lookup_company_supplier" && supplierIdx == -1 {
			supplierIdx = i
		}
	}

	if companyIdx == -1 {
		result.Success = false
		result.ErrorMessage = "lookup_company subagent should be called"
		return result, nil
	}

	if supplierIdx == -1 {
		result.Success = false
		result.ErrorMessage = "lookup_company_supplier subagent should be called"
		return result, nil
	}

	if saveCount < 2 {
		result.Success = false
		result.ErrorMessage = "save_memory should be called at least twice"
		return result, nil
	}

	result.Metadata["tool_order"] = toolOrder
	result.Metadata["save_count"] = saveCount
	result.Metadata["company_index"] = companyIdx
	result.Metadata["supplier_index"] = supplierIdx
	result.Metadata["response_preview"] = TruncateString(finalContent, 150)

	return result, nil
}
