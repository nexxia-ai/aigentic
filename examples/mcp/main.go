package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nexxia-ai/aigentic"
	openai "github.com/nexxia-ai/aigentic-openai"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/utils"
)

var config = &ai.MCPConfig{
	MCPServers: map[string]ai.ServerConfig{
		"fetch": {
			Command: "uvx",
			Args:    []string{"mcp-server-fetch"},
		},
		"files": {
			Command: "go",
			Args:    []string{"run", "github.com/mark3labs/mcp-filesystem-server@latest", "./"},
		},
	},
}

func init() {
	utils.LoadEnvFile("./.env")
}

func main() {
	mcpHost, err := ai.NewMCPHost(config)
	if err != nil {
		log.Fatal(err)
	}
	defer mcpHost.Close()

	agentTools := []aigentic.AgentTool{}
	for _, client := range mcpHost.Clients {
		for _, tool := range client.Tools {
			agentTools = append(agentTools, aigentic.WrapTool(tool))
		}
	}

	agent := aigentic.Agent{
		Model:       openai.NewModel("gpt-4o-mini", os.Getenv("OPENAI_API_KEY")),
		Name:        "News Agent",
		Description: "You are a news agent that fetches the latest news from the website and saves it to a file",
		Instructions: `
		Fetch the first 4000 characters only.
		Use the fetch tool to fetch the latest news. 
		Use the fetch tool once only; even if the response is incomplete.
		Do not save to memory.
		`,

		AgentTools: agentTools,
		Trace:      aigentic.NewTrace(),
		// IncludeHistory: true,
	}
	result, err := agent.Execute("Fetch the latest news from the abc.com.au, format it in markdown and save it to a file called ./news.md. ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}
