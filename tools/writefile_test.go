package tools

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic"
)

func TestNewWriteFileTool(t *testing.T) {
	tool := NewWriteFileTool()

	if tool.Name != WriteFileToolName {
		t.Errorf("expected name '%s', got %s", WriteFileToolName, tool.Name)
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

	if props["content"] == nil {
		t.Error("expected 'content' property in schema")
	}

	// Check required fields
	required := schema["required"].([]string)
	hasFileName := false
	hasStoreName := false
	hasContent := false
	for _, r := range required {
		if r == "file_name" {
			hasFileName = true
		}
		if r == "store_name" {
			hasStoreName = true
		}
		if r == "content" {
			hasContent = true
		}
	}
	if !hasFileName {
		t.Error("file_name should be required")
	}
	if !hasStoreName {
		t.Error("store_name should be required")
	}
	if !hasContent {
		t.Error("content should be required")
	}
}

func TestWriteFile_Success(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "test.txt"
	testContent := "Hello, World!\nThis is a test file."

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify file was created and has correct content
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(writtenContent) != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, string(writtenContent))
	}

	// Check success message
	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "Successfully wrote") {
		t.Errorf("expected success message, got: %s", output)
	}
	if !strings.Contains(output, "text file") {
		t.Errorf("expected 'text file' in message, got: %s", output)
	}
}

func TestWriteFile_OverwriteExisting(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "overwrite.txt"
	originalContent := "Original content"
	newContent := "New content"

	// Create original file
	err := os.WriteFile(filepath.Join(tempDir, testFile), []byte(originalContent), 0644)
	if err != nil {
		t.Fatalf("failed to create original file: %v", err)
	}

	// Overwrite with new content
	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    newContent,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify file was overwritten
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(writtenContent) != newContent {
		t.Errorf("expected content '%s', got '%s'", newContent, string(writtenContent))
	}
}

func TestWriteFile_EmptyContent(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "empty.txt"

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    "",
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify file exists and is empty
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if len(writtenContent) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(writtenContent))
	}
}

func TestWriteFile_BinaryContent(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "binary.bin"
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	base64Content := base64.StdEncoding.EncodeToString(binaryData)
	content := "[BINARY FILE - Base64 Encoded]\n" + base64Content

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    content,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify binary file was written correctly
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if len(writtenContent) != len(binaryData) {
		t.Errorf("expected %d bytes, got %d bytes", len(binaryData), len(writtenContent))
	}

	for i, b := range binaryData {
		if writtenContent[i] != b {
			t.Errorf("byte %d: expected 0x%02x, got 0x%02x", i, b, writtenContent[i])
		}
	}

	// Check success message mentions binary
	output := result.Content[0].Content.(string)
	if !strings.Contains(output, "binary file") {
		t.Errorf("expected 'binary file' in message, got: %s", output)
	}
}

func TestWriteFile_MultilineContent(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "multiline.txt"
	testContent := `Line 1
Line 2
Line 3
Line 4`

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify content
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(writtenContent) != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, string(writtenContent))
	}
}

func TestWriteFile_CreateNestedDirectory(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "subdir/nested/file.txt"
	testContent := "Nested file content"

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify nested directories were created
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(writtenContent) != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, string(writtenContent))
	}

	// Verify directories exist
	dirPath := filepath.Join(tempDir, "subdir", "nested")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory to be created")
	}
}

func TestWriteFile_MissingFileName(t *testing.T) {
	tempDir := t.TempDir()

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  "",
		"store_name": tempDir,
		"content":    "test",
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

func TestWriteFile_MissingStoreName(t *testing.T) {
	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  "test.txt",
		"store_name": "",
		"content":    "test",
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

func TestWriteFile_InvalidBase64(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "invalid.bin"
	// Invalid base64 content
	content := "[BINARY FILE - Base64 Encoded]\nNot valid base64!!!"

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    content,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err == nil {
		t.Error("expected error for invalid base64")
	}

	if result != nil && !result.Error {
		t.Error("expected error result")
	}

	if err != nil && !strings.Contains(err.Error(), "decoding base64") {
		t.Errorf("expected base64 decode error, got: %v", err)
	}
}

func TestWriteFile_MalformedBase64Header(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "malformed.bin"
	// Malformed base64 header (missing content)
	content := "[BINARY FILE - Base64 Encoded]"

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    content,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err == nil {
		t.Error("expected error for malformed base64 header")
	}

	if result != nil && !result.Error {
		t.Error("expected error result")
	}

	if err != nil && !strings.Contains(err.Error(), "invalid base64 content format") {
		t.Errorf("expected format error, got: %v", err)
	}
}

func TestWriteFile_SpecialCharacters(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "special.txt"
	testContent := "Special chars: @#$%^&*()_+-={}[]|\\:\";<>?,./\nEmojis: 😀🎉\nUnicode: 你好世界"

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify content
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(writtenContent) != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, string(writtenContent))
	}
}

func TestWriteFile_JSONContent(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "data.json"
	testContent := `{
  "name": "test",
  "value": 42,
  "items": ["a", "b", "c"]
}`

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify content
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if string(writtenContent) != testContent {
		t.Errorf("expected content '%s', got '%s'", testContent, string(writtenContent))
	}
}

func TestWriteFile_LargeFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "large.txt"

	// Create large content (1MB)
	var builder strings.Builder
	line := "This is a test line with some content.\n"
	for i := 0; i < 25000; i++ {
		builder.WriteString(line)
	}
	testContent := builder.String()

	tool := NewWriteFileTool()
	args := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	result, err := tool.Execute(&aigentic.AgentRun{}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error {
		t.Errorf("expected success, got error: %s", result.Content[0].Content)
	}

	// Verify file size
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, testFile))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	if len(writtenContent) != len(testContent) {
		t.Errorf("expected %d bytes, got %d bytes", len(testContent), len(writtenContent))
	}
}

// Benchmark tests
func BenchmarkWriteFile_SmallText(b *testing.B) {
	tempDir := b.TempDir()
	testContent := "Small test content"

	tool := NewWriteFileTool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		args := map[string]interface{}{
			"file_name":  "bench.txt",
			"store_name": tempDir,
			"content":    testContent,
		}
		_, err := tool.Execute(&aigentic.AgentRun{}, args)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
