package run

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
)

// BatchPolicy configures batch execution behaviour.
type BatchPolicy struct {
	MaxItems        int
	MaxConcurrency  int
	TimeoutPerItem  int // seconds
	TimeoutTotal    int // seconds
	ContinueOnError bool
	MaxFailedCount  int
}

// PlanDef is a named, ordered list of sub-agent steps with dependencies.
type PlanDef struct {
	Name        string
	Description string
	Steps       []PlanStep
}

// PlanStep is one step inside a plan.
type PlanStep struct {
	ID           string
	SubAgent     string
	Instructions string // inline instructions for dynamic plans
	DependsOn    []string
}

// ExecutionToolsConfig holds everything needed to register execution tools on an AgentRun.
type ExecutionToolsConfig struct {
	BatchPolicy     *BatchPolicy
	Plans           []PlanDef
	DynamicPlanning bool
}

// AddExecutionTools registers batch, plan, and dynamic-planning tools on the given AgentRun.
func AddExecutionTools(r *AgentRun, cfg ExecutionToolsConfig) {
	if cfg.BatchPolicy != nil {
		r.tools = append(r.tools, newAgentBatchTool(r, cfg.BatchPolicy))
	}
	for _, plan := range cfg.Plans {
		r.tools = append(r.tools, newPlanTool(r, plan))
	}
	if cfg.DynamicPlanning {
		injectDynamicPlanningTools(r)
	}
}

// --- agent_batch tool ---

type batchItem struct {
	ItemID string `json:"item_id"`
}

type batchInput struct {
	SubAgent    string      `json:"sub_agent"`
	Description string      `json:"description"`
	Items       []batchItem `json:"items"`
}

