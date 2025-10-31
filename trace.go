package aigentic

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

const (
	defaultRetentionDuration = 7 * 24 * time.Hour
	defaultMaxTraceFiles     = 10
)

var (
	traceSync = sync.Mutex{} // keep all trace lines in sync
)

type TraceConfig struct {
	SessionID         string
	Directory         string
	RetentionDuration time.Duration
	MaxTraceFiles     int
}

// Trace stores the execution trace of an LLM.
// Trace implements Interceptor for automatic tracing via interceptors.
type Trace struct {
	SessionID         string        // Unique session ID for the entire interaction
	StartTime         time.Time     // Start time of the trace
	EndTime           time.Time     // End time of the trace
	directory         string        // Path to the trace directory
	Filepath          string        // Path to the trace file
	file              traceWriter   // File to write traces to (or io.Discard if file creation fails)
	RetentionDuration time.Duration // How long to keep traces
	MaxTraceFiles     int           // Maximum number of files to keep
	fileInitialized   bool          // Whether the file has been created and opened
}

// traceWriter interface for writing trace data
type traceWriter interface {
	io.Writer
	Sync() error
	Close() error
}

// discardWriter wraps io.Discard to implement traceWriter interface
type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return io.Discard.Write(p)
}

func (d *discardWriter) Sync() error {
	return nil
}

func (d *discardWriter) Close() error {
	return nil
}

func newTraceID() string {
	now := time.Now()
	return now.Format("20060102150405") + fmt.Sprintf("%09d", now.Nanosecond())
}

// ensureFileInitialized creates and opens the trace file if it hasn't been initialized yet
func (t *Trace) ensureFileInitialized() error {
	if t.fileInitialized {
		return nil
	}

	var file traceWriter
	osFile, err := os.OpenFile(t.Filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Failed to open trace file, using io.Discard", "file", t.Filepath, "error", err)
		// Use io.Discard wrapped in a traceWriter when file creation fails
		file = &discardWriter{}
	} else {
		// Write initial header to the file
		fmt.Fprintf(osFile, "trace for sessionID: %s\n", t.SessionID)
		file = osFile
	}

	t.file = file
	t.fileInitialized = true
	slog.Debug("Trace file initialized", "file", t.Filepath)
	return nil
}

// NewTrace creates a new Trace instance with default cleanup settings.
func NewTrace(config ...TraceConfig) *Trace {
	defaultDir := filepath.Join(os.TempDir(), "aigentic-traces")

	cfg := TraceConfig{
		Directory:         defaultDir,
		RetentionDuration: defaultRetentionDuration,
		MaxTraceFiles:     defaultMaxTraceFiles,
		SessionID:         newTraceID(),
	}

	if len(config) > 0 {
		if config[0].Directory != "" {
			cfg.Directory = config[0].Directory
		}
		if config[0].RetentionDuration > 0 {
			cfg.RetentionDuration = config[0].RetentionDuration
		}
		if config[0].MaxTraceFiles > 0 {
			cfg.MaxTraceFiles = config[0].MaxTraceFiles
		}
		if config[0].SessionID != "" {
			cfg.SessionID = config[0].SessionID
		}
	}

	// Create the trace directory if it doesn't exist
	if _, err := os.Stat(cfg.Directory); os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
			slog.Error("Failed to create trace directory", "directory", cfg.Directory, "error", err)
		}
	}

	filename := filepath.Join(cfg.Directory, fmt.Sprintf("trace-%s.txt", cfg.SessionID))

	t := &Trace{
		SessionID:         cfg.SessionID,
		StartTime:         time.Now(),
		Filepath:          filename,
		file:              nil, // File will be created on first write
		directory:         cfg.Directory,
		RetentionDuration: cfg.RetentionDuration,
		MaxTraceFiles:     cfg.MaxTraceFiles,
		fileInitialized:   false,
	}
	slog.Debug("Trace file will be created at", "file", filename)

	t.Cleanup() // remove old entries

	return t
}

