# aigentic

**Declarative AI agent framework for Go**

aigentic is a production-ready Go framework for building AI-powered applications with sophisticated agent orchestration, designed to address the lack of agentic frameworks in Go that are comparable to Python. Its design was greatly influenced by agno.com.

aigentic provides a declarative solution where you declare agents and their capabilities, then run them with a simple, clean API.

---

## Why aigentic?

- **Declarative Agent Design**: Define agents and let the framework handle execution
- **Multi-Provider Support**: OpenAI, Ollama, Google Gemini, etc with unified interface  
- **Production Ready**: Built-in retries, rate limiting, tracing, and error handling
- **Real-time Streaming**: Live content generation with event-driven architecture
- **Multi-Agent Orchestration**: Teams of specialized agents working together
- **Document Processing**: Native support for PDFs, images, and multi-modal content
- **Tool Integration**: Extensible tool system with approval workflows
- **Context Management**: Plugable context manager
- **Built in evaluations**: Run A/B evaluations using your Agent to identify the better prompt for accuracy and relevance
- **Type Safety**: Full Go type safety with comprehensive error handling

## Key Features

- **🤖 Simple Agent Creation** - Declarative agent configuration
- **🔄 Streaming Responses** - Real-time content generation with chunked delivery
- **🛠️ Tool Integration** - Custom tools with validation and approval workflows
- **📄 Document Support** - Multi-modal document processing (PDF, images, text)
- **👥 Multi-Agent Teams** - Coordinated agent collaboration
- **💾 Persistent Memory** - Context retention across sessions
- **🔍 Built-in Tracing** - Comprehensive execution monitoring
- **⚡ Event-Driven** - Real-time progress updates and user interaction
- **🔒 Human-in-the-Loop** - Approval workflows for sensitive operations
- **🎯 Provider Agnostic** - Support for OpenAI, Ollama, Google Gemini, and others.

---

## Quick Start

### Simple Agent

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/nexxia-ai/aigentic"
    openai "github.com/nexxia-ai/aigentic-openai"
)

func main() {
    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "Assistant",
        Description:  "A helpful AI assistant",
        Instructions: "You are a friendly and knowledgeable assistant.",
    }
    
    response, err := agent.Execute("What is the capital of France?")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Response:", response)
}
```

### Streaming Agent

```go
func main() {
    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "StreamingAssistant",
        Description:  "An assistant that streams responses",
        Instructions: "Provide detailed explanations step by step.",
        Stream:       true, // Enable streaming
    }
    
    run, err := agent.Start("Explain quantum computing in simple terms")
    if err != nil {
        log.Fatal(err)
    }
    
    // Process real-time events
    for event := range run.Next() {
        switch e := event.(type) {
        case *aigentic.ContentEvent:
            fmt.Print(e.Content) // Stream content as it generates
        case *aigentic.ThinkingEvent:
            fmt.Printf("\n🤔 Thinking: %s\n", e.Thought)
        case *aigentic.ErrorEvent:
            fmt.Printf("\n❌ Error: %v\n", e.Err)
        }
    }
}
```

### Tool Integration

```go
func main() {
    // Create a custom calculator tool
    calculatorTool := aigentic.AgentTool{
        Name:        "calculator",
        Description: "Performs mathematical calculations",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "expression": map[string]interface{}{
                    "type":        "string",
                    "description": "Mathematical expression to evaluate",
                },
            },
            "required": []string{"expression"},
        },
        Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
            expr := args["expression"].(string)
            result := evaluateExpression(expr) // Your implementation
            
            return &ai.ToolResult{
                Content: []ai.ToolContent{{
                    Type:    "text", 
                    Content: fmt.Sprintf("Result: %v", result),
                }},
            }, nil
        },
    }

    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "MathAssistant",
        Description:  "An assistant that can perform calculations",
        Instructions: "Use the calculator tool for mathematical operations.",
        AgentTools:   []aigentic.AgentTool{calculatorTool},
    }

    response, _ := agent.Execute("What is 15 * 23 + 100?")
    fmt.Println(response)
}
```

### Document Usage

```go
func main() {
    // Load document from file or memory
    doc := aigentic.NewInMemoryDocument("", "report.pdf", pdfData, nil)
    
    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "DocumentAnalyst", 
        Description:  "Analyzes documents and extracts insights",
        Instructions: "Analyze the provided documents and summarize key findings.",
        Documents:    []*aigentic.Document{doc},
    }

    response, _ := agent.Execute("What are the main points in this document?")
    fmt.Println(response)
}
```

### Multi-Agent Setup

```go
func main() {
    // Create specialized agents
    researchAgent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "Researcher",
        Description:  "Expert at gathering and analyzing information",
        Instructions: "Conduct thorough research and provide detailed findings.",
    }

    writerAgent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "Writer", 
        Description:  "Expert at creating engaging content",
        Instructions: "Write clear, engaging content based on research.",
    }

    // Coordinator agent that manages the team
    coordinator := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "ProjectManager",
        Description:  "Coordinates research and writing tasks",
        Instructions: "Delegate tasks to specialists and synthesize results.",
        
        // Team members become available as tools
        Agents: []aigentic.Agent{researchAgent, writerAgent},
    }

    // The coordinator automatically delegates to team members
    response, _ := coordinator.Execute("Write an article about renewable energy trends")
    fmt.Println(response)
}
```

---

## Advanced Features

### Memory and Sessions (coming soon)

```go
session := aigentic.NewSession(context.Background())

