# Aigentic Framework Features

The aigentic package is a powerful Go framework for building AI-powered applications with sophisticated agent orchestration, multi-modal document processing, and real-time event handling.

## 🚀 Quick Start - Simple API

The framework provides two main interaction patterns to suit different needs:

### 1. Simple Synchronous API

For straightforward use cases, use `RunAndWait()` to get the final result:

```go
package main

import (
    "log"
    "github.com/nexxia-ai/aigentic"
    openai "github.com/nexxia-ai/aigentic-openai"
)

func main() {
    // Create a simple agent
    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "Assistant",
        Description:  "A helpful AI assistant",
        Instructions: "You are a friendly and knowledgeable assistant.",
    }
    
    // Simple synchronous call
    response, err := agent.Execute("What is the capital of France?")
    if err != nil {
        log.Fatal(err)
    }
    
    println("Response:", response)
}
```

### 2. Real-time Event Stream API

For interactive applications, UI updates, and human-in-the-loop workflows, use the event system:

```go
func main() {
    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "Assistant",
        Description:  "A helpful AI assistant",
        Instructions: "You are a friendly assistant that thinks step by step.",
    }
    
    // Start the agent asynchronously
    run, err := agent.Start("Explain quantum computing in simple terms")
    if err != nil {
        log.Fatal(err)
    }
    
    // Process events in real-time
    for event := range run.Next() {
        switch e := event.(type) {
        case *aigentic.ThinkingEvent:
            // Show the AI's reasoning process
            fmt.Printf("🤔 Thinking: %s\n", e.Thought)
            
        case *aigentic.ContentEvent:
            if e.IsChunk {
                // Stream content as it's generated (like ChatGPT typing)
                fmt.Print(e.Content)
            } else {
                // Final content piece
                fmt.Printf("📝 Final: %s\n", e.Content)
            }
            
        case *aigentic.ToolEvent:
            // Show when tools are being called
            fmt.Printf("🔧 Using tool: %s\n", e.ToolName)
            
        case *aigentic.ToolResponseEvent:
            // Show tool results
            fmt.Printf("✅ Tool result: %s\n", e.Content)
            
        case *aigentic.ErrorEvent:
            // Handle errors gracefully
            fmt.Printf("❌ Error: %v\n", e.Err)
        }
    }
}
```

## 🎯 Key Features

### Multi-Provider LLM Support

Work seamlessly with different AI providers:

```go
// OpenAI
openaiModel := openai.NewModel("gpt-4", "api-key")

// Local Ollama
ollamaModel := ollama.NewModel("llama3:8b", "")

// Google Gemini
geminiModel := google.NewModel("gemini-pro", "api-key")

agent := aigentic.Agent{
    Model: openaiModel, // Switch providers easily
    // ... other config
}
```

### Multi-Agent Orchestration

Create teams of specialized agents that work together:

```go
// Specialized research agent
researchAgent := &aigentic.Agent{
    Model:        model,
    Name:         "Researcher",
    Description:  "Expert at gathering and analyzing information",
    Instructions: "Conduct thorough research and provide detailed findings.",
}

// Writing specialist agent
writerAgent := &aigentic.Agent{
    Model:        model,
    Name:         "Writer",
    Description:  "Expert at creating engaging content",
    Instructions: "Write clear, engaging articles based on research.",
}

// Coordinator agent that manages the team
coordinator := aigentic.Agent{
    Model:        model,
    Name:         "ProjectManager",
    Description:  "Coordinates research and writing tasks",
    Instructions: "Delegate tasks to specialists and synthesize results.",
    
    // Add team members - they become available as tools
    Agents: []*aigentic.Agent{researchAgent, writerAgent},
}

// The coordinator can now call team members automatically
response, _ := coordinator.Execute("Write an article about renewable energy trends")
```

### Document Processing & Multi-Modal Support

Handle various document types and media:

```go
agent := aigentic.Agent{
    Model:        model,
    Name:         "DocumentAnalyst",
    Instructions: "Analyze documents and extract key insights.",
    
    // Attach documents directly
    Documents: []*aigentic.Document{
        aigentic.NewInMemoryDocument("", "report.pdf", pdfData, nil),
        aigentic.NewInMemoryDocument("", "chart.png", imageData, nil),
    },
    
    // Reference external files
    DocumentReferences: []*aigentic.Document{
        &aigentic.Document{FilePath: "/path/to/large-dataset.csv"},
    },
}

response, _ := agent.Execute("Summarize the key findings from these documents")
```

### Intelligent Tool System

Extend agent capabilities with custom tools:

```go
// Create a custom tool
calculatorTool := ai.Tool{
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
    Execute: func(args map[string]interface{}) (*ai.ToolResult, error) {
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
    Model:        model,
    Tools:        []ai.Tool{calculatorTool},
    Instructions: "You can perform calculations using the calculator tool.",
}
```

### Persistent Memory System

Agents remember context across interactions:

