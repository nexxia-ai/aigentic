# Aigentic Evaluation System

## Quick Start

```go
// Enable evaluation on any agent
agent := core.NewMultiAgentChainAgent(model).WithEvaluation()

// Create evaluation suite
eval := evals.NewEvalSuite("MultiAgent Test")
eval.AddCheck("has experts", evals.HasKeywords("expert1", "expert2", "expert3"))
eval.AddCheck("calls memory", evals.CallsTools("save_memory"))
eval.AddCheck("responds quickly", evals.LatencyUnder(5*time.Second))
eval.AddCheck("no errors", evals.NoErrors())

// Run and evaluate
run, _ := agent.Start("get names of expert1, expert2, expert3")
for event := range run.Next() {
    if evalEvent, ok := event.(*aigentic.EvalEvent); ok {
        results := eval.Evaluate(*evalEvent)
        evals.PrintResults(results)
    }
}
```

## Built-in Checks

### Accuracy Checks (Behavioral correctness)
```go
eval.AddCheck("tool accuracy", evals.ToolAccuracy(map[string][]string{
    "save_memory": []string{"plan", "progress", "step"},
    "expert1":     []string{"name", "contact", "expert1"},
}))
eval.AddCheck("calls tools", evals.CallsTools("save_memory"))
eval.AddCheck("no errors", evals.NoErrors())
eval.AddCheck("sequence", evals.SequenceCheck(1)) // First call
eval.AddCheck("behavioral", evals.BehavioralAccuracy(
    []string{"sequential", "wait", "plan", "memory"}, 1.0))
```

### Relevance Checks (Content appropriateness)
```go
eval.AddCheck("content relevance", evals.ContentRelevance(
    []string{"expert1", "expert2", "table", "company"}, 1.0))
eval.AddCheck("keywords", evals.HasKeywords("expert1", "expert2"))
eval.AddCheck("has content", evals.HasContent(10))
```

## Custom Checks

```go
customCheck := func(event aigentic.EvalEvent) (bool, float64, string) {
    hasTable := strings.Contains(event.Response.Content, "table")
    score := 0.0
    if hasTable { score = 1.0 }
    return hasTable, score, fmt.Sprintf("table mentioned: %t", hasTable)
}
eval.AddCheck("has table", customCheck)
```

## Report Output

```
=== Evaluation Summary ===
Total LLM Calls: 4
Pass Rate: 85.7% (24/28)
Average Score: 0.89
Total Duration: 12.3s

=== Quality Metrics ===
📊 Accuracy Score: 0.92/1.00
🎯 Relevance Score: 0.87/1.00
✅ Overall Quality: Good (0.90)

=== Detailed Results ===
✅ content relevance: 0.87 - found 4/5 elements
✅ behavioral accuracy: 0.92 - found 5/6 behaviors
✅ tool accuracy: 0.89 - 8/9 tool calls correct
❌ expert keywords: 0.67 - found 2/3 keywords
✅ calls tools correctly: 1.00 - called 1/1 expected tools
✅ no errors: 1.00 - no errors
✅ responds quickly: 1.00 - took 2.8s (max 30s)
```

## Multi-Call Analysis

For agents with multiple LLM calls:

```
--- Call 1 (Planning) ---
✅ content relevance: 0.95 - found planning elements
✅ behavioral accuracy: 0.90 - shows sequential thinking
✅ calls tools correctly: 1.00 - called save_memory appropriately

--- Call 2 (Expert Contact) ---  
✅ content relevance: 0.85 - expert1 mentioned correctly
✅ tool accuracy: 0.90 - expert1 called with proper context
❌ behavioral accuracy: 0.60 - didn't wait explicitly
```

## Benchmark Integration

```bash
go run main.go -eval -test "MultiAgentChain" gpt-4o-mini
```

## Key Features

- **Zero Code Changes**: Existing agents work unchanged
- **Event-Driven**: Uses existing event system
- **Real-time**: Evaluate calls as they happen
- **Automatic Classification**: Accuracy vs relevance scoring
- **Call-level Granularity**: Per-call performance analysis
- **Comparative Analysis**: Easy A/B testing of prompts
