# aigentic

**Declarative AI agent framework for Go**

aigentic is a production-ready Go framework for building AI-powered applications with sophisticated agent orchestration.

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
- **💾 Persistent Memory** - Context retention across agent runs
- **🔍 Built-in Tracing** - Comprehensive execution monitoring
- **⚡ Event-Driven** - Real-time progress updates and user interaction
- **🔒 Human-in-the-Loop** - Approval workflows for sensitive operations
- **🎯 Provider Agnostic** - Support for OpenAI, Ollama, Google Gemini, and others

---

## Quick Start

**→ [View Complete Runnable Examples](https://github.com/nexxia-ai/aigentic-examples)** – Learn by doing with real-world use cases

### Simple Agent

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/simple)

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

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/simple@latest
```

### Streaming Agent

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/streaming)

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

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/streaming@latest
```

### Tool Integration

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/tools)

Use `run.NewTool()` for type-safe tools with automatic JSON schema generation:

```go
import "github.com/nexxia-ai/aigentic/run"

func createCalculatorTool() aigentic.AgentTool {
    type CalculatorInput struct {
        Expression string `json:"expression" description:"Mathematical expression to evaluate"`
    }

    return run.NewTool(
        "calculator",
        "Performs mathematical calculations",
        func(run *run.AgentRun, input CalculatorInput) (string, error) {
            result := evaluateExpression(input.Expression)
            return fmt.Sprintf("Result: %v", result), nil
        },
    )
}

func main() {
    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "MathAssistant",
        Description:  "An assistant that can perform calculations",
        Instructions: "Use the calculator tool for mathematical operations.",
        AgentTools:   []aigentic.AgentTool{createCalculatorTool()},
    }

    response, _ := agent.Execute("What is 15 * 23 + 100?")
    fmt.Println(response)
}
```

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/tools@latest
```

### Human-in-the-Loop Approval

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/approval)

To enable approval for tools, create the tool with `run.NewTool()` and set `RequireApproval: true`. This will generate an `ApprovalEvent` in the event stream.

```go
import "github.com/nexxia-ai/aigentic/run"

func createSendEmailTool() aigentic.AgentTool {
    type SendEmailInput struct {
        To      string `json:"to" description:"Email recipient address"`
        Subject string `json:"subject" description:"Email subject line"`
        Body    string `json:"body" description:"Email body content"`
    }

    emailTool := run.NewTool(
        "send_email",
        "Sends an email to a recipient with subject and body. Requires approval before sending.",
        func(run *run.AgentRun, input SendEmailInput) (string, error) {
            return fmt.Sprintf("Email sent to %s", input.To), nil
        },
    )
    emailTool.RequireApproval = true
    return emailTool
}

func main() {
    agent := aigentic.Agent{
        Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:         "EmailAgent",
        Instructions: "Use the send_email tool when asked to send emails.",
        AgentTools:   []aigentic.AgentTool{createSendEmailTool()},
        Stream:       true,
    }

    run, err := agent.Start("Send an email to john@example.com")
    if err != nil {
        log.Fatal(err)
    }
    
    for event := range run.Next() {
        switch e := event.(type) {
        case *aigentic.ContentEvent:
            fmt.Print(e.Content)
        case *aigentic.ApprovalEvent:
            approved := yourUIApprovalFlow(e)
            run.Approve(e.ApprovalID, approved)
        case *aigentic.ErrorEvent:
            fmt.Printf("\n❌ Error: %v\n", e.Err)
        }
    }
}
```

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/approval@latest
```

### Document Usage

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/documents)

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

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/documents@latest
```

### Multi-Agent Setup

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/multi-agent)

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

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/multi-agent@latest
```

---

## Advanced Features

**→ [Explore all examples](https://github.com/nexxia-ai/aigentic-examples)** for MCP integration, memory systems, document processing, and production-ready patterns.

### Memory

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/memory)

Enable memory tools for persistent context across agent runs:

```go
import (
    "github.com/nexxia-ai/aigentic"
    "github.com/nexxia-ai/aigentic/tools"
)

agent := aigentic.Agent{
    Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
    Name:         "PersonalAssistant",
    Description:  "A personal assistant that remembers user preferences",
    Instructions: "Remember user preferences and context using the memory tools.",
    AgentTools:   []aigentic.AgentTool{tools.NewMemoryTool()}, // Enable memory tools
}

// First conversation - agent saves information to memory
agent.Execute("My name is John and I'm a software engineer")

// Later conversation - agent retrieves from memory
agent.Execute("What did I tell you about my profession?")
```

The memory system provides a single tool that the agent can use:
- `update_memory` - Store or update memory entries. Set both description and content to empty strings to delete.

**How Memory Works:**
- Memories persist across multiple agent runs with the same agent instance
- All memories are automatically injected into the system prompt for every LLM call
- The agent can use `update_memory` to store information with an ID, description, and content
- To update an existing memory, call `update_memory` with the same ID
- To delete a memory, call `update_memory` with empty description and content strings
- Memories maintain insertion order and are always available to the LLM in the system prompt