type batchItemResult struct {
	ItemID string `json:"item_id"`
	Status string `json:"status"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type batchResult struct {
	SubAgent  string            `json:"sub_agent"`
	Status    string            `json:"status"`
	Total     int               `json:"total"`
	Completed int               `json:"completed"`
	Failed    int               `json:"failed"`
	Items     []batchItemResult `json:"items"`
}

func newAgentBatchTool(parent *AgentRun, policy *BatchPolicy) AgentTool {
	return AgentTool{
		Name:        "agent_batch",
		Description: "Execute a sub-agent across multiple items concurrently. Each item is processed independently by the named sub-agent.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sub_agent": map[string]interface{}{
					"type":        "string",
					"description": "Name of the declared sub-agent to dispatch",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Human-readable label for this batch execution",
				},
				"items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"item_id": map[string]interface{}{
								"type":        "string",
								"description": "Identifier for the item. Supports file:// for files/folders relative to the working directory (e.g. file://uploads) which expand to one item per file recursively, and https:// for web pages.",
							},
						},
						"required": []string{"item_id"},
					},
					"description": "List of items to process",
				},
			},
			"required": []string{"sub_agent", "description", "items"},
		},
		Execute: func(run *AgentRun, args map[string]interface{}) (*ToolCallResult, error) {
			data, _ := json.Marshal(args)
			var input batchInput
			if err := json.Unmarshal(data, &input); err != nil {
				return nil, fmt.Errorf("invalid batch input: %w", err)
			}
			return executeBatch(run, input, policy)
		},
	}
}

func executeBatch(parentRun *AgentRun, input batchInput, policy *BatchPolicy) (*ToolCallResult, error) {
	ws := parentRun.AgentContext().Workspace()
	if ws != nil {
		expanded, err := expandItems(ws.LLMDir, input.Items)
		if err != nil {
			return &ToolCallResult{
				Result: &ai.ToolResult{
					Content: []ai.ToolContent{{Type: "text", Content: err.Error()}},
					Error:   true,
				},
			}, nil
		}
		input.Items = expanded
	}

	def, ok := parentRun.subAgentDefs[input.SubAgent]
	if !ok {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("unknown sub-agent: %s", input.SubAgent)}},
				Error:   true,
			},
		}, nil
	}

	if policy.MaxItems > 0 && len(input.Items) > policy.MaxItems {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("too many items: %d (max %d)", len(input.Items), policy.MaxItems)}},
				Error:   true,
			},
		}, nil
	}

	batchID := uuid.New().String()[:8]
	toolCallID := parentRun.CurrentToolCallID()
	total := len(input.Items)

	concurrency := policy.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	var batchCtx context.Context
	var batchCancel context.CancelFunc
	if policy.TimeoutTotal > 0 {
		batchCtx, batchCancel = context.WithTimeout(parentRun.ctx, time.Duration(policy.TimeoutTotal)*time.Second)
	} else {
		batchCtx, batchCancel = context.WithCancel(parentRun.ctx)
	}
	defer batchCancel()

	results := make([]batchItemResult, total)
	var completedCount atomic.Int32
	var failedCount atomic.Int32

	// Persist intermediate results
	batchDir := ""
	if ws != nil {
		batchDir = filepath.Join(ws.PrivateDir, "batch", batchID)
		os.MkdirAll(batchDir, 0755)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for idx, item := range input.Items {
		wg.Add(1)
		go func(i int, it batchItem) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-batchCtx.Done():
				results[i] = batchItemResult{ItemID: it.ItemID, Status: "cancelled", Error: "batch cancelled"}
				failedCount.Add(1)
				return
			}

			if policy.MaxFailedCount > 0 && int(failedCount.Load()) >= policy.MaxFailedCount {
				results[i] = batchItemResult{ItemID: it.ItemID, Status: "skipped", Error: "max failures reached"}
				failedCount.Add(1)
				return
			}

			parentRun.EmitToolActivity(toolCallID, fmt.Sprintf("Processing item %d/%d: %s", i+1, total, it.ItemID))

			output, err := runChildAgent(batchCtx, parentRun, def, it.ItemID, "Process "+it.ItemID, batchID, policy.TimeoutPerItem)
			if err != nil {
				results[i] = batchItemResult{ItemID: it.ItemID, Status: "failed", Error: err.Error()}
				failedCount.Add(1)
				if !policy.ContinueOnError {
					batchCancel()
				}
			} else {
				results[i] = batchItemResult{ItemID: it.ItemID, Status: "completed", Output: output}
			}
			completedCount.Add(1)

			c := int(completedCount.Load())
			parentRun.EmitToolActivity(toolCallID, fmt.Sprintf("Completed %d/%d", c, total))

			// Incremental persistence
			if batchDir != "" {
				persistBatchResult(batchDir, batchResult{
					SubAgent:  input.SubAgent,
					Status:    "running",
					Total:     total,
					Completed: c,
					Failed:    int(failedCount.Load()),
					Items:     results,
				})
			}
		}(idx, item)
	}

	wg.Wait()

	status := "completed"
	f := int(failedCount.Load())
	if f > 0 && f == total {
		status = "failed"
	} else if f > 0 {
		status = "partial"
	}

	res := batchResult{
		SubAgent:  input.SubAgent,
		Status:    status,
		Total:     total,
		Completed: int(completedCount.Load()),
		Failed:    f,
		Items:     results,
	}

	if batchDir != "" {
		persistBatchResult(batchDir, res)
	}

	data, _ := json.Marshal(res)
	return &ToolCallResult{
		Result: &ai.ToolResult{
			Content: []ai.ToolContent{{Type: "text", Content: string(data)}},
		},
	}, nil
}

func runChildAgent(ctx context.Context, parentRun *AgentRun, def subAgentDef, itemID, message, batchID string, timeoutPerItem int) (string, error) {
	ws := parentRun.AgentContext().Workspace()
	privateDir := filepath.Join(ws.PrivateDir, "batch", batchID, "items", itemID)

	childCtx, err := ctxt.NewChild(def.name+"-"+itemID, def.description, def.instructions, privateDir, ws.LLMDir)
	if err != nil {
		return "", fmt.Errorf("failed to create child context: %w", err)
	}

	childRun, err := Continue(childCtx, def.model, def.tools)
	if err != nil {
		return "", fmt.Errorf("failed to create child run: %w", err)
	}
	childRun.SetAgentName(def.name)
	childRun.trace = parentRun.trace
	childRun.SetEnableTrace(parentRun.enableTrace)
	childRun.Logger = parentRun.Logger.With("batch-item", itemID)
	if parentRun.streaming {
		childRun.SetStreaming(true)
	}

	var itemCtx context.Context
	var itemCancel context.CancelFunc
	if timeoutPerItem > 0 {
		itemCtx, itemCancel = context.WithTimeout(ctx, time.Duration(timeoutPerItem)*time.Second)
	} else {
		itemCtx, itemCancel = context.WithCancel(ctx)
	}
	defer itemCancel()

	childRun.Run(itemCtx, message)
	content, err := childRun.Wait(0)
	return content, err
}

func persistBatchResult(batchDir string, result batchResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		slog.Warn("failed to marshal batch result", "error", err)
		return
	}
	path := filepath.Join(batchDir, "result.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Warn("failed to write batch result", "path", path, "error", err)
	}
}

func expandItems(llmDir string, items []batchItem) ([]batchItem, error) {
	var out []batchItem
	for _, it := range items {
		id := it.ItemID
		if strings.HasPrefix(id, "file://") {
			expanded, err := expandFileURL(llmDir, id)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
			continue
		}
		if strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://") {
			out = append(out, it)
			continue
		}
		out = append(out, it)
	}
	return out, nil
}

func expandFileURL(llmDir, rawURL string) ([]batchItem, error) {
	path := strings.TrimPrefix(rawURL, "file://")
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return nil, fmt.Errorf("invalid file URL path: %s", rawURL)
	}
	if filepath.IsAbs(path) || strings.HasPrefix(path, string(filepath.Separator)) {
		return nil, fmt.Errorf("file URL path must be relative to the agent VM directory, not absolute: %s", rawURL)
	}
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("file URL path must be relative to the agent VM directory: %s", rawURL)
	}
	fullPath := filepath.Join(llmDir, path)
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("file URL path resolve: %w", err)
	}
	absLLM, err := filepath.Abs(llmDir)
	if err != nil {
		return nil, fmt.Errorf("llm dir resolve: %w", err)
	}
	rel, err := filepath.Rel(absLLM, absFull)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("file URL path must be relative to the agent VM directory: %s", rawURL)
	}
	info, err := os.Stat(absFull)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file or folder not found: %s", path)
		}
		return nil, fmt.Errorf("file URL stat: %w", err)
	}
	if info.IsDir() {
		var items []batchItem
		err := filepath.WalkDir(absFull, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			itemRel, err := filepath.Rel(absLLM, p)
			if err != nil || strings.HasPrefix(itemRel, "..") {
				return nil
			}
			items = append(items, batchItem{ItemID: filepath.ToSlash(itemRel)})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("file URL walk: %w", err)
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("folder is empty: %s", path)
		}
		return items, nil
	}
	return []batchItem{{ItemID: filepath.ToSlash(rel)}}, nil
}

// --- DAG Executor ---

type dagStep struct {
	ID           string
	SubAgent     string
	Instructions string
	Message      string
	DependsOn    []string
}

type stepResult struct {
	StepID string `json:"step_id"`
	Status string `json:"status"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// validateDAG checks for cycles and unknown step references.
func validateDAG(steps []dagStep) error {
	ids := make(map[string]bool, len(steps))
	for _, s := range steps {
		if ids[s.ID] {
			return fmt.Errorf("duplicate step ID: %s", s.ID)
		}
		ids[s.ID] = true
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			if !ids[dep] {
				return fmt.Errorf("step %q depends on unknown step %q", s.ID, dep)
			}
			if dep == s.ID {
				return fmt.Errorf("step %q depends on itself", s.ID)
			}
		}
	}

	// Topological sort to detect cycles
	inDegree := make(map[string]int, len(steps))
	adj := make(map[string][]string, len(steps))
	for _, s := range steps {
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			adj[dep] = append(adj[dep], s.ID)
			inDegree[s.ID]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adj[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != len(steps) {
		return fmt.Errorf("plan contains a cycle")
	}
	return nil
}

// orderDAG returns steps grouped by execution level (steps in the same level can run in parallel).
func orderDAG(steps []dagStep) [][]dagStep {
	stepMap := make(map[string]dagStep, len(steps))
	inDegree := make(map[string]int, len(steps))
	adj := make(map[string][]string, len(steps))

	for _, s := range steps {
		stepMap[s.ID] = s
		if _, ok := inDegree[s.ID]; !ok {
			inDegree[s.ID] = 0
		}
		for _, dep := range s.DependsOn {
			adj[dep] = append(adj[dep], s.ID)
			inDegree[s.ID]++
		}
	}

	var levels [][]dagStep
	for {
		var level []dagStep
		for id, deg := range inDegree {
			if deg == 0 {
				level = append(level, stepMap[id])
			}
		}
		if len(level) == 0 {
			break
		}
		levels = append(levels, level)
		for _, s := range level {
			delete(inDegree, s.ID)
			for _, next := range adj[s.ID] {
				inDegree[next]--
			}
		}
	}
	return levels
}

type executeStepFn func(ctx context.Context, step dagStep, upstream map[string]stepResult) (stepResult, error)

func executeDAG(ctx context.Context, steps []dagStep, concurrency int, fn executeStepFn, progressFn func(completed, total int, label string)) ([]stepResult, error) {
	if err := validateDAG(steps); err != nil {
		return nil, err
	}
	levels := orderDAG(steps)
	total := len(steps)

	results := make(map[string]stepResult)
	var mu sync.Mutex
	completed := 0

	for _, level := range levels {
		if concurrency <= 0 {
			concurrency = len(level)
		}
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		// Build step label
		names := make([]string, len(level))
		for i, s := range level {
			names[i] = s.ID
		}
		if progressFn != nil {
			progressFn(completed, total, fmt.Sprintf("Running %s", strings.Join(names, ", ")))
		}

		for _, step := range level {
			wg.Add(1)
			go func(s dagStep) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					mu.Lock()
					results[s.ID] = stepResult{StepID: s.ID, Status: "cancelled", Error: "context cancelled"}
					mu.Unlock()
					return
				}

				// Collect upstream outputs
				mu.Lock()
				upstream := make(map[string]stepResult)
				for _, dep := range s.DependsOn {
					upstream[dep] = results[dep]
				}
				mu.Unlock()

				res, err := fn(ctx, s, upstream)
				if err != nil {
					res = stepResult{StepID: s.ID, Status: "failed", Error: err.Error()}
				}
				res.StepID = s.ID

				mu.Lock()
				results[s.ID] = res
				completed++
				c := completed
				mu.Unlock()

				if progressFn != nil {
					progressFn(c, total, fmt.Sprintf("Completed %d/%d steps", c, total))
				}
			}(step)
		}
		wg.Wait()
	}

	// Collect ordered results
	ordered := make([]stepResult, 0, len(steps))
	for _, s := range steps {
		ordered = append(ordered, results[s.ID])
	}
	return ordered, nil
}

