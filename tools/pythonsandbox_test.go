package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic"
)

func TestNewPythonSandboxTool(t *testing.T) {
	tool := NewPythonSandboxTool()

	if tool.Name != PythonSandboxToolName {
		t.Errorf("expected name '%s', got %s", PythonSandboxToolName, tool.Name)
	}

	if tool.Description == "" {
		t.Error("expected non-empty description")
	}

	// Check schema generation
	schema := tool.InputSchema
	if schema["type"] != "object" {
		t.Errorf("expected type 'object', got %v", schema["type"])
	}

	props := schema["properties"].(map[string]interface{})
	if props["code"] == nil {
		t.Error("expected 'code' property in schema")
	}

	codeProp := props["code"].(map[string]interface{})
	if codeProp["type"] != "string" {
		t.Errorf("expected code type 'string', got %v", codeProp["type"])
	}

	// Check required fields
	required := schema["required"].([]string)
	hasCode := false
	for _, r := range required {
		if r == "code" {
			hasCode = true
		}
	}
	if !hasCode {
		t.Error("code should be required")
	}

	// Timeout should be optional
	for _, r := range required {
		if r == "timeout" {
			t.Error("timeout should not be required (omitempty)")
		}
	}
}

func TestPythonSandbox_SimpleExecution(t *testing.T) {
	tool := NewPythonSandboxTool()

	args := map[string]interface{}{
		"code": "print('Hello, World!')",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "Hello, World!") {
		t.Errorf("expected output to contain 'Hello, World!', got: %s", output)
	}
}

func TestPythonSandbox_MultiLineCode(t *testing.T) {
	tool := NewPythonSandboxTool()

	code := `
x = 10
y = 20
result = x + y
print(f"Result: {result}")
`

	args := map[string]interface{}{
		"code": code,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "Result: 30") {
		t.Errorf("expected output to contain 'Result: 30', got: %s", output)
	}
}

func TestPythonSandbox_WithCustomTimeout(t *testing.T) {
	tool := NewPythonSandboxTool()

	args := map[string]interface{}{
		"code":    "import time; print('Starting'); print('Done')",
		"timeout": 5,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "Starting") {
		t.Errorf("expected output to contain 'Starting', got: %s", output)
	}
}

func TestPythonSandbox_SyntaxError(t *testing.T) {
	tool := NewPythonSandboxTool()

	args := map[string]interface{}{
		"code": "print('missing closing quote",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Error {
		t.Error("expected error for syntax error")
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "Python execution failed") {
		t.Errorf("expected error message, got: %s", output)
	}
}

func TestPythonSandbox_RuntimeError(t *testing.T) {
	tool := NewPythonSandboxTool()

	args := map[string]interface{}{
		"code": "x = 1 / 0",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Error {
		t.Error("expected error for runtime error")
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "ZeroDivisionError") && !strings.Contains(output, "Python execution failed") {
		t.Errorf("expected division error message, got: %s", output)
	}
}

func TestPythonSandbox_Timeout(t *testing.T) {
	tool := NewPythonSandboxTool()

	// Code that runs longer than timeout
	args := map[string]interface{}{
		"code":    "import time; time.sleep(5)",
		"timeout": 1,
	}

	start := time.Now()
	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Error {
		t.Error("expected timeout error")
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "timeout") && !strings.Contains(output, "Timeout") {
		t.Errorf("expected timeout message, got: %s", output)
	}

	// Should complete in approximately timeout duration (with some tolerance)
	if elapsed > 3*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestPythonSandbox_NoOutput(t *testing.T) {
	tool := NewPythonSandboxTool()

	args := map[string]interface{}{
		"code": "x = 1 + 1",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "successfully") {
		t.Errorf("expected success message for no output, got: %s", output)
	}
}

func TestPythonSandbox_EmptyCode(t *testing.T) {
	tool := NewPythonSandboxTool()

	args := map[string]interface{}{
		"code": "",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Error {
		t.Error("expected error for empty code")
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "required") {
		t.Errorf("expected 'required' in error message, got: %s", output)
	}
}

func TestPythonSandbox_MaxTimeoutEnforcement(t *testing.T) {
	tool := NewPythonSandboxTool()

	args := map[string]interface{}{
		"code":    "print('test')",
		"timeout": 999, // Exceeds maximum
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Error {
		t.Error("expected error for timeout exceeding maximum")
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "maximum") || !strings.Contains(output, "300") {
		t.Errorf("expected maximum timeout error message, got: %s", output)
	}
}

func TestPythonSandbox_DefaultTimeout(t *testing.T) {
	tool := NewPythonSandboxTool()

	// Don't specify timeout, should use default
	args := map[string]interface{}{
		"code": "print('Using default timeout')",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success with default timeout, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "Using default timeout") {
		t.Errorf("expected output, got: %s", output)
	}
}

func TestPythonSandbox_StderrOutput(t *testing.T) {
	tool := NewPythonSandboxTool()

	// Code that writes to stderr but doesn't fail
	args := map[string]interface{}{
		"code": "import sys; print('stdout message'); sys.stderr.write('stderr message\\n')",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "stdout message") {
		t.Errorf("expected stdout in output, got: %s", output)
	}
	if !strings.Contains(output, "stderr message") {
		t.Errorf("expected stderr in output, got: %s", output)
	}
}

func TestPythonSandbox_ComplexCalculation(t *testing.T) {
	tool := NewPythonSandboxTool()

	code := `
import math

def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)

result = fibonacci(10)
print(f"Fibonacci(10) = {result}")
`

	args := map[string]interface{}{
		"code": code,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "55") {
		t.Errorf("expected Fibonacci result '55', got: %s", output)
	}
}

func TestPythonSandbox_ToolIntegration(t *testing.T) {
	// Test using the tool through the AgentTool interface
	tool := NewPythonSandboxTool()

	// Test AgentRun is passed (matches aigentic.NewTool pattern)
	type PythonSandboxInput struct {
		Code    string `json:"code" description:"Python code to execute"`
		Timeout int    `json:"timeout,omitempty" description:"Timeout in seconds"`
	}

	// Simulate what aigentic.NewTool's Execute function does
	args := map[string]interface{}{
		"code": "print('Integration test')",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("tool.Execute failed: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "Integration test") {
		t.Errorf("expected output, got: %s", output)
	}
}

// Benchmark tests
func BenchmarkPythonSandbox_SimpleExecution(b *testing.B) {
	tool := NewPythonSandboxTool()
	args := map[string]interface{}{
		"code": "print('benchmark')",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.Execute(&aigentic.AgentRun{}, args)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestPythonSandbox_ImportStandardLibrary(t *testing.T) {
	tool := NewPythonSandboxTool()

	code := `
import json
import os
import sys

data = {"test": "value", "number": 42}
print(json.dumps(data))
print(f"Python version: {sys.version_info.major}.{sys.version_info.minor}")
`

	args := map[string]interface{}{
		"code": code,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.Contains(output, `"test": "value"`) && !strings.Contains(output, `"test":"value"`) {
		t.Errorf("expected JSON output, got: %s", output)
	}
	if !strings.Contains(output, "Python version:") {
		t.Errorf("expected Python version in output, got: %s", output)
	}
}

func TestPythonSandbox_ExecutionWithTimeout(t *testing.T) {
	// Verify that the tool works with timeout as expected
	tool := NewPythonSandboxTool()

	// The timeout is used internally in executePythonCode
	args := map[string]interface{}{
		"code":    "print('timeout test')",
		"timeout": 5,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}
}