The LLM automatically decides when to use the memory tool based on your instructions.

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/memory@latest
```

### Conversation History

Track full conversation history across multiple `Execute()` or `Start()` calls with automatic message capture and trace correlation.

**Single Agent with Multiple Calls:**

```go
func main() {
    // Create conversation history
    history := aigentic.NewConversationHistory()
    
    agent := aigentic.Agent{
        Model:               openai.NewModel("gpt-4o-mini", "your-api-key"),
        Name:                "Chatbot",
        ConversationHistory: history,  // Enable history tracking
        Tracer:              aigentic.NewTracer(),
    }
    
    // First conversation turn
    agent.Execute("My name is Alice and I love programming")
    
    // Second turn - agent remembers context
    agent.Execute("What's my name?")
    
    // View history with trace files
    for i, entry := range history.GetEntries() {
        fmt.Printf("Turn %d:\n", i+1)
        fmt.Printf("  User: %v\n", entry.UserMessage)
        fmt.Printf("  Assistant: %v\n", entry.AssistantMessage)
        fmt.Printf("  Trace: %s\n", entry.TraceFile)
        fmt.Printf("  RunID: %s\n", entry.RunID)
    }
}
```

**Multiple Agents Sharing History:**

```go
func main() {
    sharedHistory := aigentic.NewConversationHistory()
    
    // Multiple agents share the same history
    greeter := aigentic.Agent{
        Model:               openai.NewModel("gpt-4o-mini", "your-api-key"),
        ConversationHistory: sharedHistory,
    }
    
    assistant := aigentic.Agent{
        Model:               openai.NewModel("gpt-4o-mini", "your-api-key"),
        ConversationHistory: sharedHistory,  // Same history
    }
    
    // Greeter interacts
    greeter.Execute("Hello!")
    
    // Assistant sees full conversation context
    assistant.Execute("What did we talk about?")
}
```

**History Management:**

```go
// Clear all history
history.Clear()

// Remove specific entry
history.RemoveAt(0)

// Keep only recent messages (sliding window)
entries := history.GetEntries()
history.SetEntries(entries[len(entries)-10:])

// Find entries by trace file
entries := history.FindByTraceFile("trace-20251102042420.001.txt")

// Find entries by run ID
entries := history.FindByRunID("a1b2c3d4-...")
```

**HistoryEntry Structure:**

Each conversation turn stores:
- `UserMessage` - The user's input message
- `AssistantMessage` - The AI's response
- `ToolMessages` - Any tool call/response messages in this turn
- `TraceFile` - Path to trace file for debugging
- `RunID` - The agent run ID that produced this turn
- `Timestamp` - When the turn started
- `AgentName` - Which agent handled this turn

### Event-Driven Architecture

The framework is event-driven, allowing you to react to execution state changes in real-time:

```go
agent := aigentic.Agent{
    Model:        openai.NewModel("gpt-4o-mini", "your-api-key"),
    Instructions: "Use tools to help users",
    AgentTools:   []aigentic.AgentTool{createCalculatorTool()},
    Stream:       true,
}

run, _ := agent.Start("Calculate 15 * 23")

for event := range run.Next() {
    switch e := event.(type) {
    case *aigentic.ContentEvent:
        fmt.Print(e.Content) // Stream content as it generates
    case *aigentic.ThinkingEvent:
        fmt.Printf("\n🤔 Thinking: %s\n", e.Thought)
    case *aigentic.ToolEvent:
        fmt.Printf("\n🔧 Tool executed: %s\n", e.ToolName)
    case *aigentic.ApprovalEvent:
        // Handle approval requests
        run.Approve(e.ApprovalID, true)
    case *aigentic.ErrorEvent:
        fmt.Printf("\n❌ Error: %v\n", e.Err)
    }
}
```

### Tracing and Debugging

All interactions are automatically logged if you set the Tracer field.
Traces are saved to `<tmp_dir>/traces/`

```go
import "log/slog"

agent := aigentic.Agent{
    Model:    openai.NewModel("gpt-4o-mini", "your-api-key"),
    Tracer:   aigentic.NewTracer(),     // Enable tracing for prompt debugging
    LogLevel: slog.LevelDebug,          // Enable debug logging for execution flow
}

// All interactions are automatically traced
response, _ := agent.Execute("Complex reasoning task")
```

### Execution Environment

Each agent run creates an `ExecutionEnvironment` that provides a structured directory layout for agent execution. The environment is automatically created when an agent run starts and provides directories for organizing files, session data, and outputs.

The execution environment is automatically created when an agent run starts. If no base directory is specified, it defaults to the system temporary directory.


**Directory Structure:**

The execution environment creates the following directory structure under a base directory:

```
{baseDir}/
  └── agent-{runID}/
      ├── session/    # Session files automatically loaded into prompts
      ├── files/      # General file storage
      └── output/     # Agent output files
```

### MCP (Model Context Protocol) Integration

aigentic supports MCP servers for tool integration. See the [MCP examples](https://github.com/nexxia-ai/aigentic-examples) for complete integration patterns.

### Built-in Tools

The framework includes built-in tools for common operations:

```go
import "github.com/nexxia-ai/aigentic/tools"

agent := aigentic.Agent{
    Model:      openai.NewModel("gpt-4o-mini", "your-api-key"),
    AgentTools: []aigentic.AgentTool{
        tools.NewMemoryTool(),        // Memory operations
        tools.NewReadFileTool(),      // Read files from filesystem
        tools.NewWriteFileTool(),     // Write files to filesystem
    },
}
```

Create custom tools using `run.NewTool()` for type-safe tool definitions with automatic schema generation.

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


