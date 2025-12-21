package tools

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/run"
)

// TestWriteThenRead tests the integration of write and read operations
func TestWriteThenRead_TextFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "integration.txt"
	testContent := "Integration test content\nLine 2\nLine 3"

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write file
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if writeResult.Error {
		t.Errorf("write error: %s", writeResult.Content[0].Content)
	}

	// Read file back
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if readResult.Error {
		t.Errorf("read error: %s", readResult.Content[0].Content)
	}

	// Verify content matches
	readContent := readResult.Content[0].Content.(string)
	if readContent != testContent {
		t.Errorf("content mismatch: expected '%s', got '%s'", testContent, readContent)
	}
}

// TestWriteThenRead_BinaryFile tests binary file round-trip
func TestWriteThenRead_BinaryFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "binary.bin"
	binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC}
	base64Content := base64.StdEncoding.EncodeToString(binaryData)
	writeContent := "[BINARY FILE - Base64 Encoded]\n" + base64Content

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write binary file
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    writeContent,
	}

	writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if writeResult.Error {
		t.Errorf("write error: %s", writeResult.Content[0].Content)
	}

	// Read binary file back
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if readResult.Error {
		t.Errorf("read error: %s", readResult.Content[0].Content)
	}

	// Verify content is base64 encoded
	readContent := readResult.Content[0].Content.(string)
	if !strings.HasPrefix(readContent, "[BINARY FILE - Base64 Encoded]") {
		t.Error("expected base64 encoded content")
	}

	// Extract and decode base64
	lines := strings.Split(readContent, "\n")
	if len(lines) < 2 {
		t.Fatal("expected base64 content after header")
	}

	decodedBase64 := strings.Join(lines[1:], "\n")
	decodedData, err := base64.StdEncoding.DecodeString(decodedBase64)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	// Verify binary data matches
	if len(decodedData) != len(binaryData) {
		t.Errorf("data length mismatch: expected %d, got %d", len(binaryData), len(decodedData))
	}

	for i, b := range binaryData {
		if decodedData[i] != b {
			t.Errorf("byte %d mismatch: expected 0x%02x, got 0x%02x", i, b, decodedData[i])
		}
	}
}

// TestWriteThenRead_MultipleFiles tests multiple file operations
func TestWriteThenRead_MultipleFiles(t *testing.T) {
	tempDir := t.TempDir()
	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	files := map[string]string{
		"file1.txt": "Content 1",
		"file2.txt": "Content 2",
		"file3.txt": "Content 3",
	}

	// Write all files
	for fileName, content := range files {
		writeArgs := map[string]interface{}{
			"file_name":  fileName,
			"store_name": tempDir,
			"content":    content,
		}

		writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
		if err != nil {
			t.Fatalf("write failed for %s: %v", fileName, err)
		}

		if writeResult.Error {
			t.Errorf("write error for %s: %s", fileName, writeResult.Content[0].Content)
		}
	}

	// Read all files and verify
	for fileName, expectedContent := range files {
		readArgs := map[string]interface{}{
			"file_name":  fileName,
			"store_name": tempDir,
		}

		readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
		if err != nil {
			t.Fatalf("read failed for %s: %v", fileName, err)
		}

		if readResult.Error {
			t.Errorf("read error for %s: %s", fileName, readResult.Content[0].Content)
		}

		readContent := readResult.Content[0].Content.(string)
		if readContent != expectedContent {
			t.Errorf("content mismatch for %s: expected '%s', got '%s'", fileName, expectedContent, readContent)
		}
	}
}

// TestWriteReadModifyWrite tests modifying a file
func TestWriteReadModifyWrite(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "modify.txt"
	originalContent := "Original content"
	modifiedContent := "Original content\nModified line"

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write original file
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    originalContent,
	}

	_, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// Read file
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	readContent := readResult.Content[0].Content.(string)
	if readContent != originalContent {
		t.Errorf("initial read mismatch: expected '%s', got '%s'", originalContent, readContent)
	}

	// Modify and write back
	writeArgs["content"] = modifiedContent
	_, err = writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("modified write failed: %v", err)
	}

	// Read again
	readResult, err = readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}

	readContent = readResult.Content[0].Content.(string)
	if readContent != modifiedContent {
		t.Errorf("modified read mismatch: expected '%s', got '%s'", modifiedContent, readContent)
	}
}

