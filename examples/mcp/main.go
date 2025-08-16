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

	agentTools := []aigentic.AgentTool{}
	for _, tool := range mcpHost.Tools {
		agentTools = append(agentTools, aigentic.WrapTool(*tool))
	}

	agent := aigentic.Agent{
		Model:       openai.NewModel("gpt-4o-mini", os.Getenv("OPENAI_API_KEY")),
		Name:        "Test agent",
		Description: "This is a test agent",
		AgentTools:  agentTools,
	}
	result, err := agent.Execute("Fetch the latest news from the abc.com.au ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}
