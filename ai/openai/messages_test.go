package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexxia-ai/aigentic/ai"
)

func TestToResponsesUserItem_FilePart_FileDataIsDataURI(t *testing.T) {
	msg := ai.UserMessage{
		Role: ai.UserRole,
		Parts: []ai.ContentPart{{
			Type:     ai.ContentPartFile,
			MimeType: "application/pdf",
			Name:     "doc.pdf",
			Data:     []byte("pdf content"),
		}},
	}
	item, err := toResponsesUserItem(msg)
	if err != nil {
		t.Fatalf("toResponsesUserItem: %v", err)
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	content, ok := parsed["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array with file part, got %T %v", parsed["content"], parsed["content"])
	}
	part, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected part as object, got %T", content[0])
	}
	fileData, _ := part["file_data"].(string)
	if fileData == "" {
		t.Fatal("file_data is empty")
	}
	if !strings.HasPrefix(fileData, "data:") {
		t.Errorf("file_data must be data URI, got: %q", fileData)
	}
	if !strings.Contains(fileData, ";base64,") {
		t.Errorf("file_data must contain ;base64,, got: %q", fileData)
	}
	if !strings.HasPrefix(fileData, "data:application/pdf;base64,") {
		t.Errorf("file_data must start with data:application/pdf;base64,, got: %q", fileData)
	}
	filename, _ := part["filename"].(string)
	if filename != "doc.pdf" {
		t.Errorf("filename want doc.pdf, got %q", filename)
	}
}