// --- Plan Tools ---

func newPlanTool(parentRun *AgentRun, plan PlanDef) AgentTool {
	toolName := strings.ReplaceAll(plan.Name, "-", "_")

	// Build input schema: per-step inputs for root steps (no dependencies)
	inputProps := map[string]interface{}{
		"description": map[string]interface{}{
			"type":        "string",
			"description": "Human-readable label for this plan execution",
		},
		"inputs": map[string]interface{}{
			"type":        "object",
			"description": "Per-step text inputs. Only root steps (those with no dependencies) need input; dependent steps receive upstream output automatically.",
		},
	}

	return AgentTool{
		Name:        toolName,
		Description: plan.Description,
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": inputProps,
			"required":   []string{"description", "inputs"},
		},
		Execute: func(run *AgentRun, args map[string]interface{}) (*ToolCallResult, error) {
			return executePlan(run, plan, args)
		},
	}
}

func executePlan(parentRun *AgentRun, plan PlanDef, args map[string]interface{}) (*ToolCallResult, error) {
	planID := uuid.New().String()[:8]
	toolCallID := parentRun.CurrentToolCallID()

	inputs := make(map[string]string)
	if inp, ok := args["inputs"].(map[string]interface{}); ok {
		for k, v := range inp {
			if s, ok := v.(string); ok {
				inputs[k] = s
			}
		}
	}

	// Convert plan steps to DAG steps with messages
	dagSteps := make([]dagStep, len(plan.Steps))
	for i, ps := range plan.Steps {
		dagSteps[i] = dagStep{
			ID:        ps.ID,
			SubAgent:  ps.SubAgent,
			DependsOn: ps.DependsOn,
			Message:   inputs[ps.ID], // may be empty for dependent steps
		}
	}

	ws := parentRun.AgentContext().Workspace()
	planDir := ""
	if ws != nil {
		planDir = filepath.Join(ws.PrivateDir, "plan", planID)
		os.MkdirAll(planDir, 0755)
	}

	fn := func(ctx context.Context, step dagStep, upstream map[string]stepResult) (stepResult, error) {
		def, ok := parentRun.subAgentDefs[step.SubAgent]
		if !ok {
			return stepResult{Status: "failed", Error: fmt.Sprintf("unknown sub-agent: %s", step.SubAgent)}, nil
		}

		// Wire upstream output into the message
		message := step.Message
		if len(upstream) > 0 {
			var upstreamText strings.Builder
			upstreamText.WriteString("Previous step results:\n\n")
			for depID, depRes := range upstream {
				upstreamText.WriteString(fmt.Sprintf("[%s]: %s\n", depID, depRes.Output))
			}
			if message != "" {
				upstreamText.WriteString("\n---\n")
				upstreamText.WriteString(message)
			}
			message = upstreamText.String()
		}

		privateDir := filepath.Join(ws.PrivateDir, "plan", planID, "tasks", step.ID)
		childCtx, err := ctxt.NewChild(def.name+"-"+step.ID, def.description, def.instructions, privateDir, ws.LLMDir)
		if err != nil {
			return stepResult{Status: "failed", Error: err.Error()}, nil
		}

		childRun, err := Continue(childCtx, def.model, def.tools)
		if err != nil {
			return stepResult{Status: "failed", Error: err.Error()}, nil
		}
		childRun.SetAgentName(def.name)
		childRun.trace = parentRun.trace
		childRun.SetEnableTrace(parentRun.enableTrace)
		childRun.Logger = parentRun.Logger.With("plan-step", step.ID)
		if parentRun.streaming {
			childRun.SetStreaming(true)
		}

		childRun.Run(ctx, message)
		content, err := childRun.Wait(0)
		if err != nil {
			return stepResult{Status: "failed", Error: err.Error()}, nil
		}
		return stepResult{Status: "completed", Output: content}, nil
	}

	progressFn := func(completed, total int, label string) {
		parentRun.EmitToolActivity(toolCallID, label)
	}

	stepResults, err := executeDAG(parentRun.ctx, dagSteps, 5, fn, progressFn)
	if err != nil {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("plan execution failed: %v", err)}},
				Error:   true,
			},
		}, nil
	}

	// Persist plan result
	if planDir != "" {
		planResult := map[string]interface{}{
			"plan":  plan.Name,
			"steps": stepResults,
		}
		data, _ := json.MarshalIndent(planResult, "", "  ")
		os.WriteFile(filepath.Join(planDir, "plan.json"), data, 0644)
	}

	status := "completed"
	for _, sr := range stepResults {
		if sr.Status == "failed" || sr.Status == "cancelled" {
			status = "partial"
			break
		}
	}

	result := map[string]interface{}{
		"plan":   plan.Name,
		"status": status,
		"steps":  stepResults,
	}
	data, _ := json.Marshal(result)
	return &ToolCallResult{
		Result: &ai.ToolResult{
			Content: []ai.ToolContent{{Type: "text", Content: string(data)}},
		},
	}, nil
}

