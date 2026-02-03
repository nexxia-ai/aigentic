# aigentic

**Declarative AI agent framework for Go**

aigentic is a production-ready Go framework for building AI-powered applications with sophisticated agent orchestration.

aigentic provides a declarative solution where you declare agents and their capabilities, then run them with a simple, clean API.

## How It Works

aigentic uses a declarative architecture where you define agent configurations as data structures rather than implementing orchestration logic. You specify an agent's role, capabilities, tools, and instructions, then the framework handles the execution pipeline.

The core architecture is event-driven, with agents emitting events during execution that you can consume for real-time monitoring and control. When you declare an agent, the framework creates an execution context that manages LLM interactions, tool calls, file references, and multi-agent coordination.

The event system provides hooks into the execution lifecycle: content generation, thinking processes, tool executions, and error handling. This allows you to build reactive applications that can respond to execution state changes, implement approval workflows, or provide real-time UI updates.

Tools can register files to be automatically included in subsequent prompts using the `ToolCallResult` type. This enables seamless workflows where tools generate documents, reports, or processed files that the agent can immediately reference without explicit loading.

For multi-agent systems, agents can be composed as tools within other agents, enabling hierarchical delegation patterns. The framework automatically handles the routing and coordination between agents, while maintaining the event stream for monitoring and debugging.

The declarative approach means you focus on agent configuration and event handling rather than implementing conversation management, tool orchestration, file handling, or LLM integration details.

---

## Key Features

- **🤖 Simple Agent Creation** - Declarative agent configuration
- **🔄 Streaming Responses** - Real-time content generation with chunked delivery
- **🛠️ Tool Integration** - Custom tools (including MCP) with type-safe schemas
- **📄 Document Support** - Multi-modal document processing (PDF, images, text)
- **👥 Multi-Agent Teams** - Coordinated agent collaboration
- **🔍 Built-in Tracing** - Comprehensive execution monitoring
- **⚡ Event-Driven** - Real-time progress updates and user interaction
- **🎯 Provider Agnostic** - Support for multiple AI providers

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
	"github.com/nexxia-ai/aigentic/ai"
)

