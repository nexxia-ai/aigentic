package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"benchmark/core"

	gemini "github.com/nexxia-ai/aigentic-google"
	ollama "github.com/nexxia-ai/aigentic-ollama"
	openai "github.com/nexxia-ai/aigentic-openai"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/utils"
)

type Capability struct {
	Name         string
	RunFunction  func(*ai.Model) (core.BenchResult, error)
	EvalFunction func(*ai.Model, *ai.Model) // Optional evaluation function (model, scoreModel)
}

var capabilities = []Capability{
	{Name: "SimpleAgent", RunFunction: core.RunSimpleAgent},
	{Name: "ToolIntegration", RunFunction: core.RunToolIntegration},
	{Name: "TeamCoordination", RunFunction: core.RunTeamCoordination},
	{Name: "FileAttachments", RunFunction: core.RunFileAttachmentsAgent},
	{Name: "MultiAgentChain", RunFunction: core.RunMultiAgentChain},
	{Name: "MultiAgentVariations", RunFunction: core.RunMultiAgentVariationsWrapper},
	{Name: "ConcurrentRuns", RunFunction: core.RunConcurrentRuns},
	{Name: "Streaming", RunFunction: core.RunStreaming},
	{Name: "StreamingWithTools", RunFunction: core.RunStreamingWithTools},
	{Name: "MemoryPersistence", RunFunction: core.RunMemoryPersistenceAgent},
}

type ModelDesc struct {
	Name         string
	ProviderFunc func(modelName string) *ai.Model
}

func openAIProvider(modelName string) *ai.Model {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		slog.Error("OPENAI_API_KEY is not set")
		return nil
	}
	return openai.NewModel(modelName, apiKey)
}

func ollamaProvider(modelName string) *ai.Model {
	return ollama.NewModel(modelName, "")
}

func geminiProvider(modelName string) *ai.Model {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		slog.Error("GOOGLE_API_KEY is not set")
		return nil
	}
	return gemini.NewGeminiModel(modelName, apiKey)
}

var modelsTable = []ModelDesc{
	{Name: "gpt-4o-mini", ProviderFunc: openAIProvider},
	{Name: "gpt-4o", ProviderFunc: openAIProvider},
	{Name: "gpt", ProviderFunc: openAIProvider},
	{Name: "qwen", ProviderFunc: ollamaProvider},
	{Name: "llama3.2", ProviderFunc: ollamaProvider},
	{Name: "gemma", ProviderFunc: ollamaProvider},
	{Name: "deepseek", ProviderFunc: ollamaProvider},
	{Name: "gemini", ProviderFunc: geminiProvider},
}

func main() {
	// Load environment variables
	utils.LoadEnvFile("./.env")

	// Define command-line flags
	var testsFlag string
	var evalMode bool
	flag.StringVar(&testsFlag, "test", "", "Comma-separated list of tests to run (case-insensitive)")
	flag.BoolVar(&evalMode, "eval", false, "Run evaluation mode for tests that support it")
	flag.Parse()

	// Get remaining arguments (model names)
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: go run main.go [-test \"test1,test2\"] [-eval] <model_name> [model_name...]")
		fmt.Println("\nAvailable models:")
		for _, model := range modelsTable {
			fmt.Printf("  %-s\n", model.Name)
		}
		fmt.Println("\nAvailable tests:")
		for _, cap := range capabilities {
			evalSupport := ""
			if cap.EvalFunction != nil {
				evalSupport = " (supports -eval)"
			}
			fmt.Printf("  %s%s\n", cap.Name, evalSupport)
		}
		fmt.Println("\nExamples:")
		fmt.Println("  go run main.go gpt-4o-mini gemma3:12b")
		fmt.Println("  go run main.go -test \"SimpleAgent,ToolIntegration\" qwen gpt-4o")
		fmt.Println("  go run main.go -eval -test \"MultiAgentChain\" gpt-4o-mini")
		fmt.Println("  go run main.go -eval -test \"MultiAgentContextManager\" gpt-4o-mini")
		os.Exit(1)
	}

	modelName := strings.Join(args, " ")

	// Parse individual model names from the input
	modelNames := strings.Fields(modelName)

	models := []*ai.Model{}
	for _, name := range modelNames {
		model := createModel(name)
		if model == nil {
			fmt.Printf("Model unknown or missing authentication: %s\n", name)
			fmt.Println("\nAvailable models:")
			for _, modelDesc := range modelsTable {
				fmt.Printf("  %s\n", modelDesc.Name)
			}
			os.Exit(1)
		}
		models = append(models, model)
	}

	if len(models) == 0 {
		fmt.Println("No valid models specified")
		os.Exit(1)
	}

	// Filter capabilities based on test flag
	filteredCapabilities := filterCapabilities(testsFlag)

	if evalMode {
		runEvaluationMode(models, filteredCapabilities)
	} else {
		runModels(models, filteredCapabilities)
	}
}

func filterCapabilities(testsFlag string) []Capability {
	if testsFlag == "" {
		return capabilities
	}

	// Parse comma-separated test names
	testNames := strings.Split(testsFlag, ",")
	for i, name := range testNames {
		testNames[i] = strings.TrimSpace(name)
	}

	var filtered []Capability
	for _, capability := range capabilities {
		for _, testName := range testNames {
			if strings.EqualFold(capability.Name, testName) {
				filtered = append(filtered, capability)
				break
			}
		}
	}

	if len(filtered) == 0 {
		fmt.Printf("No matching tests found for: %s\n", testsFlag)
		fmt.Println("\nAvailable tests:")
		for _, cap := range capabilities {
			fmt.Printf("  %s\n", cap.Name)
		}
		os.Exit(1)
	}

	return filtered
}