// --- Dynamic Planning Tools ---

func injectDynamicPlanningTools(r *AgentRun) {
	// Inject the planner sub-agent
	plannerInstructions := buildPlannerInstructions(r)
	r.AddSubAgent("planner", "Creates structured execution plans by decomposing goals into tasks with dependencies", plannerInstructions, r.model, nil)

	// Inject create_plan tool
	r.tools = append(r.tools, newCreatePlanTool(r))

	// Inject execute_plan tool
	r.tools = append(r.tools, newExecutePlanTool(r))
}

func buildPlannerInstructions(r *AgentRun) string {
	var b strings.Builder
	b.WriteString(`You are a planning agent. Your job is to decompose a goal into a structured plan of tasks.

Each task has:
- id: a short unique identifier (lowercase, hyphens allowed)
- instructions: clear instructions for what the task should accomplish
- depends_on: list of task IDs that must complete before this task starts (empty list for root tasks)

Rules:
- Maximum 20 tasks per plan
- Tasks run in parallel when they have no dependencies on each other
- Tasks with dependencies receive the output of their upstream tasks automatically
- Keep tasks focused and atomic -- each task should do one thing well

`)

	// List available sub-agents if any
	if len(r.subAgentDefs) > 0 {
		b.WriteString("Available sub-agents that tasks can reference:\n")
		for name, def := range r.subAgentDefs {
			if name == "planner" {
				continue
			}
			b.WriteString(fmt.Sprintf("- %s: %s\n", name, def.description))
		}
		b.WriteString("\nWhen a task should use a specific sub-agent, include the sub-agent name in the task instructions.\n")
	}

	b.WriteString(`
Return your plan as valid JSON with this structure:
{
  "goal": "brief description of the overall goal",
  "tasks": [
    {"id": "task-id", "instructions": "What to do", "sub_agent": "optional-agent-name", "depends_on": []}
  ]
}

Return ONLY the JSON. No additional text.`)
	return b.String()
}