agent := aigentic.Agent{
    Model:   openai.NewModel("gpt-4o-mini", "your-api-key"),
    Session: session,    // Shared memory across conversations
    Memory:  aigentic.NewMemory(), // Persistent context
}

// First conversation
agent.Execute("My name is John and I'm a software engineer")

// Later conversation - agent remembers
agent.Execute("What did I tell you about my profession?")
```

### Human-in-the-Loop Workflows

```go
approvalTool := aigentic.AgentTool{
    Name:            "send_email",
    Description:     "Sends an email to recipients", 
    RequireApproval: true, // Human approval required
    Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
        return sendEmail(args["recipient"], args["subject"], args["body"])
    },
}

agent := aigentic.Agent{
    Model:      openai.NewModel("gpt-4o-mini", "your-api-key"),
    AgentTools: []aigentic.AgentTool{approvalTool},
}

run, _ := agent.Start("Send a follow-up email to john@example.com")

// Monitor for approval requests
for event := range run.Next() {
    if approvalEvent, ok := event.(*aigentic.ApprovalEvent); ok {
        approved := showApprovalDialog(approvalEvent) // Your UI
        run.Approve(approvalEvent.ApprovalID, approved)
    }
}
```

### Tracing and Logging

```go
agent := aigentic.Agent{
    Model: openai.NewModel("gpt-4o-mini", "your-api-key"),
    Trace: aigentic.NewTrace(),
    LogLevel: slog.LevelDebug,
}

// All interactions are automatically logged to ./traces/
response, _ := agent.Execute("Complex reasoning task")
```

---

## Installation

```bash
go get github.com/nexxia-ai/aigentic
go get github.com/nexxia-ai/aigentic-openai    # For OpenAI support
go get github.com/nexxia-ai/aigentic-ollama    # For Ollama support  
go get github.com/nexxia-ai/aigentic-google    # For Google Gemini support
```

**Note**: This module uses private repositories:
```bash
export GOPRIVATE="github.com/nexxia-ai/**"
```

## Provider Setup

### OpenAI
```go
import openai "github.com/nexxia-ai/aigentic-openai"
model := openai.NewModel("gpt-4o-mini", "your-api-key")
```

### Ollama (Local)
```go  
import ollama "github.com/nexxia-ai/aigentic-ollama"
model := ollama.NewModel("llama3.2:3b", "")
```

### Google Gemini
```go
import gemini "github.com/nexxia-ai/aigentic-google" 
model := gemini.NewGeminiModel("gemini-pro", "your-api-key")
```

---

## Architecture

aigentic uses an event-driven architecture that enables:

- **Real-time UI Updates**: Stream content as it generates
- **Progress Tracking**: Monitor agent execution in detail  
- **Human Interaction**: Pause for approvals and user input
- **Error Handling**: Graceful failure recovery
- **Concurrent Execution**: Parallel agent operations

The framework handles the complexity of agent orchestration while providing a simple, declarative API for developers.

---

## License

This project is licensed under the MIT License.
