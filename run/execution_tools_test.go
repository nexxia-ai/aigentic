package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- DAG Validation Tests ---

func TestValidateDAG_Valid(t *testing.T) {
	steps := []dagStep{
		{ID: "a", DependsOn: nil},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}
	err := validateDAG(steps)
	assert.NoError(t, err)
}

func TestValidateDAG_DuplicateID(t *testing.T) {
	steps := []dagStep{
		{ID: "a"},
		{ID: "a"},
	}
	err := validateDAG(steps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate step ID")
}

func TestValidateDAG_UnknownDependency(t *testing.T) {
	steps := []dagStep{
		{ID: "a", DependsOn: []string{"z"}},
	}
	err := validateDAG(steps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown step")
}

func TestValidateDAG_SelfDependency(t *testing.T) {
	steps := []dagStep{
		{ID: "a", DependsOn: []string{"a"}},
	}
	err := validateDAG(steps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "depends on itself")
}

func TestValidateDAG_Cycle(t *testing.T) {
	steps := []dagStep{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}
	err := validateDAG(steps)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestValidateDAG_SingleStep(t *testing.T) {
	steps := []dagStep{{ID: "only"}}
	err := validateDAG(steps)
	assert.NoError(t, err)
}

// --- DAG Ordering Tests ---

func TestOrderDAG_Linear(t *testing.T) {
	steps := []dagStep{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}
	levels := orderDAG(steps)
	require.Len(t, levels, 3)
	assert.Equal(t, "a", levels[0][0].ID)
	assert.Equal(t, "b", levels[1][0].ID)
	assert.Equal(t, "c", levels[2][0].ID)
}

func TestOrderDAG_Parallel(t *testing.T) {
	steps := []dagStep{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}
	levels := orderDAG(steps)
	require.Len(t, levels, 1)
	assert.Len(t, levels[0], 3)
}

func TestOrderDAG_Diamond(t *testing.T) {
	steps := []dagStep{
		{ID: "root"},
		{ID: "left", DependsOn: []string{"root"}},
		{ID: "right", DependsOn: []string{"root"}},
		{ID: "join", DependsOn: []string{"left", "right"}},
	}
	levels := orderDAG(steps)
	require.Len(t, levels, 3)
	assert.Len(t, levels[0], 1)
	assert.Equal(t, "root", levels[0][0].ID)
	assert.Len(t, levels[1], 2) // left and right in parallel
	assert.Len(t, levels[2], 1)
	assert.Equal(t, "join", levels[2][0].ID)
}

// --- DAG Execution Tests ---

func TestExecuteDAG_SimpleLinear(t *testing.T) {
	steps := []dagStep{
		{ID: "first", Message: "hello"},
		{ID: "second", DependsOn: []string{"first"}},
	}

	fn := func(ctx context.Context, step dagStep, upstream map[string]stepResult) (stepResult, error) {
		if step.ID == "first" {
			return stepResult{Status: "completed", Output: "result-from-first"}, nil
		}
		// second step should receive upstream from first
		assert.Contains(t, upstream, "first")
		assert.Equal(t, "result-from-first", upstream["first"].Output)
		return stepResult{Status: "completed", Output: "result-from-second"}, nil
	}

	var progressCalls []string
	progressFn := func(completed, total int, label string, activityID string) {
		progressCalls = append(progressCalls, label)
	}

	results, err := executeDAG(context.Background(), steps, 5, fn, progressFn)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "completed", results[0].Status)
	assert.Equal(t, "completed", results[1].Status)
	assert.Equal(t, "result-from-first", results[0].Output)
	assert.Equal(t, "result-from-second", results[1].Output)
	assert.NotEmpty(t, progressCalls)
}

func TestExecuteDAG_ParallelExecution(t *testing.T) {
	steps := []dagStep{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}

	fn := func(ctx context.Context, step dagStep, upstream map[string]stepResult) (stepResult, error) {
		return stepResult{Status: "completed", Output: "done-" + step.ID}, nil
	}

	results, err := executeDAG(context.Background(), steps, 3, fn, nil)
	require.NoError(t, err)
	assert.Len(t, results, 3)
	for _, r := range results {
		assert.Equal(t, "completed", r.Status)
	}
}

func TestExecuteDAG_InvalidCycle(t *testing.T) {
	steps := []dagStep{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}

	fn := func(ctx context.Context, step dagStep, upstream map[string]stepResult) (stepResult, error) {
		return stepResult{Status: "completed"}, nil
	}

	_, err := executeDAG(context.Background(), steps, 5, fn, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestExecuteDAG_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	steps := []dagStep{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
	}

	fn := func(ctx context.Context, step dagStep, upstream map[string]stepResult) (stepResult, error) {
		if step.ID == "a" {
			cancel() // cancel after first step
			return stepResult{Status: "completed", Output: "done"}, nil
		}
		return stepResult{Status: "completed"}, nil
	}

	results, err := executeDAG(ctx, steps, 5, fn, nil)
	require.NoError(t, err)
	// First step should complete, second may be cancelled
	assert.Equal(t, "completed", results[0].Status)
}

// --- Batch Execution Tests ---

func newTestAgentRun(t *testing.T) *AgentRun {
	t.Helper()
	tmpDir := t.TempDir()
	ar, err := NewAgentRun("test-batch-agent", "test batch agent", "test instructions", tmpDir)
	require.NoError(t, err)

	dummyModel := ai.NewDummyModel(func(ctx context.Context, messages []ai.Message, tools []ai.Tool) (ai.AIMessage, error) {
		for _, msg := range messages {
			if um, ok := msg.(ai.UserMessage); ok {
				return ai.AIMessage{
					Role:    ai.AssistantRole,
					Content: "processed: " + um.Content,
				}, nil
			}
		}
		return ai.AIMessage{Role: ai.AssistantRole, Content: "no input"}, nil
	})
	ar.SetModel(dummyModel)
	ar.SetEnableTrace(true)

	// Set a context so internal functions that use parentRun.ctx work
	ar.ctx, ar.cancelFunc = context.WithCancel(context.Background())
	t.Cleanup(func() { ar.cancelFunc() })

	return ar
}

func toolResultText(result *ToolCallResult) string {
	if result == nil || result.Result == nil || len(result.Result.Content) == 0 {
		return ""
	}
	s, _ := result.Result.Content[0].Content.(string)
	return s
}

func TestExecuteBatch_Success(t *testing.T) {
	ar := newTestAgentRun(t)

	// Register a sub-agent
	ar.AddSubAgent("worker", "processes items", "You process items", ar.model, nil)

	input := batchInput{
		SubAgent:    "worker",
		Description: "test batch",
		Items: []batchItem{
			{ItemID: "item-1"},
			{ItemID: "item-2"},
		},
	}
	policy := &BatchPolicy{MaxConcurrency: 2, ContinueOnError: false}

	result, err := executeBatch(ar, input, policy)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Result.Error)

	var res batchResult
	err = json.Unmarshal([]byte(toolResultText(result)), &res)
	require.NoError(t, err)
	assert.Equal(t, "completed", res.Status)
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, 2, res.Completed)
	assert.Equal(t, 0, res.Failed)
	assert.Len(t, res.Items, 2)
}

func TestExecuteBatch_UnknownSubAgent(t *testing.T) {
	ar := newTestAgentRun(t)

	input := batchInput{
		SubAgent:    "nonexistent",
		Description: "test batch",
		Items:       []batchItem{{ItemID: "item-1"}},
	}
	policy := &BatchPolicy{MaxConcurrency: 2}

	result, err := executeBatch(ar, input, policy)
	require.NoError(t, err)
	assert.True(t, result.Result.Error)
	assert.Contains(t, toolResultText(result), "unknown sub-agent")
}

func TestExecuteBatch_MaxItemsExceeded(t *testing.T) {
	ar := newTestAgentRun(t)
	ar.AddSubAgent("worker", "worker", "instructions", ar.model, nil)

	items := make([]batchItem, 5)
	for i := range items {
		items[i] = batchItem{ItemID: "i"}
	}

	input := batchInput{SubAgent: "worker", Description: "test", Items: items}
	policy := &BatchPolicy{MaxItems: 3, MaxConcurrency: 2}

	result, err := executeBatch(ar, input, policy)
	require.NoError(t, err)
	assert.True(t, result.Result.Error)
	assert.Contains(t, toolResultText(result), "too many items")
}

func TestExecuteBatch_PersistsResult(t *testing.T) {
	ar := newTestAgentRun(t)
	ar.AddSubAgent("worker", "processes items", "You process items", ar.model, nil)

	input := batchInput{
		SubAgent:    "worker",
		Description: "persist test",
		Items:       []batchItem{{ItemID: "item-1"}},
	}
	policy := &BatchPolicy{MaxConcurrency: 1}

	_, err := executeBatch(ar, input, policy)
	require.NoError(t, err)

	// Check that result.json was persisted under _private/batch/<batchID>/
	ws := ar.AgentContext().Workspace()
	require.NotNil(t, ws)
	batchDir := filepath.Join(ws.RootDir, "_private", "batch")
	entries, err := os.ReadDir(batchDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "batch directory should have at least one entry")

	// Read result.json from the first batch entry
	resultPath := filepath.Join(batchDir, entries[0].Name(), "result.json")
	data, err := os.ReadFile(resultPath)
	require.NoError(t, err)

	var res batchResult
	err = json.Unmarshal(data, &res)
	require.NoError(t, err)
	assert.Equal(t, "completed", res.Status)
}

func TestExpandFileURL_SingleFile(t *testing.T) {
	llmDir := t.TempDir()
	uploads := filepath.Join(llmDir, "uploads")
	require.NoError(t, os.MkdirAll(uploads, 0755))
	f := filepath.Join(uploads, "doc.pdf")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0644))

	items, err := expandFileURL(llmDir, "file://uploads/doc.pdf")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "uploads/doc.pdf", items[0].ItemID)
}

