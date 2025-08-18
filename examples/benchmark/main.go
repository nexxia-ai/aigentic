package main

import (
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
	Name        string
	RunFunction func(*ai.Model) (core.BenchResult, error)
}

var capabilities = []Capability{
	{Name: "SimpleAgent", RunFunction: core.RunSimpleAgent},
	{Name: "ToolIntegration", RunFunction: core.RunToolIntegration},
	{Name: "TeamCoordination", RunFunction: core.RunTeamCoordination},
	{Name: "FileAttachments", RunFunction: core.RunFileAttachmentsAgent},
	{Name: "MultiAgentChain", RunFunction: core.RunMultiAgentChain},
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
	{Name: "qwen", ProviderFunc: ollamaProvider},
	{Name: "llama3.2", ProviderFunc: ollamaProvider},
	{Name: "gemma", ProviderFunc: ollamaProvider},
	{Name: "deepseek", ProviderFunc: ollamaProvider},
	{Name: "gemini", ProviderFunc: geminiProvider},
}

func main() {
	// Load environment variables
	utils.LoadEnvFile("./.env")

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <model_name>")
		fmt.Println("\nAvailable models:")
		for _, model := range modelsTable {
			fmt.Printf("  %-s\n", model.Name)
		}
		fmt.Println("\nExamples:")
		fmt.Println("  go run main.go gpt-4o-mini gemma3:12b")
		os.Exit(1)
	}

	modelName := strings.Join(os.Args[1:], " ")

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

	runModels(models)
}

func runModels(models []*ai.Model) {
	allResults := make([][]core.BenchResult, len(models))

	for index, model := range models {
		fmt.Printf("\n🤖 Testing %s\n", model.ModelName)
		fmt.Println("-" + fmt.Sprintf("%30s", "-"))

		results := []core.BenchResult{}
		for _, testCase := range capabilities {
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
