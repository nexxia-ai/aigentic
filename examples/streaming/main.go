package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/nexxia-ai/aigentic"
	openai "github.com/nexxia-ai/aigentic-openai"
	"github.com/nexxia-ai/aigentic/utils"
)

func main() {
	utils.LoadEnvFile("./.env")

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go \"Your question here\"")
		fmt.Println("Example: go run main.go \"Tell me about artificial intelligence\"")
		os.Exit(1)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	model := openai.NewModel("gpt-4o-mini", apiKey)

	agent := aigentic.Agent{
		Model:        model,
		Description:  "You are a helpful AI assistant that provides clear and informative responses.",
		Instructions: "Provide detailed explanations and be helpful. When answering questions, be thorough but concise.",
		Stream:       true,
		Trace:        aigentic.NewTrace(),
	}

	question := strings.Join(os.Args[1:], " ")
	fmt.Printf("Question: %s\n", question)
	fmt.Println("Streaming response:")
	fmt.Println("==================")

	run, err := agent.Start(question)
	if err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	var fullResponse string
	for ev := range run.Next() {
		switch e := ev.(type) {
		case *aigentic.ContentEvent:
			fmt.Print(e.Content)
			fullResponse += e.Content
		case *aigentic.ToolEvent:
			fmt.Printf("\n[Tool called: %s]\n", e.ToolName)
		case *aigentic.ApprovalEvent:
			run.Approve(e.ApprovalID, true)
		case *aigentic.ErrorEvent:
			log.Fatalf("Error during streaming: %v", e.Err)
		}
	}

	fmt.Println("\n==================")
	fmt.Printf("Full response received (%d characters)\n", len(fullResponse))
}
