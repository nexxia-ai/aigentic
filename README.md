# aigentic

**Declarative AI agent framework for Go**

aigentic is a production-ready Go framework for building AI-powered applications with sophisticated agent orchestration, designed to address the lack of agentic frameworks in Go that are comparable to Python. Its design was greatly influenced by agno.com.

aigentic provides a declarative solution where you declare agents and their capabilities, then run them with a simple, clean API.

## How It Works

aigentic uses a declarative architecture where you define agent configurations as data structures rather than implementing orchestration logic. You specify an agent's role, capabilities, tools, and instructions, then the framework handles the execution pipeline.

The core architecture is event-driven, with agents emitting events during execution that you can consume for real-time monitoring and control. When you declare an agent, the framework creates an execution context that manages LLM interactions, tool calls, and multi-agent coordination.

The event system provides hooks into the execution lifecycle: content generation, thinking processes, tool executions, and error handling. This allows you to build reactive applications that can respond to execution state changes, implement approval workflows, or provide real-time UI updates.

For multi-agent systems, agents can be composed as tools within other agents, enabling hierarchical delegation patterns. The framework automatically handles the routing and coordination between agents, while maintaining the event stream for monitoring and debugging.

The declarative approach means you focus on agent configuration and event handling rather than implementing conversation management, tool orchestration, or LLM integration details.

---

## Key Features

- **🤖 Simple Agent Creation** - Declarative agent configuration
- **🔄 Streaming Responses** - Real-time content generation with chunked delivery
- **🛠️ Tool Integration** - Custom tools (including MCP) with validation and approval workflows
- **📄 Document Support** - Multi-modal document processing (PDF, images, text)
- **👥 Multi-Agent Teams** - Coordinated agent collaboration
- **💾 Persistent Memory** - Context retention across sessions
- **🔍 Built-in Tracing** - Comprehensive execution monitoring
- **⚡ Event-Driven** - Real-time progress updates and user interaction
- **🔒 Human-in-the-Loop** - Approval workflows for sensitive operations
- **🎯 Provider Agnostic** - Support for OpenAI, Ollama, Google Gemini, and others

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

For interactive applications, streaming UI updates, and human-in-the-loop workflows, use the advanced event system.

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
    ... error checking removed
    
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

To enable approval for tools, simply set the RequireApproval field in the AgentTool. This will generate a ApprovalEvent in the main workflow.

```go
    calculatorTool := aigentic.AgentTool {
        ...,
        RequireApproval: true
    }

    run, err := agent.Start("call the tool with value 43")
    ... error checking removed
    
    // Process real-time events
    for event := range run.Next() {
        switch e := event.(type) {
        case *aigentic.ContentEvent:
            fmt.Print(e.Content) // Stream content as it generates
        case *aigentic.ThinkingEvent:
            fmt.Printf("\n🤔 Thinking: %s\n", e.Thought)
        case *aigentic.ApprovalEvent:
            approved := yourUIApprovalFlow(e)
            run.Approve(e.ID, approved) // true - approved, false - rejected
        case *aigentic.ErrorEvent:
            fmt.Printf("\n❌ Error: %v\n", e.Err)
        }
    }
```

### Document Usage

Native document support. You can choose to embed the document on the prompt or send a reference. Embedding the document works for simple, and smaller documents.


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

If you have several documents, it is best to send the reference only by setting the
DocumentReferences field instead. This way the LLM will decide what to retrieve and when.

```go
agent := aigentic.Agent{
    ...
    DocumentReferences: []*aigentic.Document{doc},
    ...
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

This covers the essential features. The framework includes many more advanced capabilities like MCP integration, sophisticated event handling, and production-ready observability features.

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
        run.Approve(approvalEvent.ID, approved)
    }
}
```

### Tracing and Logging

All interactions are automatically logged if you set the Trace field.
Traces are saved to <tmp_dir>/traces/ 

```go
agent := aigentic.Agent{
    Model: openai.NewModel("gpt-4o-mini", "your-api-key"),
    Trace: aigentic.NewTrace(), // for prompt debugging
    LogLevel: slog.LevelDebug,  // for execution flow debugging 
}

// All interactions are automatically traced
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

## Provider Setup

Choose the model provider and set parameters if need be.

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

### Model Configuration
```go
model = model.WithTemperature(0.7).WithTopP(0.9).WithMaxTokens(2000)
```