// Cleanup removes old trace files based on retention policy
func (t *Trace) Cleanup() {
	entries, err := os.ReadDir(t.directory)
	if err != nil {
		slog.Error("Failed to read trace directory", "error", err)
		return
	}

	var traceFiles []struct {
		path    string
		modTime time.Time
	}

	cutoffTime := time.Now().Add(-t.RetentionDuration)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "trace-") || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		filePath := filepath.Join(t.directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		traceFiles = append(traceFiles, struct {
			path    string
			modTime time.Time
		}{
			path:    filePath,
			modTime: info.ModTime(),
		})
	}

	// Sort by modification time (oldest first)
	sort.Slice(traceFiles, func(i, j int) bool {
		return traceFiles[i].modTime.Before(traceFiles[j].modTime)
	})

	// Remove files older than retention duration
	if t.RetentionDuration > 0 {
		for _, file := range traceFiles {
			if file.modTime.Before(cutoffTime) {
				if err := os.Remove(file.path); err != nil {
					slog.Error("Failed to remove old trace file", "file", file.path, "error", err)
				} else {
					slog.Debug("Removed old trace file", "file", filepath.Base(file.path))
				}
			}
		}
	}

	// If we still have too many files, remove the oldest ones
	if t.MaxTraceFiles > 0 && len(traceFiles) > t.MaxTraceFiles {
		filesToRemove := len(traceFiles) - t.MaxTraceFiles
		for i := 0; i < filesToRemove && i < len(traceFiles); i++ {
			if err := os.Remove(traceFiles[i].path); err != nil {
				slog.Error("Failed to remove excess trace file", "file", traceFiles[i].path, "error", err)
			} else {
				slog.Debug("Removed excess trace file", "file", filepath.Base(traceFiles[i].path))
			}
		}
	}
}

// BeforeCall implements Interceptor - records LLM call before invocation
func (t *Trace) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	if err := t.ensureFileInitialized(); err != nil {
		return messages, tools, err
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "\n====> [%s] Start %s (%s) sessionID: %s\n", time.Now().Format("15:04:05"),
		run.agent.Name, run.model.ModelName, t.SessionID)

	for _, message := range messages {
		role, _ := message.Value()

		// Handle each message type specifically
		switch msg := message.(type) {
		case ai.UserMessage:
			fmt.Fprintf(t.file, "⬆️  %s:\n", role)
			t.logMessageContent("content", msg.Content)
		case ai.SystemMessage:
			fmt.Fprintf(t.file, "⬆️  %s:\n", role)
			t.logMessageContent("content", msg.Content)
		case ai.AIMessage:
			fmt.Fprintf(t.file, "⬆️  assistant: role=%s\n", msg.Role) // Role might vary by provider
			t.logAIMessage(msg)
		case ai.ToolMessage:
			fmt.Fprintf(t.file, "⬆️  %s:\n", role)
			fmt.Fprintf(t.file, " tool_call_id: %s\n", msg.ToolCallID)
			t.logMessageContent("content", msg.Content)
		case ai.ResourceMessage:
			fmt.Fprintf(t.file, "⬆️  %s:\n", role)
			// Determine if this is a file ID reference or has content
			var isFileID bool
			var contentLen int
			var contentPreview string

			if body, ok := msg.Body.([]byte); ok && body != nil {
				// Has actual content
				isFileID = false
				contentLen = len(body)
				if contentLen > 0 {
					// Show first 64 characters of content
					previewLen := 64
					if contentLen < previewLen {
						previewLen = contentLen
					}
					contentPreview = string(body[:previewLen])
				}
			} else {
				// Likely a file ID reference
				isFileID = true
				contentLen = len(msg.Name)
			}

			// Log the resource type and basic info
			if isFileID {
				fmt.Fprintf(t.file, " resource: %s (file ID reference)\n", msg.Name)
			} else {
				fmt.Fprintf(t.file, " resource: %s (content length: %d)\n", msg.Name, contentLen)
			}

			// Log additional metadata
			if msg.URI != "" {
				fmt.Fprintf(t.file, " uri: %s\n", msg.URI)
			}
			if msg.MIMEType != "" {
				fmt.Fprintf(t.file, " mime_type: %s\n", msg.MIMEType)
			}
			if msg.Description != "" {
				fmt.Fprintf(t.file, " description: %s\n", msg.Description)
			}

			// Log content preview if available
			if contentPreview != "" {
				t.logMessageContent("content_preview", contentPreview)
			}
		default:
			// Fallback for unknown message types
			_, content := message.Value()
			t.logMessageContent("content", content)
		}
	}

	t.file.Sync()
	return messages, tools, nil
}

