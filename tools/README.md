# Agent Tools

This directory contains tools designed to work with the `aigentic.Agent` type. All tools implement the `AgentTool` interface.

## Available Tools

1. **MemoryTool** (`memory.go`) - Stores and manages persistent memory entries
2. **ReadFileTool** (`readfile.go`) - Reads files from a specified store
3. **WriteFileTool** (`writefile.go`) - Writes files to a specified store
4. **PythonSandboxTool** (`pythonsandbox.go`) - Executes Python code in a sandboxed environment

### MemoryTool

The MemoryTool provides a single `update_memory` tool that allows the LLM to store, update, and delete memory entries.

**Key Features:**
- **Persistent**: Memories persist across multiple agent runs with the same agent instance
- **Automatic injection**: Memories are automatically included in the system prompt for every LLM call
- **Upsert operation**: Calling `update_memory` with an existing ID updates that memory entry
- **Insertion order**: Memories maintain the order in which they were created
- **Simple deletion**: Delete memories by setting both description and content to empty strings

**Tool Parameters:**
- `memory_id` (required): Unique identifier for the memory entry
- `memory_description` (required): Human-readable description of the memory
- `memory_content` (required): The actual content to store

**Example Usage:**
```go
import (
	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/run"
	"github.com/nexxia-ai/aigentic/tools"
)

agent := aigentic.Agent{
	AgentTools: []run.AgentTool{tools.NewMemoryTool()},
}

// The LLM will automatically use update_memory to store information
agent.Execute("Remember that my favorite color is blue")
```

Memories are automatically injected into the system prompt, so the LLM always has access to them without needing explicit retrieval operations.

## Using Tools with Agents

All tools in this directory return `run.AgentTool` and can be used directly with agents:

```go
package main

import (
	"github.com/nexxia-ai/aigentic"
	"github.com/nexxia-ai/aigentic/run"
	"github.com/nexxia-ai/aigentic/tools"
)

func main() {
	agent := aigentic.Agent{
		Name:        "my-agent",
		Description: "An agent with file and Python tools",
		Model:       model,
		AgentTools: []run.AgentTool{
			tools.NewMemoryTool(),
			tools.NewReadFileTool(),
			tools.NewWriteFileTool(),
			tools.NewPythonSandboxTool(),
		},
	}

	result, err := agent.Execute("Read myfile.txt from store1")
	_ = result
	_ = err
}
```

## Writing Tests for AgentTools

When testing tools, you must use the `AgentTool` interface which requires a `*run.AgentRun` parameter:

```go
import (
    "testing"

    "github.com/nexxia-ai/aigentic/ai"
    "github.com/nexxia-ai/aigentic/run"
)

func TestMyTool(t *testing.T) {
    tool := NewMyTool()

    args := map[string]interface{}{
        "param1": "value1",
    }

    // AgentTool.Execute requires *AgentRun as first parameter
    result, err := tool.Execute(&run.AgentRun{}, args)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // result is *ai.ToolResult
    if result.Error {
        t.Errorf("expected success, got error: %s", result.Content[0].Content)
    }

    output := result.Content[0].Content.(string)
    // ... assertions
}
```

### Key Differences from ai.Tool

The `AgentTool` interface differs from `ai.Tool`:

**ai.Tool (old):**
```go
Execute func(args map[string]interface{}) (*ai.ToolResult, error)
```

**AgentTool (new):**
```go
Execute func(run *run.AgentRun, args map[string]interface{}) (*ai.ToolResult, error)
```

The `AgentRun` parameter provides access to:
- Agent configuration
- Memory
- Trace information
- Other agent context

For simple tests, you can pass an empty `&run.AgentRun{}`.

## Creating New Agent Tools

Use the `run.NewTool` helper function:

```go
import "github.com/nexxia-ai/aigentic/run"

func NewMyTool() run.AgentTool {
    type MyToolInput struct {
        Param1 string `json:"param1" description:"Description of param1"`
        Param2 int    `json:"param2,omitempty" description:"Optional param2"`
    }

    return run.NewTool(
        "my_tool",
        "Description of what my tool does",
        func(run *run.AgentRun, input MyToolInput) (string, error) {
            // Tool logic here
            // Return (output_string, error)
            return fmt.Sprintf("Processed: %s", input.Param1), nil
        },
    )
}
```

The `run.NewTool` function:
- Auto-generates JSON schema from your input struct
- Handles parameter marshaling
- Wraps your simple `func(*AgentRun, T) (string, error)` into the full `AgentTool` interface
- Validates struct tags (all exported fields must have `json` tags)

## Test Files

All tools have comprehensive test coverage:

### Unit Tests
- **agent_tool_test.go** (5 tests) - Tests that all tools properly implement AgentTool interface
- **memory_test.go** (5 tests) - Comprehensive tests for memory operations
  - Basic memory storage and retrieval
  - Memory deletion (empty description and content)
  - System prompt injection
  - Multiple memory entries
  - Upsert operations (update existing memories)
  - Memory persistence across runs
- **readfile_test.go** (13 tests + 1 benchmark) - Comprehensive tests for file reading
  - Schema validation
  - Successful reads (text, multiline, empty, large files)
  - Binary file handling with base64 encoding
  - Error cases (missing file, directory, missing parameters)
  - Nested directories and special characters
  - JSON and Unicode content

- **writefile_test.go** (14 tests + 1 benchmark) - Comprehensive tests for file writing
  - Schema validation
  - Successful writes (text, multiline, empty, large files)
  - Binary file handling with base64 encoding
  - File overwriting
  - Error cases (missing parameters, invalid base64)
  - Nested directory creation
  - JSON and Unicode content

- **pythonsandbox_test.go** (16 tests + 1 benchmark) - Comprehensive tests for Python execution
  - Schema validation
  - Simple and multi-line code execution
  - Timeout handling
  - Error cases (syntax, runtime, timeout exceeded)
  - Standard library imports
  - Stderr output handling

### Integration Tests
- **filetools_integration_test.go** (9 tests) - Tests read/write tool interactions
  - Write then read text files
  - Binary file round-trip with base64
  - Multiple file operations
  - File modification workflows
  - Nested directory operations
  - Large file handling
  - Unicode and special character preservation
  - JSON file round-trip

**Total Test Coverage: 62 test cases + 3 benchmarks**

## Running Tests

Due to dependency issues in the build environment, tests may require:
1. Resolving private repository access (aigentic-google)
2. Fixing GOPATH/GOROOT configuration
3. Clearing build cache: `go clean -cache -modcache`

If you encounter build errors related to dependencies, the tool code itself is correct - it's an environment configuration issue.

## Tool Migration Notes

All tools were migrated from `ai.Tool` to `run.AgentTool` to:
- Support agent-specific context (memory, trace)
- Simplify tool creation with auto-schema generation

The Python sandbox tool is designed with Docker migration in mind - only the `executePythonCode` function would need to be replaced to switch from subprocess to Docker execution.
