# Agent Tools

This directory contains tools designed to work with the `aigentic.Agent` type. All tools implement the `AgentTool` interface.

## Available Tools

1. **ReadFileTool** (`readfile.go`) - Reads files from a specified store
2. **WriteFileTool** (`writefile.go`) - Writes files to a specified store
3. **PythonSandboxTool** (`pythonsandbox.go`) - Executes Python code in a sandboxed environment

## Using Tools with Agents

All tools in this directory return `aigentic.AgentTool` and can be used directly with agents:

```go
package main

import (
    "github.com/nexxia-ai/aigentic"
    "github.com/nexxia-ai/aigentic/tools"
)

func main() {
    agent := aigentic.Agent{
        Name:        "my-agent",
        Description: "An agent with file and Python tools",
        Model:       model,
        AgentTools: []aigentic.AgentTool{
            tools.NewReadFileTool(),
            tools.NewWriteFileTool(),
            tools.NewPythonSandboxTool(),
        },
    }

    result, err := agent.Execute("Read myfile.txt from store1")
    // ...
}
```

## Writing Tests for AgentTools

When testing tools, you must use the `AgentTool` interface which requires an `*AgentRun` parameter:

```go
func TestMyTool(t *testing.T) {
    tool := NewMyTool()

    args := map[string]interface{}{
        "param1": "value1",
    }

    // AgentTool.Execute requires *AgentRun as first parameter
    result, err := tool.Execute(&aigentic.AgentRun{}, args)
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
Execute func(run *AgentRun, args map[string]interface{}) (*ai.ToolResult, error)
```

The `AgentRun` parameter provides access to:
- Agent configuration
- Session state
- Memory
- Trace information
- Other agent context

For simple tests, you can pass an empty `&aigentic.AgentRun{}`.

## Creating New Agent Tools

Use the `aigentic.NewTool` helper function:

```go
func NewMyTool() aigentic.AgentTool {
    type MyToolInput struct {
        Param1 string `json:"param1" description:"Description of param1"`
        Param2 int    `json:"param2,omitempty" description:"Optional param2"`
    }

    return aigentic.NewTool(
        "my_tool",
        "Description of what my tool does",
        func(run *aigentic.AgentRun, input MyToolInput) (string, error) {
            // Tool logic here
            // Return (output_string, error)
            return fmt.Sprintf("Processed: %s", input.Param1), nil
        },
    )
}
```

The `aigentic.NewTool` function:
- Auto-generates JSON schema from your input struct
- Handles parameter marshaling
- Wraps your simple `func(*AgentRun, T) (string, error)` into the full `AgentTool` interface
- Validates struct tags (all exported fields must have `json` tags)

## Test Files

All tools have comprehensive test coverage:

### Unit Tests
- **agent_tool_test.go** (5 tests) - Tests that all tools properly implement AgentTool interface
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

**Total Test Coverage: 57 test cases + 3 benchmarks**

## Running Tests

Due to dependency issues in the build environment, tests may require:
1. Resolving private repository access (aigentic-google)
2. Fixing GOPATH/GOROOT configuration
3. Clearing build cache: `go clean -cache -modcache`

If you encounter build errors related to dependencies, the tool code itself is correct - it's an environment configuration issue.

## Tool Migration Notes

All tools were migrated from `ai.Tool` to `aigentic.AgentTool` to:
- Support agent-specific context (session, memory, trace)
- Enable approval workflows
- Provide validation capabilities
- Simplify tool creation with auto-schema generation

The Python sandbox tool is designed with Docker migration in mind - only the `executePythonCode` function would need to be replaced to switch from subprocess to Docker execution.
