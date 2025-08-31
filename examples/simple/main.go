package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/nexxia-ai/aigentic"
	ollama "github.com/nexxia-ai/aigentic-ollama"
	openai "github.com/nexxia-ai/aigentic-openai"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	"github.com/nexxia-ai/aigentic/utils"
)

var (
	openaiModel = openai.NewModel("gpt-4o-mini", getAPIKey())
	ollamaModel = ollama.NewModel("qwen3:1.7b", "")

	// change this if you like to use ollama instead of openai
	model = openaiModel
)

func getAPIKey() string {
	utils.LoadEnvFile("./.env")
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY environment variable not set")
		fmt.Println("Please set your OpenAI API key: export OPENAI_API_KEY=your_api_key_here")
		os.Exit(1)
	}
	return apiKey
}

// simpleAgent demonstrates a simple agent that takes an user input and returns a response
var simpleAgent = aigentic.Agent{
	Model:        ollamaModel, // show use of ollama model
	Name:         "SimpleAgent",
	Description:  "A simple agent that responds to user messages",
	Instructions: "You are a helpful assistant. Respond to user questions in a friendly and informative way.",
}

// attachmentAgent demonstrates an agent that accepts file attachments
var doc = document.NewInMemoryDocument("", "sample.txt", []byte("This is a sample text file with some information about artificial intelligence."), nil)
var attachmentAgent = aigentic.Agent{
	Model:        model,
	Name:         "AttachmentAgent",
	Description:  "An agent that can analyze and work with file attachments",
	Instructions: "You can analyze images and documents. Describe what you see and provide insights about the content.",
	Documents:    []*document.Document{doc},
}

// multiAgent demonstrates a multi-agent system that can delegate tasks to other agents
var multiAgent = aigentic.Agent{
	Model:        model,
	Name:         "MultiAgent",
	Description:  "An agent that coordinates with other agents to complete complex tasks",
	Instructions: "You can delegate tasks to other agents. Use the ResearchAgent for detailed research tasks.",
	LogLevel:     slog.LevelDebug,
	Agents: []aigentic.Agent{
		{
			Model:        model,
			Name:         "ResearchAgent",
			Description:  "A specialized agent for researching topics",
			Instructions: "You are a research specialist. Provide detailed information about topics you're asked about.",
		},
	},
}

// toolAgent demonstrates an agent that uses tools to perform tasks
var toolAgent = aigentic.Agent{
	Model:        model,
	Name:         "ToolAgent",
	Description:  "An agent that can perform mathematical calculations using tools",
	Instructions: "You have access to a calculator tool. Use it to help users with mathematical calculations.",
	AgentTools:   []aigentic.AgentTool{createCalculatorTool()},
}

// createCalculatorTool creates a tool that returns a greeting
func createCalculatorTool() aigentic.AgentTool {
	return aigentic.AgentTool{
		Name:        "greeting",
		Description: "A tool that returns a greeting",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the person to greet",
				},
			},
			"required": []string{"name"},
		},
		Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
			name, ok := args["name"].(string)
			if !ok {
				return &ai.ToolResult{
					Content: []ai.ToolContent{{
						Type:    "text",
						Content: "Error: name must be a string",
					}},
					Error: true,
				}, nil
			}

			result := fmt.Sprintf("Hello, %s! Have a nice day!", name)
			return &ai.ToolResult{
				Content: []ai.ToolContent{{
					Type:    "text",
					Content: fmt.Sprintf("Result: %s", result),
				}},
			}, nil
		},
	}
}

func main() {
	fmt.Println("🤖 Aigentic Agent Showcase")
	fmt.Println("==========================")
	fmt.Println()

	// Run Simple Agent
	fmt.Println("=== Running Simple Agent ===")
	response, err := simpleAgent.Execute("Hello! Can you tell me a fun fact about space?")
	if err != nil {
		log.Fatalf("Error running simple agent: %v", err)
	}
	fmt.Printf("Simple Agent Response: %s\n\n", response)

	// Run Tool Agent
	fmt.Println("=== Running Tool Agent ===")
	response, err = toolAgent.Execute("I am Nexxia")
	if err != nil {
		log.Fatalf("Error running tool agent: %v", err)
	}
	fmt.Printf("Tool Agent Response: %s\n\n", response)

	// Run Attachment Agent
	fmt.Println("=== Running Attachment Agent ===")
	response, err = attachmentAgent.Execute("Please analyze this text file and tell me what it contains.")
	if err != nil {
		log.Fatalf("Error running attachment agent: %v", err)
	}
	fmt.Printf("Attachment Agent Response: %s\n\n", response)

	// Run Multi Agent
	fmt.Println("=== Running Multi Agent ===")
	response, err = multiAgent.Execute("I need information about quantum computing. Can you research this topic for me?")
	if err != nil {
		log.Fatalf("Error running multi agent: %v", err)
	}
	fmt.Printf("Multi Agent Response: %s\n\n", response)

	fmt.Println("✅ All agents completed successfully!")
}