// TestNestedDirectoryOperations tests reading/writing in nested directories
func TestNestedDirectoryOperations(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "subdir1/subdir2/nested.txt"
	testContent := "Nested directory content"

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write to nested directory (should create directories)
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("write to nested directory failed: %v", err)
	}

	if writeResult.Error {
		t.Errorf("write error: %s", writeResult.Content[0].Content)
	}

	// Read from nested directory
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read from nested directory failed: %v", err)
	}

	if readResult.Error {
		t.Errorf("read error: %s", readResult.Content[0].Content)
	}

	readContent := readResult.Content[0].Content.(string)
	if readContent != testContent {
		t.Errorf("content mismatch: expected '%s', got '%s'", testContent, readContent)
	}

	// Verify directory structure exists
	dirPath := filepath.Join(tempDir, "subdir1", "subdir2")
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory to exist")
	}
}

// TestEmptyFileOperations tests writing and reading empty files
func TestEmptyFileOperations(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "empty.txt"

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write empty file
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    "",
	}

	writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("write empty file failed: %v", err)
	}

	if writeResult.Error {
		t.Errorf("write error: %s", writeResult.Content[0].Content)
	}

	// Read empty file
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read empty file failed: %v", err)
	}

	if readResult.Error {
		t.Errorf("read error: %s", readResult.Content[0].Content)
	}

	readContent := readResult.Content[0].Content.(string)
	if readContent != "" {
		t.Errorf("expected empty content, got '%s'", readContent)
	}
}

// TestLargeFileOperations tests large file handling
func TestLargeFileOperations(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "large.txt"

	// Create 1MB content
	var builder strings.Builder
	line := "This is a test line with some content that repeats.\n"
	for i := 0; i < 20000; i++ {
		builder.WriteString(line)
	}
	testContent := builder.String()

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write large file
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("write large file failed: %v", err)
	}

	if writeResult.Error {
		t.Errorf("write error: %s", writeResult.Content[0].Content)
	}

	// Read large file
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read large file failed: %v", err)
	}

	if readResult.Error {
		t.Errorf("read error: %s", readResult.Content[0].Content)
	}

	readContent := readResult.Content[0].Content.(string)
	if len(readContent) != len(testContent) {
		t.Errorf("content length mismatch: expected %d, got %d", len(testContent), len(readContent))
	}
}

// TestUnicodeAndSpecialCharacters tests handling of special characters
func TestUnicodeAndSpecialCharacters(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "unicode.txt"
	testContent := "Unicode: 你好世界 🎉\nSpecial: @#$%^&*()\nNewlines:\n\n\nTabs:\t\t\t"

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write file with special characters
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if writeResult.Error {
		t.Errorf("write error: %s", writeResult.Content[0].Content)
	}

	// Read file with special characters
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if readResult.Error {
		t.Errorf("read error: %s", readResult.Content[0].Content)
	}

	readContent := readResult.Content[0].Content.(string)
	if readContent != testContent {
		t.Errorf("content mismatch: expected '%s', got '%s'", testContent, readContent)
	}
}

// TestJSONRoundTrip tests JSON file operations
func TestJSONRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	testFile := "config.json"
	testContent := `{
  "name": "test-app",
  "version": "1.0.0",
  "settings": {
    "enabled": true,
    "timeout": 30
  },
  "items": [
    "item1",
    "item2",
    "item3"
  ]
}`

	writeTool := NewWriteFileTool()
	readTool := NewReadFileTool()

	// Write JSON file
	writeArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
		"content":    testContent,
	}

	writeResult, err := writeTool.Execute(&run.AgentRun{}, writeArgs)
	if err != nil {
		t.Fatalf("write JSON failed: %v", err)
	}

	if writeResult.Error {
		t.Errorf("write error: %s", writeResult.Content[0].Content)
	}

	// Read JSON file
	readArgs := map[string]interface{}{
		"file_name":  testFile,
		"store_name": tempDir,
	}

	readResult, err := readTool.Execute(&run.AgentRun{}, readArgs)
	if err != nil {
		t.Fatalf("read JSON failed: %v", err)
	}

	if readResult.Error {
		t.Errorf("read error: %s", readResult.Content[0].Content)
	}

	readContent := readResult.Content[0].Content.(string)
	if readContent != testContent {
		t.Errorf("JSON content mismatch: expected '%s', got '%s'", testContent, readContent)
	}
}