type createPlanInput struct {
	Goal  string             `json:"goal"`
	Tasks []dynamicPlanTask  `json:"tasks"`
}

type dynamicPlanTask struct {
	ID           string   `json:"id"`
	Instructions string   `json:"instructions"`
	SubAgent     string   `json:"sub_agent,omitempty"`
	DependsOn    []string `json:"depends_on"`
}

func newCreatePlanTool(parentRun *AgentRun) AgentTool {
	return AgentTool{
		Name:        "create_plan",
		Description: "Freeze a dynamic plan. Validates structure and persists to workspace. Returns a plan_id for use with execute_plan.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"goal": map[string]interface{}{
					"type":        "string",
					"description": "Brief description of the plan's goal",
				},
				"tasks": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":        "string",
								"description": "Unique task identifier",
							},
							"instructions": map[string]interface{}{
								"type":        "string",
								"description": "Instructions for the task",
							},
							"sub_agent": map[string]interface{}{
								"type":        "string",
								"description": "Optional sub-agent name to execute this task",
							},
							"depends_on": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "Task IDs this task depends on",
							},
						},
						"required": []string{"id", "instructions", "depends_on"},
					},
				},
			},
			"required": []string{"goal", "tasks"},
		},
		Execute: func(run *AgentRun, args map[string]interface{}) (*ToolCallResult, error) {
			data, _ := json.Marshal(args)
			var input createPlanInput
			if err := json.Unmarshal(data, &input); err != nil {
				return nil, fmt.Errorf("invalid create_plan input: %w", err)
			}
			return executeCreatePlan(run, input)
		},
	}
}