func TestExpandFileURL_Folder(t *testing.T) {
	llmDir := t.TempDir()
	uploads := filepath.Join(llmDir, "uploads")
	require.NoError(t, os.MkdirAll(uploads, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "a.pdf"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "b.pdf"), []byte("b"), 0644))

	items, err := expandFileURL(llmDir, "file://uploads")
	require.NoError(t, err)
	require.Len(t, items, 2)
	ids := []string{items[0].ItemID, items[1].ItemID}
	assert.Contains(t, ids, "uploads/a.pdf")
	assert.Contains(t, ids, "uploads/b.pdf")
}

func TestExpandFileURL_Recursive(t *testing.T) {
	llmDir := t.TempDir()
	base := filepath.Join(llmDir, "uploads", "nested")
	require.NoError(t, os.MkdirAll(base, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "deep.pdf"), []byte("x"), 0644))

	items, err := expandFileURL(llmDir, "file://uploads")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "uploads/nested/deep.pdf", items[0].ItemID)
}

func TestExpandFileURL_EmptyFolder(t *testing.T) {
	llmDir := t.TempDir()
	empty := filepath.Join(llmDir, "empty")
	require.NoError(t, os.MkdirAll(empty, 0755))

	_, err := expandFileURL(llmDir, "file://empty")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestExpandFileURL_NotFound(t *testing.T) {
	llmDir := t.TempDir()
	_, err := expandFileURL(llmDir, "file://nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExpandFileURL_PathEscape(t *testing.T) {
	llmDir := t.TempDir()
	_, err := expandFileURL(llmDir, "file://../../etc/passwd")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "..") || strings.Contains(err.Error(), "agent VM directory"), "error should reject path escape")
}