// AfterCall implements Interceptor - records LLM response after invocation
func (t *Trace) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	if err := t.ensureFileInitialized(); err != nil {
		return response, err
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "⬇️  assistant: role=%s\n", response.Role) // Role might vary by provider
	t.logAIMessage(response)
	t.file.Sync()

	fmt.Fprintf(t.file, "==== [%s] End %s\n\n", time.Now().Format("15:04:05"), run.agent.Name)

	return response, nil
}

// BeforeToolCall implements Interceptor - records tool call before execution
func (t *Trace) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult) (ValidationResult, error) {
	if err := t.ensureFileInitialized(); err != nil {
		return validationResult, err
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "\n---- Tool START: %s (callID=%s) agent=%s\n", toolName, toolCallID, run.agent.Name)

	argsJSON, _ := json.Marshal(validationResult)
	fmt.Fprintf(t.file, " args: %s\n", string(argsJSON))
	t.file.Sync()

	return validationResult, nil
}

// AfterToolCall implements Interceptor - records tool call after execution
func (t *Trace) AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
	if err := t.ensureFileInitialized(); err != nil {
		return result, err
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	response := ""
	if result != nil {
		if len(result.Content) > 0 {
			parts := make([]string, 0, len(result.Content))
			for _, item := range result.Content {
				segment := ""
				switch v := item.Content.(type) {
				case string:
					segment = v
				case []byte:
					if len(v) > 0 {
						segment = string(v)
					}
				default:
					encoded, err := json.Marshal(v)
					if err == nil {
						segment = string(encoded)
					}
				}
				if segment != "" {
					if item.Type != "" && item.Type != "text" {
						segment = fmt.Sprintf("[%s] %s", item.Type, segment)
					}
					parts = append(parts, segment)
				}
			}
			response = strings.Join(parts, "\n")
		}
		if result.Error {
			response = fmt.Sprintf("ERROR: %s", response)
		}
	}

	fmt.Fprintf(t.file, " result: %s\n", response)
	fmt.Fprintf(t.file, "---- Tool END: %s (callID=%s)\n", toolName, toolCallID)

	argsJSON, _ := json.Marshal(validationResult)
	fmt.Fprintf(t.file, "🛠️️  %s tool response:\n", run.agent.Name)
	fmt.Fprintf(t.file, "   • %s(%s)\n", toolName, string(argsJSON))

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(t.file, "     %s\n", line)
		}
	}
	t.file.Sync()

	return result, nil
}

// logMessageContent is a helper method to format and log message content
func (t *Trace) logMessageContent(contentType, content string) {
	if content == "" {
		fmt.Fprintf(t.file, " %s: (empty)\n", contentType)
		return
	}

	fmt.Fprintf(t.file, " %s:\n", contentType)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(t.file, "   %s\n", line)
		}
	}
}

func (t *Trace) logAIMessage(msg ai.AIMessage) {
	t.logMessageContent("content", msg.Content)
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(t.file, " tool request:\n")
			fmt.Fprintf(t.file, "   tool_call_id: %s\n", tc.ID)
			fmt.Fprintf(t.file, "   tool_name: %s\n", tc.Name)
			fmt.Fprintf(t.file, "   tool_args: %s\n", tc.Args)
		}
	}
}

// LLMToolResponse records a single tool call response.
func (t *Trace) LLMToolResponse(agentName string, toolCall *ai.ToolCall, content string) error {
	if err := t.ensureFileInitialized(); err != nil {
		return err
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "🛠️️  %s tool response:\n", agentName)
	fmt.Fprintf(t.file, "   • %s(%s)\n",
		toolCall.Name,
		toolCall.Args)

	// Format the response content
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(t.file, "     %s\n", line)
		}
	}
	t.file.Sync()
	return nil
}

// RecordError records an error that occurred during the interaction.
func (t *Trace) RecordError(err error) error {
	if err := t.ensureFileInitialized(); err != nil {
		return err
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "❌ Error: %v\n", err)
	t.file.Sync()
	return nil
}

// End ends the trace and saves the trace information to a file.
func (t *Trace) Close() error {
	traceSync.Lock()
	defer traceSync.Unlock()

	// If file was never initialized, there's nothing to close
	if !t.fileInitialized {
		return nil
	}

	t.EndTime = time.Now()
	fmt.Fprintf(t.file, "End Time: %s\n", t.EndTime.Format(time.RFC3339))

	return t.file.Close()
}