func runModels(models []*ai.Model, capabilitiesToRun []Capability) {
	allResults := make([][]core.BenchResult, len(models))

	for index, model := range models {
		fmt.Printf("\n🤖 Testing %s\n", model.ModelName)
		fmt.Println("-" + fmt.Sprintf("%30s", "-"))

		results := []core.BenchResult{}
		for _, testCase := range capabilitiesToRun {
			fmt.Printf("  %s... ", testCase.Name)

			result, err := testCase.RunFunction(model)
			results = append(results, result)
			if err != nil {
				fmt.Printf("❌ FAILED (%v)\n", result.Duration)
			} else {
				fmt.Printf("✅ SUCCESS (%v)\n", result.Duration)
			}
		}
		allResults[index] = results
	}

	generateComparisonReport(allResults)
}

func runEvaluationMode(models []*ai.Model, capabilitiesToRun []Capability) {
	fmt.Println("🔍 Running in Evaluation Mode")
	fmt.Println("=" + strings.Repeat("=", 40))

	if len(models) < 1 {
		fmt.Println("❌ Evaluation mode requires at least 1 model")
		os.Exit(1)
	}

	// Use first model as primary, second as scoring model (or same if only one)
	primaryModel := models[0]
	scoreModel := primaryModel
	if len(models) > 1 {
		scoreModel = models[1]
		fmt.Printf("📊 Using %s for scoring evaluations\n", scoreModel.ModelName)
	} else {
		fmt.Printf("📊 Using %s for both testing and scoring\n", primaryModel.ModelName)
	}

	fmt.Printf("🤖 Primary model: %s\n\n", primaryModel.ModelName)

	for _, capability := range capabilitiesToRun {
		fmt.Printf("🔬 Evaluating %s...\n", capability.Name)
		fmt.Println("-" + strings.Repeat("-", 40))

		if capability.EvalFunction != nil {
			// Use custom evaluation function if available
			capability.EvalFunction(primaryModel, scoreModel)
		} else {
			// Run standard benchmark with evaluation enabled
			runCapabilityWithEval(capability, primaryModel)
		}

		fmt.Println()
	}

	fmt.Println("✅ Evaluation complete!")
}

// runCapabilityWithEval runs a capability with evaluation enabled
func runCapabilityWithEval(capability Capability, model *ai.Model) {
	fmt.Printf("Running %s with evaluation instrumentation...\n", capability.Name)

	// For capabilities that don't have custom eval functions,
	// we would need to modify them to enable evaluation
	// For now, just run the regular function
	result, err := capability.RunFunction(model)

	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Printf("✅ Completed in %v\n", result.Duration)
		if result.Success {
			fmt.Printf("📊 Result: Success\n")
		} else {
			fmt.Printf("📊 Result: Failed - %s\n", result.ErrorMessage)
		}
	}
}

func createModel(modelName string) *ai.Model {
	for _, modelDesc := range modelsTable {
		if modelDesc.Name == modelName {
			return modelDesc.ProviderFunc(modelName)
		}
	}

	for _, modelDesc := range modelsTable {
		if strings.HasPrefix(modelName, modelDesc.Name) {
			return modelDesc.ProviderFunc(modelName)
		}
	}
	return nil
}

func generateComparisonReport(results [][]core.BenchResult) {
	if len(results) == 0 {
		return
	}

	// Group results by test case and model
	testGroups := make(map[string]map[string]core.BenchResult)
	allModels := make(map[string]bool)
	allCapabilities := make(map[string]bool)

	for _, modelResults := range results {
		for _, result := range modelResults {
			if testGroups[result.TestCase] == nil {
				testGroups[result.TestCase] = make(map[string]core.BenchResult)
			}
			testGroups[result.TestCase][result.ModelName] = result
			allModels[result.ModelName] = true
			allCapabilities[result.TestCase] = true
		}
	}

	// Convert maps to sorted slices
	var models []string
	for model := range allModels {
		models = append(models, model)
	}

	var capabilities []string
	for capability := range allCapabilities {
		capabilities = append(capabilities, capability)
	}

	report := "# Model Comparison Report\n\n"
	report += fmt.Sprintf("Generated on: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Create header row
	report += "| Capability"
	for _, model := range models {
		report += fmt.Sprintf(" | %s", model)
	}
	report += " |\n"

	// Create separator row
	report += "|---"
	for range models {
		report += "|---"
	}
	report += "|\n"

	// Create rows for each capability
	for _, capability := range capabilities {
		// Success/Failure row
		report += fmt.Sprintf("| %s", capability)
		for _, model := range models {
			result, exists := testGroups[capability][model]
			if !exists {
				report += " | N/A"
			} else if result.Success {
				report += " | ✅ Success"
			} else {
				report += " | ❌ Failure"
			}
		}
		report += " |\n"

		// Timing row
		report += fmt.Sprintf("| %s (timing)", capability)
		for _, model := range models {
			result, exists := testGroups[capability][model]
			if !exists {
				report += " | N/A"
			} else {
				// Format duration to show seconds with 1 decimal place
				seconds := result.Duration.Seconds()
				report += fmt.Sprintf(" | %.1fs", seconds)
			}
		}
		report += " |\n"
	}

	filename := "comparison_report.md"
	err := os.WriteFile(filename, []byte(report), 0644)
	if err != nil {
		fmt.Printf("Error writing comparison report: %v\n", err)
		return
	}

	fmt.Printf("📊 Comparison report generated: %s\n", filename)
}