func main() {
	model, err := ai.New("GPT-4o Mini", "your-api-key")
	if err != nil {
		log.Fatal(err)
	}

	agent := aigentic.Agent{
		Model:        model,
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
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func main() {
	model, err := ai.New("GPT-4o Mini", "your-api-key")
	if err != nil {
		log.Fatal(err)
	}

	agent := aigentic.Agent{
		Model:        model,
		Name:         "StreamingAssistant",
		Description:  "An assistant that streams responses",
		Instructions: "Provide detailed explanations step by step.",
		Stream:       true,
	}

	run, err := agent.Start("Explain quantum computing in simple terms")
	if err != nil {
		log.Fatal(err)
	}

	for event := range run.Next() {
		switch e := event.(type) {
		case *aigentic.ContentEvent:
			fmt.Print(e.Content)
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

Use `run.NewTool()` for type-safe tools with automatic JSON schema generation. Tools return strings that are automatically converted to the appropriate format:

```go
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
)

func createCalculatorTool() run.AgentTool {
	type CalculatorInput struct {
		Expression string `json:"expression" description:"Mathematical expression to evaluate"`
	}

	return run.NewTool(
		"calculator",
		"Performs mathematical calculations",
		func(run *run.AgentRun, input CalculatorInput) (string, error) {
			return fmt.Sprintf("Result: %s", input.Expression), nil
		},
	)
}

func main() {
	model, err := ai.New("GPT-4o Mini", "your-api-key")
	if err != nil {
		log.Fatal(err)
	}

	agent := aigentic.Agent{
		Model:        model,
		Name:         "MathAssistant",
		Description:  "An assistant that can perform calculations",
		Instructions: "Use the calculator tool for mathematical operations.",
		AgentTools:   []run.AgentTool{createCalculatorTool()},
	}

	response, err := agent.Execute("What is 15 * 23 + 100?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response)
}
```

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/tools@latest
```

#### Advanced Tools with File References

Tools can register files to be automatically included in the next turn's prompt using `ToolCallResult`:

```go
import (
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/ctxt"
	"github.com/nexxia-ai/aigentic/run"
)

func createReportGeneratorTool() run.AgentTool {
	tool := run.AgentTool{
		Name:        "generate_report",
		Description: "Generates a markdown report and makes it available for review",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Report title",
				},
			},
			"required": []string{"title"},
		},
		Execute: func(run *run.AgentRun, args map[string]interface{}) (*run.ToolCallResult, error) {
			title := args["title"].(string)
			
			// Generate report content
			reportContent := fmt.Sprintf("# %s\n\nReport content here...", title)
			
			// Save to file
			ctx := run.AgentContext()
			err := ctx.UploadDocument("output/report.md", []byte(reportContent), true)
			if err != nil {
				return nil, err
			}
			
			// Return result with file reference
			return &run.ToolCallResult{
				Result: &ai.ToolResult{
					Content: []ai.ToolContent{{
						Type:    "text",
						Content: "Report generated successfully at output/report.md",
					}},
				},
				FileRefs: []ctxt.FileRefEntry{
					{Path: "output/report.md", IncludeInPrompt: true},
				},
			}, nil
		},
	}
	return tool
}
```

When `IncludeInPrompt` is `true`, the file content is automatically injected into the next turn's context, allowing the agent to reference it without explicit file reading.

### Document Usage

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/documents)

Native document support with enhanced MIME type detection for text, code, and binary files. You can choose to embed the document on the prompt or send a reference. Embedding the document works for simple, and smaller documents. Documents passed via `Agent.Documents` are stored in the run filesystem under `llm/uploads`.

```go
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
)

func main() {
	doc := document.NewInMemoryDocument("", "report.pdf", pdfData, nil)

	model, err := ai.New("GPT-4o Mini", "your-api-key")
	if err != nil {
		log.Fatal(err)
	}

	agent := aigentic.Agent{
		Model:        model,
		Name:         "DocumentAnalyst",
		Description:  "Analyzes documents and extracts insights",
		Instructions: "Analyze the provided documents and summarize key findings.",
		Documents:    []*document.Document{doc},
	}

	response, err := agent.Execute("What are the main points in this document?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response)
}
```

If you have several documents, pass them via the `Documents` field; they are uploaded to the run filesystem and listed in the prompt for the LLM to reference.

```go
agent := aigentic.Agent{
    ...
    Documents: []*document.Document{doc},
    ...
}
```

The framework automatically detects MIME types for common formats including:
- **Text files**: `.txt`, `.md`, `.csv`, `.log`
- **Code files**: `.go`, `.py`, `.js`, `.ts`, `.java`, `.c`, `.cpp`, etc.
- **Data files**: `.json`, `.yaml`, `.xml`
- **Binary files**: `.pdf`, images, audio, video

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/documents@latest
```

### Multi-Agent Setup

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/multi-agent)

```go
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

func main() {
	model, err := ai.New("GPT-4o Mini", "your-api-key")
	if err != nil {
		log.Fatal(err)
	}

	researchAgent := aigentic.Agent{
		Model:        model,
		Name:         "Researcher",
		Description:  "Expert at gathering and analyzing information",
		Instructions: "Conduct thorough research and provide detailed findings.",
	}

	writerAgent := aigentic.Agent{
		Model:        model,
		Name:         "Writer",
		Description:  "Expert at creating engaging content",
		Instructions: "Write clear, engaging content based on research.",
	}

	coordinator := aigentic.Agent{
		Model:        model,
		Name:         "ProjectManager",
		Description:  "Coordinates research and writing tasks",
		Instructions: "Delegate tasks to specialists and synthesize results.",
		Agents:       []aigentic.Agent{researchAgent, writerAgent},
	}

	response, err := coordinator.Execute("Write an article about renewable energy trends")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response)
}
```

**Run this example:**
```bash
go run github.com/nexxia-ai/aigentic-examples/multi-agent@latest
```

---

## Advanced Features

**→ [Explore all examples](https://github.com/nexxia-ai/aigentic-examples)** for MCP integration, document processing, and production-ready patterns.

### Conversation History

Each `AgentRun` keeps a per-run conversation history inside its execution environment. History is included in prompts by default; set `IncludeHistory` to `false` on your `Agent` to disable it. You can inspect turns after a run finishes:

```go
run, err := agent.Start("Hello!")
if err != nil {
	log.Fatal(err)
}
if _, err := run.Wait(0); err != nil {
	log.Fatal(err)
}

history := run.AgentContext().GetHistory()
for _, turn := range history.GetTurns() {
	fmt.Printf("[%s] user: %s\n", turn.TurnID, turn.UserMessage)
}
```

To clear on-disk history between runs, set `BaseDir` on the agent and delete the generated `agent-<runID>` directory when you are done.

### Event-Driven Architecture

The framework is event-driven, allowing you to react to execution state changes in real-time. All events are emitted during agent execution, including tool calls and file registrations:

```go
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
)

func main() {
	model, err := ai.New("GPT-4o Mini", "your-api-key")
	if err != nil {
		log.Fatal(err)
	}

	agent := aigentic.Agent{
		Model:        model,
		Instructions: "Use tools to help users",
		AgentTools:   []run.AgentTool{createCalculatorTool()},
		Stream:       true,
	}

	run, err := agent.Start("Calculate 15 * 23")
	if err != nil {
		log.Fatal(err)
	}

	for event := range run.Next() {
		switch e := event.(type) {
		case *aigentic.ContentEvent:
			fmt.Print(e.Content)
		case *aigentic.ThinkingEvent:
			fmt.Printf("\n🤔 Thinking: %s\n", e.Thought)
		case *aigentic.ToolEvent:
			fmt.Printf("\n🔧 Tool executed: %s\n", e.ToolName)
		case *aigentic.ErrorEvent:
			fmt.Printf("\n❌ Error: %v\n", e.Err)
		}
	}
}
```

### Tracing and Debugging

All interactions are automatically logged if you set the Tracer field.
Traces are saved to `<tmp_dir>/traces/`

```go
import (
	"log"
	"log/slog"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
)

model, err := ai.New("GPT-4o Mini", "your-api-key")
if err != nil {
	log.Fatal(err)
}

agent := aigentic.Agent{
	Model:    model,
	Tracer:   aigentic.NewTracer(),
	LogLevel: slog.LevelDebug,
}

response, err := agent.Execute("Complex reasoning task")
if err != nil {
	log.Fatal(err)
}
```

### Execution Environment

Each agent run creates an `ExecutionEnvironment` that provides a structured directory layout for agent execution. The environment is automatically created when an agent run starts and provides directories for organizing files and outputs.

The execution environment is automatically created when an agent run starts. If no base directory is specified, it defaults to the system temporary directory.


**Directory Structure:**

The execution environment creates the following directory structure under a base directory:

```
{baseDir}/
  └── agent-{runID}/
      ├── llm/
      │   ├── uploads/  # Documents uploaded for the run
      │   └── output/   # Agent output files and tool-generated content
      └── _private/
          ├── memory/   # Memory files automatically loaded into prompts
          ├── history/  # Conversation history with file references
          └── turns/    # Per-turn execution traces
```

**File References:**

Tools can register files in the `llm/` directory to be included in subsequent turns. When a tool returns `FileRefs` with `IncludeInPrompt: true`, the framework automatically injects the file content into the next prompt, enabling seamless file-based workflows.

### MCP (Model Context Protocol) Integration

aigentic supports MCP servers for tool integration. See the [MCP examples](https://github.com/nexxia-ai/aigentic-examples) for complete integration patterns.

### Built-in Tools

The framework includes built-in tools for common operations:

```go
import (
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
	"github.com/nexxia-ai/aigentic/tools"
)

model, err := ai.New("GPT-4o Mini", "your-api-key")
if err != nil {
	log.Fatal(err)
}

agent := aigentic.Agent{
	Model: model,
	AgentTools: []run.AgentTool{
		tools.NewMemoryTool(),
	},
}
```

Create custom tools using `run.NewTool()` for type-safe tool definitions with automatic schema generation. For advanced use cases, tools can return `*run.ToolCallResult` directly to include file references.

---

## Installation

```bash
go get github.com/nexxia-ai/aigentic
```

## Provider Setup

Providers register themselves with the `ai` registry when imported. Use `ai.New(identifier, apiKey)` to create models.

### OpenAI
```go
import (
	"log"

	"github.com/nexxia-ai/aigentic/ai"
	_ "github.com/nexxia-ai/aigentic-openai"
)

model, err := ai.New("GPT-4o Mini", "your-api-key")
if err != nil {
	log.Fatal(err)
}
```

### Model Configuration
```go
model = model.WithTemperature(0.7).WithTopP(0.9).WithMaxTokens(2000)
```


