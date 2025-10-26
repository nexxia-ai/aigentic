package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic"
)

func TestNewReadFileTool(t *testing.T) {
	tool := NewReadFileTool()

	if tool.Name != ReadFileToolName {
		t.Errorf("expected name '%s', got %s", ReadFileToolName, tool.Name)
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
	if props["file_name"] == nil {
		t.Error("expected 'file_name' property in schema")
	}

	if props["store_name"] == nil {
		t.Error("expected 'store_name' property in schema")
	}

	// Check required fields
	required := schema["required"].([]string)
	hasFileName := false
	hasStoreName := false
	for _, r := range required {
		if r == "file_name" {
			hasFileName = true
		}
		if r == "store_name" {
			hasStoreName = true
		}
	}
	if !hasFileName {
		t.Error("file_name should be required")
	}
	if !hasStoreName {
		t.Error("store_name should be required")
	}
}

func TestReadFile_Success(t *testing.T) {
	// Setup: Create a temporary directory and file
	tempDir := t.TempDir()
	testContent := "Hello, World!\nThis is a test file."
	testFile := "test.txt"

	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test
	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if output != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, output)
	}
}

func TestReadFile_MultilineContent(t *testing.T) {
	tempDir := t.TempDir()
	testContent := `Line 1
Line 2
Line 3
Line 4`
	testFile := "multiline.txt"

	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if output != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, output)
	}
}

func TestReadFile_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "empty.txt"

	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if output != "" {
		t.Errorf("expected empty content, got '%s'", output)
	}
}

func TestReadFile_BinaryFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "binary.bin"
	// Create binary content (non-UTF-8)
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}

	err := os.WriteFile(filepath.Join(tempDir, testFile), binaryContent, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if !strings.HasPrefix(output, "[BINARY FILE - Base64 Encoded]") {
		t.Errorf("expected base64 encoded output for binary file, got: %s", output)
	}

	// Verify base64 content is present
	if !strings.Contains(output, "\n") {
		t.Error("expected base64 content after header")
	}
}

func TestReadFile_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  "nonexistent.txt",
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	if result != nil && !result.Error {
		t.Error("expected error result")
	}

	if err != nil && !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected 'file not found' error, got: %v", err)
	}
}

func TestReadFile_DirectoryInsteadOfFile(t *testing.T) {
	tempDir := t.TempDir()
	subDir := "subdir"

	err := os.Mkdir(filepath.Join(tempDir, subDir), 0755)
	if err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  subDir,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err == nil {
		t.Error("expected error when trying to read directory")
	}

	if result != nil && !result.Error {
		t.Error("expected error result")
	}

	if err != nil && !strings.Contains(err.Error(), "directory") {
		t.Errorf("expected directory error, got: %v", err)
	}
}

func TestReadFile_MissingFileName(t *testing.T) {
	tempDir := t.TempDir()

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  "",
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err == nil {
		t.Error("expected error for missing file_name")
	}

	if result != nil && !result.Error {
		t.Error("expected error result")
	}

	if err != nil && !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

func TestReadFile_MissingStoreName(t *testing.T) {
	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  "test.txt",
		"store_name": "",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err == nil {
		t.Error("expected error for missing store_name")
	}

	if result != nil && !result.Error {
		t.Error("expected error result")
	}

	if err != nil && !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

func TestReadFile_NestedDirectory(t *testing.T) {
	tempDir := t.TempDir()
	subDir := "subdir"
	testFile := "nested.txt"
	testContent := "Nested file content"

	// Create nested directory
	err := os.Mkdir(filepath.Join(tempDir, subDir), 0755)
	if err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create file in nested directory
	err = os.WriteFile(filepath.Join(tempDir, subDir, testFile), []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  filepath.Join(subDir, testFile),
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if output != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, output)
	}
}

func TestReadFile_SpecialCharacters(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "special.txt"
	testContent := "Special chars: @#$%^&*()_+-={}[]|\\:\";<>?,./\nEmojis: 😀🎉\nUnicode: 你好世界"

	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if output != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, output)
	}
}

func TestReadFile_LargeFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "large.txt"

	// Create a large file (1MB of repeated content)
	var builder strings.Builder
	line := "This is a test line with some content.\n"
	for i := 0; i < 25000; i++ { // ~1MB
		builder.WriteString(line)
	}
	testContent := builder.String()

	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if len(output) != len(testContent) {
		t.Errorf("expected content length %d, got %d", len(testContent), len(output))
	}
}

func TestReadFile_JSONContent(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "data.json"
	testContent := `{
  "name": "test",
  "value": 42,
  "items": ["a", "b", "c"]
}`

	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	output := result.Content[0].Content.(string)
	if output != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, output)
	}
}

// Benchmark tests
func BenchmarkReadFile_SmallText(b *testing.B) {
	tempDir := b.TempDir()
	testFile := "bench.txt"
	testContent := "Small test content"

	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(testContent), 0644)
	if err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}

	tool := NewReadFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.Execute(&aigentic.AgentRun{}, args)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