func executeCreatePlan(r *AgentRun, input createPlanInput) (*ToolCallResult, error) {
	if len(input.Tasks) == 0 {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "plan must have at least one task"}},
				Error:   true,
			},
		}, nil
	}
	if len(input.Tasks) > 20 {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "plan must have at most 20 tasks"}},
				Error:   true,
			},
		}, nil
	}

	// Validate as a DAG
	dagSteps := make([]dagStep, len(input.Tasks))
	for i, t := range input.Tasks {
		dagSteps[i] = dagStep{
			ID:           t.ID,
			SubAgent:     t.SubAgent,
			Instructions: t.Instructions,
			DependsOn:    t.DependsOn,
		}
	}
	if err := validateDAG(dagSteps); err != nil {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("invalid plan: %v", err)}},
				Error:   true,
			},
		}, nil
	}

	planID := "plan_" + uuid.New().String()[:8]

	// Persist frozen plan
	ws := r.AgentContext().Workspace()
	if ws != nil {
		planDir := filepath.Join(ws.PrivateDir, "plan", planID)
		os.MkdirAll(planDir, 0755)

		planData := map[string]interface{}{
			"plan_id": planID,
			"goal":    input.Goal,
			"tasks":   input.Tasks,
			"status":  "frozen",
		}
		data, _ := json.MarshalIndent(planData, "", "  ")
		os.WriteFile(filepath.Join(planDir, "plan.json"), data, 0644)
	}

	result := map[string]interface{}{
		"plan_id": planID,
		"status":  "frozen",
	}
	data, _ := json.Marshal(result)
	return &ToolCallResult{
		Result: &ai.ToolResult{
			Content: []ai.ToolContent{{Type: "text", Content: string(data)}},
		},
	}, nil
}

