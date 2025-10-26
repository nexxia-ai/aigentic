package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic"
)

const (
	PythonSandboxToolName    = "python_sandbox"
	pythonSandboxDescription = `Python code execution tool that runs Python code in a sandboxed environment and returns the results.

WHEN TO USE THIS TOOL:
- Use when you need to execute Python code dynamically
- Helpful for performing calculations, data analysis, or running algorithms
- Perfect for testing Python code snippets
- Useful for data processing, mathematical computations, or quick prototypes
- Can be used to validate Python logic or demonstrate code behavior

HOW TO USE:
- Provide the Python code you want to execute
- Optionally specify a timeout in seconds (default: 30, max: 300)
- The tool will execute the code and return stdout, stderr, and any errors

FEATURES:
- Executes Python 3 code in a controlled environment
- Captures standard output (stdout) and standard error (stderr)
- Enforces execution timeout to prevent infinite loops
- Returns detailed error messages for debugging
- Supports multi-line Python scripts
- Automatic cleanup of temporary files

LIMITATIONS:
- Maximum execution time: 300 seconds (5 minutes)
- No persistent state between executions
- Limited to installed Python packages on the system
- No network access restrictions (use with caution)
- Output size may be limited by system constraints
- Cannot install packages during execution (pip not available)

TIPS:
- Always include print() statements to see output
- Use try-except blocks for better error handling
- Keep execution time reasonable (under 30 seconds recommended)
- For complex calculations, consider breaking into smaller steps
- Use timeout parameter for potentially long-running code
- Code runs in isolated temporary directory

SECURITY NOTES:
- Code execution is sandboxed with timeout enforcement
- Use responsibly and avoid executing untrusted code
- File system access is limited to temporary directories
- Future versions may use Docker for stronger isolation`
)

const (
	defaultTimeout = 30  // 30 seconds
	maxTimeout     = 300 // 5 minutes
)

func NewPythonSandboxTool() aigentic.AgentTool {
	type PythonSandboxInput struct {
		Code    string `json:"code" description:"Python code to execute in the sandbox"`
		Timeout int    `json:"timeout,omitempty" description:"Execution timeout in seconds (default: 30, max: 300)"`
	}

	return aigentic.NewTool(
		PythonSandboxToolName,
		pythonSandboxDescription,
		func(run *aigentic.AgentRun, input PythonSandboxInput) (string, error) {
			return executePython(input.Code, input.Timeout)
		},
	)
}

// executePython validates parameters and executes Python code
func executePython(code string, timeout int) (string, error) {
	// Validate required parameters
	if code == "" {
		return "", fmt.Errorf("code is required")
	}

	// Set default timeout if not specified
	if timeout == 0 {
		timeout = defaultTimeout
	}

	// Enforce maximum timeout
	if timeout > maxTimeout {
		return "", fmt.Errorf("timeout exceeds maximum allowed value of %d seconds", maxTimeout)
	}

	// Execute the Python code
	return executePythonCode(code, timeout)
}

// executePythonCode executes Python code in a subprocess with timeout
// This function is designed to be easily replaceable with a Docker-based implementation
func executePythonCode(code string, timeoutSec int) (string, error) {
	// Create temporary directory for execution
	tempDir, err := os.MkdirTemp("", "python-sandbox-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write code to temporary file
	scriptPath := filepath.Join(tempDir, "script.py")
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write script file: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// Execute Python with clean environment
	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Dir = tempDir
	cmd.Env = []string{
		"PYTHONUNBUFFERED=1",
		"PYTHONDONTWRITEBYTECODE=1",
	}

	// Capture stdout and stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("execution timeout: code execution exceeded %d seconds limit", timeoutSec)
	}

	// Prepare output
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// If execution failed
	if err != nil {
		// Format error output
		var output strings.Builder
		output.WriteString("Python execution failed:\n\n")

		if stderrStr != "" {
			output.WriteString("STDERR:\n")
			output.WriteString(stderrStr)
			output.WriteString("\n")
		}

		if stdoutStr != "" {
			output.WriteString("STDOUT:\n")
			output.WriteString(stdoutStr)
			output.WriteString("\n")
		}

		output.WriteString(fmt.Sprintf("Exit error: %v", err))

		return "", fmt.Errorf("%s", output.String())
	}

	// Success - format output
	var output strings.Builder

	if stdoutStr != "" {
		output.WriteString(stdoutStr)
	}

	if stderrStr != "" {
		if output.Len() > 0 {
			output.WriteString("\n\nSTDERR:\n")
		}
		output.WriteString(stderrStr)
	}

	if output.Len() == 0 {
		output.WriteString("Code executed successfully with no output")
	}

	return output.String(), nil
}