```go
session := aigentic.NewSession()

agent := aigentic.Agent{
    Model:   model,
    Session: session, // Shared memory across conversations
    Name:    "PersonalAssistant",
}

// First conversation
agent.Execute("My name is John and I'm a software engineer")

// Later conversation - agent remembers John
agent.Execute("What did I tell you about my profession?")
// Response: "You mentioned that you're a software engineer, John."
```

### Human-in-the-Loop Workflows

Enable approval workflows for sensitive operations:

```go
// Tool that requires human approval
approvalTool := aigentic.AgentTool{
    Name:            "send_email",
    Description:     "Sends an email to recipients",
    RequireApproval: true, // Human approval required
    Execute: func(run *aigentic.AgentRun, args map[string]interface{}) (*ai.ToolResult, error) {
        // This will trigger an approval request
        return sendEmail(args["recipient"], args["subject"], args["body"])
    },
}

agent := aigentic.Agent{
    Model:      model,
    AgentTools: []aigentic.AgentTool{approvalTool},
}

run, _ := agent.Start("Send a follow-up email to john@example.com")

// Monitor for approval requests
for event := range run.Next() {
    if toolEvent, ok := event.(*aigentic.ToolEvent); ok {
        if toolEvent.RequiresApproval {
            // Show approval UI to user
            approved := showApprovalDialog(toolEvent)
            run.Approve(toolEvent.EventID, approved)
        }
    }
}
```

### Advanced Configuration

Fine-tune model behavior:

```go
agent := aigentic.Agent{
    Model: openai.NewModel("gpt-4", "api-key").
        WithTemperature(0.7).
        WithMaxTokens(2000).
        WithTopP(0.9),
    
    // Retry configuration
    Retries:             3,
    DelayBetweenRetries: 1000, // milliseconds
    ExponentialBackoff:  true,
    
    // Streaming for real-time responses
    Stream: true,
    
    // Limit LLM calls to prevent runaway costs
    MaxLLMCalls: 10,
}
```

### Built-in Tracing & Observability

Monitor and debug agent behavior:

```go
trace := aigentic.NewTrace(aigentic.TraceConfig{
    SessionID: "session-123",
    Directory: "./traces",
})

agent := aigentic.Agent{
    Model: model,
    Trace: trace, // Automatic execution tracing
}

// All interactions are automatically logged
response, _ := agent.Execute("Complex reasoning task")

// Traces are saved to ./traces/ with full conversation history
```

## 🔄 Why Event-Driven Architecture?

The event system is designed to solve real-world application needs:

### Real-time UI Updates
```go
// Update your UI as the AI generates content
for event := range run.Next() {
    switch e := event.(type) {
    case *aigentic.ContentEvent:
        if e.IsChunk {
            // Stream to UI like ChatGPT
            ui.AppendText(e.Content)
        }
    case *aigentic.ThinkingEvent:
        // Show AI reasoning
        ui.ShowThinking(e.Thought)
    }
}
```

### Human Approval Workflows
```go
// Pause execution for human input
for event := range run.Next() {
    if toolEvent, ok := event.(*aigentic.ToolEvent); ok {
        approved := ui.ShowApprovalDialog(toolEvent)
        run.Approve(toolEvent.EventID, approved)
    }
}
```

### Progress Tracking
```go
// Show detailed progress to users
for event := range run.Next() {
    switch e := event.(type) {
    case *aigentic.LLMCallEvent:
        ui.ShowStatus("Thinking...")
    case *aigentic.ToolEvent:
        ui.ShowStatus(fmt.Sprintf("Using %s...", e.ToolName))
    case *aigentic.ContentEvent:
        ui.ShowStatus("Generating response...")
    }
}
```

## 🛠 MCP (Model Context Protocol) Integration

Connect to external tools and data sources:

```go
// Load MCP configuration
mcpHost, err := ai.NewMCPHostFromConfig("./mcp-config.json")
if err != nil {
    log.Fatal(err)
}

agent := aigentic.Agent{
    Model: model,
}

// Convert MCP tools to AgentTools
for _, tool := range mcpHost.Tools {
    agent.AgentTools = append(agent.AgentTools, aigentic.WrapTool(tool))
}

// Now agent can use file systems, databases, APIs, etc. through MCP
```

## 📈 Production Ready Features

- **Automatic retry logic** with exponential backoff
- **Rate limiting** and cost controls
- **Error recovery** and graceful degradation  
- **Comprehensive logging** and tracing
- **Memory management** with automatic cleanup
- **Concurrent execution** with goroutine pools
- **Type safety** with Go's strong typing system

## 🎯 Use Cases

- **Chatbots & Assistants**: Real-time streaming responses
- **Document Processing**: Multi-modal analysis with progress tracking
- **Workflow Automation**: Human-in-the-loop approvals
- **Research Tools**: Multi-agent collaboration 
- **Content Generation**: Step-by-step creation process
- **Data Analysis**: Tool-enhanced investigation

The aigentic framework gives you the flexibility to build simple scripts or sophisticated AI applications with the same clean API.