func newExecutePlanTool(parentRun *AgentRun) AgentTool {
	return AgentTool{
		Name:        "execute_plan",
		Description: "Execute a frozen dynamic plan by plan_id. The plan must have been created with create_plan.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Human-readable label for this plan execution",
				},
				"plan_id": map[string]interface{}{
					"type":        "string",
					"description": "The plan_id returned by create_plan",
				},
			},
			"required": []string{"description", "plan_id"},
		},
		Execute: func(run *AgentRun, args map[string]interface{}) (*ToolCallResult, error) {
			planID, _ := args["plan_id"].(string)
			if planID == "" {
				return &ToolCallResult{
					Result: &ai.ToolResult{
						Content: []ai.ToolContent{{Type: "text", Content: "plan_id is required"}},
						Error:   true,
					},
				}, nil
			}
			return executeStoredPlan(run, planID)
		},
	}
}

func executeStoredPlan(parentRun *AgentRun, planID string) (*ToolCallResult, error) {
	ws := parentRun.AgentContext().Workspace()
	if ws == nil {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: "workspace not available"}},
				Error:   true,
			},
		}, nil
	}

	planDir := filepath.Join(ws.PrivateDir, "plan", planID)
	planFile := filepath.Join(planDir, "plan.json")
	data, err := os.ReadFile(planFile)
	if err != nil {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("plan not found: %s", planID)}},
				Error:   true,
			},
		}, nil
	}

	var stored struct {
		PlanID string            `json:"plan_id"`
		Goal   string            `json:"goal"`
		Tasks  []dynamicPlanTask `json:"tasks"`
		Status string            `json:"status"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("invalid plan data: %v", err)}},
				Error:   true,
			},
		}, nil
	}

	if stored.Status != "frozen" {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("plan is not frozen (status: %s)", stored.Status)}},
				Error:   true,
			},
		}, nil
	}

	toolCallID := parentRun.CurrentToolCallID()

	// Convert dynamic tasks to DAG steps
	dagSteps := make([]dagStep, len(stored.Tasks))
	for i, t := range stored.Tasks {
		dagSteps[i] = dagStep{
			ID:           t.ID,
			SubAgent:     t.SubAgent,
			Instructions: t.Instructions,
			DependsOn:    t.DependsOn,
		}
	}

	fn := func(ctx context.Context, step dagStep, upstream map[string]stepResult) (stepResult, error) {
		// For dynamic plans, steps may reference a sub-agent or use inline instructions
		var def subAgentDef
		if step.SubAgent != "" {
			d, ok := parentRun.subAgentDefs[step.SubAgent]
			if !ok {
				return stepResult{Status: "failed", Error: fmt.Sprintf("unknown sub-agent: %s", step.SubAgent)}, nil
			}
			def = d
		} else {
			// Dynamic task with inline instructions -- use the parent's model
			def = subAgentDef{
				name:         step.ID,
				description:  "Dynamic plan task",
				instructions: step.Instructions,
				model:        parentRun.model,
			}
		}

		// Wire upstream output
		message := step.Instructions
		if len(upstream) > 0 {
			var upstreamText strings.Builder
			upstreamText.WriteString("Previous step results:\n\n")
			for depID, depRes := range upstream {
				upstreamText.WriteString(fmt.Sprintf("[%s]: %s\n", depID, depRes.Output))
			}
			upstreamText.WriteString("\n---\n")
			upstreamText.WriteString(message)
			message = upstreamText.String()
		}

		privateDir := filepath.Join(ws.PrivateDir, "plan", planID, "tasks", step.ID)
		childCtx, err := ctxt.NewChild(def.name+"-"+step.ID, def.description, def.instructions, privateDir, ws.LLMDir)
		if err != nil {
			return stepResult{Status: "failed", Error: err.Error()}, nil
		}

		childRun, err := Continue(childCtx, def.model, def.tools)
		if err != nil {
			return stepResult{Status: "failed", Error: err.Error()}, nil
		}
		childRun.SetAgentName(def.name)
		childRun.trace = parentRun.trace
		childRun.SetEnableTrace(parentRun.enableTrace)
		childRun.Logger = parentRun.Logger.With("plan-task", step.ID)
		if parentRun.streaming {
			childRun.SetStreaming(true)
		}

		childRun.Run(ctx, message)
		content, err := childRun.Wait(0)
		if err != nil {
			return stepResult{Status: "failed", Error: err.Error()}, nil
		}
		return stepResult{Status: "completed", Output: content}, nil
	}

	progressFn := func(completed, total int, label string) {
		parentRun.EmitToolActivity(toolCallID, label)
	}

	stepResults, err := executeDAG(parentRun.ctx, dagSteps, 5, fn, progressFn)
	if err != nil {
		return &ToolCallResult{
			Result: &ai.ToolResult{
				Content: []ai.ToolContent{{Type: "text", Content: fmt.Sprintf("plan execution failed: %v", err)}},
				Error:   true,
			},
		}, nil
	}

	// Update plan.json with results
	planResult := map[string]interface{}{
		"plan_id": planID,
		"goal":    stored.Goal,
		"tasks":   stored.Tasks,
		"status":  "completed",
		"results": stepResults,
	}
	resultData, _ := json.MarshalIndent(planResult, "", "  ")
	os.WriteFile(planFile, resultData, 0644)

	status := "completed"
	for _, sr := range stepResults {
		if sr.Status == "failed" || sr.Status == "cancelled" {
			status = "partial"
			break
		}
	}

	result := map[string]interface{}{
		"plan":   planID,
		"status": status,
		"steps":  stepResults,
	}
	respData, _ := json.Marshal(result)
	return &ToolCallResult{
		Result: &ai.ToolResult{
			Content: []ai.ToolContent{{Type: "text", Content: string(respData)}},
		},
	}, nil
}

// --- Reload and Continue ---

// loadBatchState reads a previous batch result from workspace for resume.
func loadBatchState(batchDir string) (*batchResult, error) {
	path := filepath.Join(batchDir, "result.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result batchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// loadPlanState reads a previous plan state from workspace for resume.
func loadPlanState(planDir string) (*struct {
	PlanID  string            `json:"plan_id"`
	Goal    string            `json:"goal"`
	Tasks   []dynamicPlanTask `json:"tasks"`
	Status  string            `json:"status"`
	Results []stepResult      `json:"results"`
}, error) {
	path := filepath.Join(planDir, "plan.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state struct {
		PlanID  string            `json:"plan_id"`
		Goal    string            `json:"goal"`
		Tasks   []dynamicPlanTask `json:"tasks"`
		Status  string            `json:"status"`
		Results []stepResult      `json:"results"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// ResumeState holds the reload state for batch and plan executions found in workspace.
type ResumeState struct {
	Batches map[string]*batchResult // batchID -> previous result
	Plans   map[string][]stepResult // planID -> previous step results
}

// LoadResumeState scans the workspace for persisted batch/plan state.
func LoadResumeState(ws interface{ PrivateDir() string }) *ResumeState {
	return &ResumeState{
		Batches: make(map[string]*batchResult),
		Plans:   make(map[string][]stepResult),
	}
}