func TestToResponsesUserItem_FilePart_MimeFallback(t *testing.T) {
	msg := ai.UserMessage{
		Role: ai.UserRole,
		Parts: []ai.ContentPart{{
			Type: ai.ContentPartFile,
			Data: []byte("binary"),
		}},
	}
	item, err := toResponsesUserItem(msg)
	if err != nil {
		t.Fatalf("toResponsesUserItem: %v", err)
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	content := parsed["content"].([]interface{})
	part := content[0].(map[string]interface{})
	fileData, _ := part["file_data"].(string)
	if !strings.HasPrefix(fileData, "data:application/octet-stream;base64,") {
		t.Errorf("file_data must use application/octet-stream when MimeType empty, got: %q", fileData)
	}
}

func TestToChatUserMessage_FilePart_FileDataIsDataURI(t *testing.T) {
	msg := ai.UserMessage{
		Role: ai.UserRole,
		Parts: []ai.ContentPart{{
			Type:     ai.ContentPartFile,
			MimeType: "application/pdf",
			Name:     "doc.pdf",
			Data:     []byte("pdf content"),
		}},
	}
	chatMsg, err := toChatUserMessage(msg)
	if err != nil {
		t.Fatalf("toChatUserMessage: %v", err)
	}
	raw, err := json.Marshal(chatMsg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	parts, ok := parsed["content"].([]interface{})
	if !ok || len(parts) == 0 {
		t.Fatalf("expected content parts array, got %T %v", parsed["content"], parsed["content"])
	}
	part, ok := parts[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected part as object, got %T", parts[0])
	}
	filePart, ok := part["file"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected file part, got %v", part)
	}
	fileData, _ := filePart["file_data"].(string)
	if fileData == "" {
		t.Fatal("file_data is empty")
	}
	if !strings.HasPrefix(fileData, "data:") {
		t.Errorf("file_data must be data URI, got: %q", fileData)
	}
	if !strings.Contains(fileData, ";base64,") {
		t.Errorf("file_data must contain ;base64,, got: %q", fileData)
	}
	if !strings.HasPrefix(fileData, "data:application/pdf;base64,") {
		t.Errorf("file_data must start with data:application/pdf;base64,, got: %q", fileData)
	}
	filename, _ := filePart["filename"].(string)
	if filename != "doc.pdf" {
		t.Errorf("filename want doc.pdf, got %q", filename)
	}
}

func TestToChatUserMessage_FilePart_MimeFallback(t *testing.T) {
	msg := ai.UserMessage{
		Role: ai.UserRole,
		Parts: []ai.ContentPart{{
			Type: ai.ContentPartFile,
			Data: []byte("binary"),
		}},
	}
	chatMsg, err := toChatUserMessage(msg)
	if err != nil {
		t.Fatalf("toChatUserMessage: %v", err)
	}
	raw, err := json.Marshal(chatMsg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	parts := parsed["content"].([]interface{})
	part := parts[0].(map[string]interface{})
	filePart, ok := part["file"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected file part, got %v", part)
	}
	fileData, _ := filePart["file_data"].(string)
	if !strings.HasPrefix(fileData, "data:application/octet-stream;base64,") {
		t.Errorf("file_data must use application/octet-stream when MimeType empty, got: %q", fileData)
	}
}

func TestToResponsesUserItem_FilePart_FileIDOnly(t *testing.T) {
	msg := ai.UserMessage{
		Role: ai.UserRole,
		Parts: []ai.ContentPart{{
			Type:   ai.ContentPartFile,
			FileID: "file-abc123",
		}},
	}
	item, err := toResponsesUserItem(msg)
	if err != nil {
		t.Fatalf("toResponsesUserItem: %v", err)
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	content := parsed["content"].([]interface{})
	part := content[0].(map[string]interface{})
	if fileID, _ := part["file_id"].(string); fileID != "file-abc123" {
		t.Errorf("file_id want file-abc123, got %q", fileID)
	}
	if fd, ok := part["file_data"]; ok && fd != nil && fd != "" {
		t.Errorf("file_data should be omitted when using FileID, got %v", fd)
	}
}

func TestToChatUserMessage_FilePart_FileIDOnly(t *testing.T) {
	msg := ai.UserMessage{
		Role: ai.UserRole,
		Parts: []ai.ContentPart{{
			Type:   ai.ContentPartFile,
			FileID: "file-abc123",
		}},
	}
	chatMsg, err := toChatUserMessage(msg)
	if err != nil {
		t.Fatalf("toChatUserMessage: %v", err)
	}
	raw, err := json.Marshal(chatMsg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	parts := parsed["content"].([]interface{})
	filePart := parts[0].(map[string]interface{})["file"].(map[string]interface{})
	if fileID, _ := filePart["file_id"].(string); fileID != "file-abc123" {
		t.Errorf("file_id want file-abc123, got %q", fileID)
	}
	if fd, ok := filePart["file_data"]; ok && fd != nil && fd != "" {
		t.Errorf("file_data should be omitted when using FileID, got %v", fd)
	}
}

func TestToResponsesUserItem_ImagePart_DataURI(t *testing.T) {
	msg := ai.UserMessage{
		Role: ai.UserRole,
		Parts: []ai.ContentPart{{
			Type:     ai.ContentPartImage,
			MimeType: "image/png",
			Data:     []byte("png bytes"),
		}},
	}
	item, err := toResponsesUserItem(msg)
	if err != nil {
		t.Fatalf("toResponsesUserItem: %v", err)
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	content := parsed["content"].([]interface{})
	part := content[0].(map[string]interface{})
	url, _ := part["image_url"].(string)
	if url == "" {
		t.Fatalf("expected image_url, got %v", part)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Errorf("image URL must be data URI, got: %q", url)
	}
}

func TestToChatUserMessage_TextOnly(t *testing.T) {
	msg := ai.UserMessage{
		Role:    ai.UserRole,
		Content: "hello",
	}
	chatMsg, err := toChatUserMessage(msg)
	if err != nil {
		t.Fatalf("toChatUserMessage: %v", err)
	}
	raw, err := json.Marshal(chatMsg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	content, ok := parsed["content"].(string)
	if !ok || content != "hello" {
		t.Errorf("content want hello, got %q", content)
	}
}

func TestToResponsesUserItem_TextOnly(t *testing.T) {
	msg := ai.UserMessage{
		Role:    ai.UserRole,
		Content: "hello",
	}
	item, err := toResponsesUserItem(msg)
	if err != nil {
		t.Fatalf("toResponsesUserItem: %v", err)
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	contentList := parsed["content"].([]interface{})
	textPart := contentList[0].(map[string]interface{})
	if text, _ := textPart["text"].(string); text != "hello" {
		t.Errorf("text want hello, got %q", textPart["text"])
	}
}

