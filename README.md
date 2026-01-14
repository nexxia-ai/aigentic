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
- **🛠️ Tool Integration** - Custom tools (including MCP) with type-safe schemas
- **📄 Document Support** - Multi-modal document processing (PDF, images, text)
- **👥 Multi-Agent Teams** - Coordinated agent collaboration
- **💾 Persistent Memory** - Context retention across agent runs
- **🔍 Built-in Tracing** - Comprehensive execution monitoring
- **⚡ Event-Driven** - Real-time progress updates and user interaction
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
	"github.com/nexxia-ai/aigentic/ai"
	_ "github.com/nexxia-ai/aigentic-openai"
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
	_ "github.com/nexxia-ai/aigentic-openai"
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

Use `run.NewTool()` for type-safe tools with automatic JSON schema generation:

```go
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
	_ "github.com/nexxia-ai/aigentic-openai"
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

### Document Usage

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/documents)

Native document support. You can choose to embed the document on the prompt or send a reference. Embedding the document works for simple, and smaller documents.


```go
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/document"
	_ "github.com/nexxia-ai/aigentic-openai"
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

If you have several documents, it is best to send the reference only by setting the
DocumentReferences field instead. This way the LLM will decide what to retrieve and when.

```go
agent := aigentic.Agent{
    ...
    DocumentReferences: []*document.Document{doc},
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
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	_ "github.com/nexxia-ai/aigentic-openai"
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

**→ [Explore all examples](https://github.com/nexxia-ai/aigentic-examples)** for MCP integration, memory systems, document processing, and production-ready patterns.

### Memory

[📖 See full example](https://github.com/nexxia-ai/aigentic-examples/tree/main/memory)

Enable memory tools for persistent context across agent runs:

```go
import (
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
	"github.com/nexxia-ai/aigentic/tools"
	_ "github.com/nexxia-ai/aigentic-openai"
)

func main() {
	model, err := ai.New("GPT-4o Mini", "your-api-key")
	if err != nil {
		log.Fatal(err)
	}

	agent := aigentic.Agent{
		Model:        model,
		Name:         "PersonalAssistant",
		Description:  "A personal assistant that remembers user preferences",
		Instructions: "Remember user preferences and context using the memory tools.",
		AgentTools:   []run.AgentTool{tools.NewMemoryTool()},
	}

	agent.Execute("My name is John and I'm a software engineer")
	agent.Execute("What did I tell you about my profession?")
}
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

The framework is event-driven, allowing you to react to execution state changes in real-time:

```go
package main

import (
	"fmt"
	"log"

	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/run"
	_ "github.com/nexxia-ai/aigentic-openai"
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
	_ "github.com/nexxia-ai/aigentic-openai"
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

Each agent run creates an `ExecutionEnvironment` that provides a structured directory layout for agent execution. The environment is automatically created when an agent run starts and provides directories for organizing files, memory data, and outputs.

The execution environment is automatically created when an agent run starts. If no base directory is specified, it defaults to the system temporary directory.


**Directory Structure:**

The execution environment creates the following directory structure under a base directory:

```
{baseDir}/
  └── agent-{runID}/
      ├── memory/     # Memory files automatically loaded into prompts
      ├── files/      # General file storage
      └── output/     # Agent output files
```

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
	_ "github.com/nexxia-ai/aigentic-openai"
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

Create custom tools using `run.NewTool()` for type-safe tool definitions with automatic schema generation.

---

## Installation

```bash
go get github.com/nexxia-ai/aigentic
go get github.com/nexxia-ai/aigentic-openai
go get github.com/nexxia-ai/aigentic-ollama
go get github.com/nexxia-ai/aigentic-google
```

## Provider Setup

Providers register themselves with the `ai` registry when imported (a blank import is fine). Use `ai.Models()` to inspect available identifiers, then create a model with `ai.New(identifier, apiKey)`.

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

### Ollama (Local)
```go
import (
	"log"

	"github.com/nexxia-ai/aigentic/ai"
	_ "github.com/nexxia-ai/aigentic-ollama"
)

model, err := ai.New("Qwen3 4B", "")
if err != nil {
	log.Fatal(err)
}
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