func TestExpandFileURL_AbsolutePathRejected(t *testing.T) {
	llmDir := t.TempDir()
	// Build absolute path using filepath separator (e.g. /foo on Unix, \foo on Windows)
	absoluteURL := "file://" + string(filepath.Separator) + "foo"
	_, err := expandFileURL(llmDir, absoluteURL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relative")
	assert.Contains(t, err.Error(), "agent VM directory")
}

func TestExpandItems_HTTPURL(t *testing.T) {
	items, err := expandItems(t.TempDir(), []batchItem{{ItemID: "https://example.com/page1"}})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "https://example.com/page1", items[0].ItemID)
}

func TestExpandItems_RegularItem(t *testing.T) {
	items, err := expandItems(t.TempDir(), []batchItem{{ItemID: "task-1"}})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "task-1", items[0].ItemID)
}

func TestExecuteBatch_WithFileURL(t *testing.T) {
	ar := newTestAgentRun(t)
	ar.AddSubAgent("worker", "processes items", "You process items", ar.model, nil)

	llmDir := ar.AgentContext().Workspace().LLMDir
	uploads := filepath.Join(llmDir, "uploads")
	require.NoError(t, os.MkdirAll(uploads, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "f1.pdf"), []byte("1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "f2.pdf"), []byte("2"), 0644))

	input := batchInput{
		SubAgent:    "worker",
		Description: "batch from folder",
		Items:       []batchItem{{ItemID: "file://uploads"}},
	}
	policy := &BatchPolicy{MaxConcurrency: 2}

	result, err := executeBatch(ar, input, policy)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Result.Error)

	var res batchResult
	err = json.Unmarshal([]byte(toolResultText(result)), &res)
	require.NoError(t, err)
	assert.Equal(t, "completed", res.Status)
	assert.Equal(t, 2, res.Total)
	assert.Equal(t, 2, res.Completed)
	assert.Len(t, res.Items, 2)
}

// --- Plan Execution Tests ---

func TestExecutePlan_LinearSteps(t *testing.T) {
	ar := newTestAgentRun(t)
	ar.AddSubAgent("step-agent", "executes steps", "You execute steps", ar.model, nil)

	plan := PlanDef{
		Name:        "linear-plan",
		Description: "A linear plan with two steps",
		Steps: []PlanStep{
			{ID: "step-1", SubAgent: "step-agent"},
			{ID: "step-2", SubAgent: "step-agent", DependsOn: []string{"step-1"}},
		},
	}

	args := map[string]interface{}{
		"description": "test execution",
		"inputs": map[string]interface{}{
			"step-1": "first step input",
		},
	}

	result, err := executePlan(ar, plan, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Result.Error)

	var res map[string]interface{}
	err = json.Unmarshal([]byte(toolResultText(result)), &res)
	require.NoError(t, err)
	assert.Equal(t, "completed", res["status"])
}

func TestExecutePlan_ParallelSteps(t *testing.T) {
	ar := newTestAgentRun(t)
	ar.AddSubAgent("worker", "worker", "You work", ar.model, nil)

	plan := PlanDef{
		Name:        "parallel-plan",
		Description: "A plan with parallel steps",
		Steps: []PlanStep{
			{ID: "a", SubAgent: "worker"},
			{ID: "b", SubAgent: "worker"},
			{ID: "join", SubAgent: "worker", DependsOn: []string{"a", "b"}},
		},
	}

	args := map[string]interface{}{
		"description": "parallel test",
		"inputs": map[string]interface{}{
			"a": "input for a",
			"b": "input for b",
		},
	}

	result, err := executePlan(ar, plan, args)
	require.NoError(t, err)
	assert.False(t, result.Result.Error)

	var res map[string]interface{}
	err = json.Unmarshal([]byte(toolResultText(result)), &res)
	require.NoError(t, err)
	assert.Equal(t, "completed", res["status"])

	steps := res["steps"].([]interface{})
	assert.Len(t, steps, 3)
}

func TestExecutePlan_UnknownSubAgent(t *testing.T) {
	ar := newTestAgentRun(t)

	plan := PlanDef{
		Name: "bad-plan",
		Steps: []PlanStep{
			{ID: "step-1", SubAgent: "nonexistent"},
		},
	}

	args := map[string]interface{}{
		"description": "test",
		"inputs":      map[string]interface{}{"step-1": "input"},
	}

	result, err := executePlan(ar, plan, args)
	require.NoError(t, err)
	// The result should indicate the step failed
	var res map[string]interface{}
	err = json.Unmarshal([]byte(toolResultText(result)), &res)
	require.NoError(t, err)
	assert.Equal(t, "partial", res["status"])
}

func TestExecutePlan_PersistsPlanJSON(t *testing.T) {
	ar := newTestAgentRun(t)
	ar.AddSubAgent("worker", "worker", "You work", ar.model, nil)

	plan := PlanDef{
		Name: "persist-plan",
		Steps: []PlanStep{
			{ID: "only", SubAgent: "worker"},
		},
	}

	args := map[string]interface{}{
		"description": "persist test",
		"inputs":      map[string]interface{}{"only": "input"},
	}

	_, err := executePlan(ar, plan, args)
	require.NoError(t, err)

	// Verify plan.json was persisted under _private/plan/<planID>/
	ws := ar.AgentContext().Workspace()
	require.NotNil(t, ws)
	planDir := filepath.Join(ws.RootDir, "_private", "plan")
	entries, err := os.ReadDir(planDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	planPath := filepath.Join(planDir, entries[0].Name(), "plan.json")
	data, err := os.ReadFile(planPath)
	require.NoError(t, err)

	var stored map[string]interface{}
	err = json.Unmarshal(data, &stored)
	require.NoError(t, err)
	assert.Equal(t, "persist-plan", stored["plan"])
}

// --- Dynamic Planning Tests ---

func TestCreatePlan_Valid(t *testing.T) {
	ar := newTestAgentRun(t)

	input := createPlanInput{
		Goal: "test goal",
		Tasks: []dynamicPlanTask{
			{ID: "t1", Instructions: "do first thing", DependsOn: []string{}},
			{ID: "t2", Instructions: "do second thing", DependsOn: []string{"t1"}},
		},
	}

	result, err := executeCreatePlan(ar, input)
	require.NoError(t, err)
	assert.False(t, result.Result.Error)

	var res map[string]interface{}
	err = json.Unmarshal([]byte(toolResultText(result)), &res)
	require.NoError(t, err)
	assert.Equal(t, "frozen", res["status"])
	assert.NotEmpty(t, res["plan_id"])
}

func TestCreatePlan_EmptyTasks(t *testing.T) {
	ar := newTestAgentRun(t)

	input := createPlanInput{Goal: "test", Tasks: []dynamicPlanTask{}}
	result, err := executeCreatePlan(ar, input)
	require.NoError(t, err)
	assert.True(t, result.Result.Error)
	assert.Contains(t, toolResultText(result), "at least one task")
}

func TestCreatePlan_TooManyTasks(t *testing.T) {
	ar := newTestAgentRun(t)

	tasks := make([]dynamicPlanTask, 21)
	for i := range tasks {
		tasks[i] = dynamicPlanTask{ID: "t" + string(rune('a'+i)), Instructions: "x", DependsOn: []string{}}
	}

	input := createPlanInput{Goal: "test", Tasks: tasks}
	result, err := executeCreatePlan(ar, input)
	require.NoError(t, err)
	assert.True(t, result.Result.Error)
	assert.Contains(t, toolResultText(result), "at most 20")
}

func TestCreatePlan_CyclicDAG(t *testing.T) {
	ar := newTestAgentRun(t)

	input := createPlanInput{
		Goal: "test",
		Tasks: []dynamicPlanTask{
			{ID: "a", Instructions: "x", DependsOn: []string{"b"}},
			{ID: "b", Instructions: "y", DependsOn: []string{"a"}},
		},
	}

	result, err := executeCreatePlan(ar, input)
	require.NoError(t, err)
	assert.True(t, result.Result.Error)
	assert.Contains(t, toolResultText(result), "invalid plan")
}

func TestExecuteStoredPlan_NotFound(t *testing.T) {
	ar := newTestAgentRun(t)

	result, err := executeStoredPlan(ar, "plan_nonexistent")
	require.NoError(t, err)
	assert.True(t, result.Result.Error)
	assert.Contains(t, toolResultText(result), "plan not found")
}

func TestCreateAndExecutePlan_RoundTrip(t *testing.T) {
	ar := newTestAgentRun(t)

	// Create a plan
	input := createPlanInput{
		Goal: "round trip test",
		Tasks: []dynamicPlanTask{
			{ID: "step-1", Instructions: "do the first thing", DependsOn: []string{}},
			{ID: "step-2", Instructions: "do the second thing", DependsOn: []string{"step-1"}},
		},
	}

	createResult, err := executeCreatePlan(ar, input)
	require.NoError(t, err)
	assert.False(t, createResult.Result.Error)

	// Extract plan_id
	var createRes map[string]interface{}
	err = json.Unmarshal([]byte(toolResultText(createResult)), &createRes)
	require.NoError(t, err)
	planID := createRes["plan_id"].(string)
	assert.True(t, strings.HasPrefix(planID, "plan_"))

	// Execute the frozen plan
	execResult, err := executeStoredPlan(ar, planID)
	require.NoError(t, err)
	assert.False(t, execResult.Result.Error)

	var execRes map[string]interface{}
	err = json.Unmarshal([]byte(toolResultText(execResult)), &execRes)
	require.NoError(t, err)
	assert.Equal(t, "completed", execRes["status"])

	steps := execRes["steps"].([]interface{})
	assert.Len(t, steps, 2)
}

// --- AddExecutionTools Tests ---

func TestAddExecutionTools_BatchOnly(t *testing.T) {
	ar := newTestAgentRun(t)

	initialToolCount := len(ar.tools)
	AddExecutionTools(ar, ExecutionToolsConfig{
		BatchPolicy: &BatchPolicy{MaxConcurrency: 5},
	})

	assert.Equal(t, initialToolCount+1, len(ar.tools), "should add agent_batch tool")
	assert.Equal(t, "agent_batch", ar.tools[len(ar.tools)-1].Name)
}

func TestAddExecutionTools_PlansOnly(t *testing.T) {
	ar := newTestAgentRun(t)

	initialToolCount := len(ar.tools)
	AddExecutionTools(ar, ExecutionToolsConfig{
		Plans: []PlanDef{
			{Name: "my-plan", Description: "test plan", Steps: []PlanStep{{ID: "s1", SubAgent: "w"}}},
			{Name: "other-plan", Description: "another", Steps: []PlanStep{{ID: "s1", SubAgent: "w"}}},
		},
	})

	assert.Equal(t, initialToolCount+2, len(ar.tools), "should add 2 plan tools")
	assert.Equal(t, "my_plan", ar.tools[len(ar.tools)-2].Name)
	assert.Equal(t, "other_plan", ar.tools[len(ar.tools)-1].Name)
}

func TestAddExecutionTools_DynamicPlanning(t *testing.T) {
	ar := newTestAgentRun(t)

	AddExecutionTools(ar, ExecutionToolsConfig{
		DynamicPlanning: true,
	})

	// Should have injected create_plan, execute_plan, and planner sub-agent
	toolNames := make(map[string]bool)
	for _, tool := range ar.tools {
		toolNames[tool.Name] = true
	}
	assert.True(t, toolNames["create_plan"], "should have create_plan tool")
	assert.True(t, toolNames["execute_plan"], "should have execute_plan tool")

	// Planner sub-agent should be registered
	_, hasPlanner := ar.subAgentDefs["planner"]
	assert.True(t, hasPlanner, "should have planner sub-agent")
}

// --- Reload State Tests ---

func TestLoadBatchState_NonExistent(t *testing.T) {
	result, err := loadBatchState(filepath.Join(t.TempDir(), "nonexistent"))
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestLoadBatchState_ValidResult(t *testing.T) {
	dir := t.TempDir()
	res := batchResult{
		SubAgent:  "worker",
		Status:    "completed",
		Total:     3,
		Completed: 3,
		Failed:    0,
		Items: []batchItemResult{
			{ItemID: "i1", Status: "completed", Output: "done"},
		},
	}
	data, _ := json.Marshal(res)
	os.WriteFile(filepath.Join(dir, "result.json"), data, 0644)

	loaded, err := loadBatchState(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "completed", loaded.Status)
	assert.Equal(t, 3, loaded.Total)
}

func TestLoadPlanState_NonExistent(t *testing.T) {
	result, err := loadPlanState(filepath.Join(t.TempDir(), "nonexistent"))
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestLoadPlanState_ValidResult(t *testing.T) {
	dir := t.TempDir()
	state := map[string]interface{}{
		"plan_id": "plan_abc",
		"goal":    "test",
		"tasks":   []interface{}{},
		"status":  "frozen",
	}
	data, _ := json.Marshal(state)
	os.WriteFile(filepath.Join(dir, "plan.json"), data, 0644)

	loaded, err := loadPlanState(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "plan_abc", loaded.PlanID)
	assert.Equal(t, "frozen", loaded.Status)
}
