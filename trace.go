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
	"sync/atomic"
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
	Directory         string
	RetentionDuration time.Duration
	MaxTraceFiles     int
}

// Tracer is a factory that creates TraceRun instances for agent runs
type Tracer struct {
	config  TraceConfig
	counter int64
}

// TraceRun stores the execution trace of an LLM for a single run.
// TraceRun implements Interceptor for automatic tracing via interceptors.
type TraceRun struct {
	tracer    *Tracer
	startTime time.Time   // Start time of the trace
	endTime   time.Time   // End time of the trace
	filepath  string      // Path to the trace file
	file      traceWriter // File to write traces to (or io.Discard if file creation fails)
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

// NewTracer creates a new Tracer factory with default cleanup settings.
func NewTracer(config ...TraceConfig) *Tracer {
	defaultDir := filepath.Join(os.TempDir(), "aigentic-traces")

	cfg := TraceConfig{
		Directory:         defaultDir,
		RetentionDuration: defaultRetentionDuration,
		MaxTraceFiles:     defaultMaxTraceFiles,
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
	}

	t := &Tracer{
		config:  cfg,
		counter: 0,
	}

	// Create directory if needed
	os.MkdirAll(cfg.Directory, 0755)

	return t
}

// NewTraceRun creates a new TraceRun for a specific agent run
func (tr *Tracer) NewTraceRun() *TraceRun {

	// Generate timestamp and counter for unique filename
	timestamp := time.Now().Format("20060102150405")
	counter := atomic.AddInt64(&tr.counter, 1)
	filepath := filepath.Join(tr.config.Directory, fmt.Sprintf("trace-%s.%03d.txt", timestamp, counter))

	tr.cleanup() // Clean old files based on tracer config

	traceRun := &TraceRun{
		tracer:    tr,
		startTime: time.Now(),
		filepath:  filepath,
	}

	var file traceWriter
	osFile, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Failed to open trace file, using io.Discard", "file", filepath, "error", err)
		file = &discardWriter{}
	} else {
		file = osFile
	}

	traceRun.file = file
	return traceRun
}

// cleanup removes old trace files based on retention policy
func (tr *Tracer) cleanup() {
	entries, err := os.ReadDir(tr.config.Directory)
	if err != nil {
		slog.Error("Failed to read trace directory", "error", err)
		return
	}

	var traceFiles []struct {
		path    string
		modTime time.Time
	}

	cutoffTime := time.Now().Add(-tr.config.RetentionDuration)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "trace-") || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		filePath := filepath.Join(tr.config.Directory, entry.Name())
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
	if tr.config.RetentionDuration > 0 {
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
	if tr.config.MaxTraceFiles > 0 && len(traceFiles) > tr.config.MaxTraceFiles {
		filesToRemove := len(traceFiles) - tr.config.MaxTraceFiles
		for i := 0; i < filesToRemove && i < len(traceFiles); i++ {
			if err := os.Remove(traceFiles[i].path); err != nil {
				slog.Error("Failed to remove excess trace file", "file", traceFiles[i].path, "error", err)
			} else {
				slog.Debug("Removed excess trace file", "file", filepath.Base(traceFiles[i].path))
			}
		}
	}
}

// Filepath returns the path to the trace file
func (tr *TraceRun) Filepath() string {
	return tr.filepath
}

// BeforeCall implements Interceptor - records LLM call before invocation
func (tr *TraceRun) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "\n====> [%s] Start %s (%s) runID: %s\n", time.Now().Format("15:04:05"),
		run.agent.Name, run.model.ModelName, run.ID())

	for _, message := range messages {
		role, _ := message.Value()

		// Handle each message type specifically
		switch msg := message.(type) {
		case ai.UserMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
			tr.logMessageContent("content", msg.Content)
		case ai.SystemMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
			tr.logMessageContent("content", msg.Content)
		case ai.AIMessage:
			fmt.Fprintf(tr.file, "⬆️  assistant: role=%s\n", msg.Role) // Role might vary by provider
			tr.logAIMessage(msg)
		case ai.ToolMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
			fmt.Fprintf(tr.file, " tool_call_id: %s\n", msg.ToolCallID)
			tr.logMessageContent("content", msg.Content)
		case ai.ResourceMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
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
				fmt.Fprintf(tr.file, " resource: %s (file ID reference)\n", msg.Name)
			} else {
				fmt.Fprintf(tr.file, " resource: %s (content length: %d)\n", msg.Name, contentLen)
			}

			// Log additional metadata
			if msg.URI != "" {
				fmt.Fprintf(tr.file, " uri: %s\n", msg.URI)
			}
			if msg.MIMEType != "" {
				fmt.Fprintf(tr.file, " mime_type: %s\n", msg.MIMEType)
			}
			if msg.Description != "" {
				fmt.Fprintf(tr.file, " description: %s\n", msg.Description)
			}

			// Log content preview if available
			if contentPreview != "" {
				tr.logMessageContent("content_preview", contentPreview)
			}
		default:
			// Fallback for unknown message types
			_, content := message.Value()
			tr.logMessageContent("content", content)
		}
	}

	tr.file.Sync()
	return messages, tools, nil
}

// AfterCall implements Interceptor - records LLM response after invocation
func (tr *TraceRun) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "⬇️  assistant: role=%s\n", response.Role) // Role might vary by provider
	tr.logAIMessage(response)
	tr.file.Sync()

	fmt.Fprintf(tr.file, "==== [%s] End %s\n\n", time.Now().Format("15:04:05"), run.agent.Name)

	return response, nil
}

// BeforeToolCall implements Interceptor - records tool call before execution
func (tr *TraceRun) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult) (ValidationResult, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "\n---- Tool START: %s (callID=%s) agent=%s\n", toolName, toolCallID, run.agent.Name)

	argsJSON, _ := json.Marshal(validationResult)
	fmt.Fprintf(tr.file, " args: %s\n", string(argsJSON))
	tr.file.Sync()

	return validationResult, nil
}

// AfterToolCall implements Interceptor - records tool call after execution
func (tr *TraceRun) AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
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

	fmt.Fprintf(tr.file, " result: %s\n", response)
	fmt.Fprintf(tr.file, "---- Tool END: %s (callID=%s)\n", toolName, toolCallID)

	argsJSON, _ := json.Marshal(validationResult)
	fmt.Fprintf(tr.file, "🛠️️  %s tool response:\n", run.agent.Name)
	fmt.Fprintf(tr.file, "   • %s(%s)\n", toolName, string(argsJSON))

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(tr.file, "     %s\n", line)
		}
	}
	tr.file.Sync()

	return result, nil
}

// logMessageContent is a helper method to format and log message content
func (tr *TraceRun) logMessageContent(contentType, content string) {
	if content == "" {
		fmt.Fprintf(tr.file, " %s: (empty)\n", contentType)
		return
	}

	fmt.Fprintf(tr.file, " %s:\n", contentType)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(tr.file, "   %s\n", line)
		}
	}
}

func (tr *TraceRun) logAIMessage(msg ai.AIMessage) {
	tr.logMessageContent("content", msg.Content)
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(tr.file, " tool request:\n")
			fmt.Fprintf(tr.file, "   tool_call_id: %s\n", tc.ID)
			fmt.Fprintf(tr.file, "   tool_name: %s\n", tc.Name)
			fmt.Fprintf(tr.file, "   tool_args: %s\n", tc.Args)
		}
	}
}

// LLMToolResponse records a single tool call response.
func (tr *TraceRun) LLMToolResponse(agentName string, toolCall *ai.ToolCall, content string) error {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "🛠️️  %s tool response:\n", agentName)
	fmt.Fprintf(tr.file, "   • %s(%s)\n",
		toolCall.Name,
		toolCall.Args)

	// Format the response content
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(tr.file, "     %s\n", line)
		}
	}
	tr.file.Sync()
	return nil
}

// RecordError records an error that occurred during the interaction.
func (tr *TraceRun) RecordError(err error) error {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "❌ Error: %v\n", err)
	tr.file.Sync()
	return nil
}

// End ends the trace and saves the trace information to a file.
func (tr *TraceRun) Close() error {
	traceSync.Lock()
	defer traceSync.Unlock()

	tr.endTime = time.Now()
	fmt.Fprintf(tr.file, "End Time: %s\n", tr.endTime.Format(time.RFC3339))
	tr.file.Sync()

	return tr.file.Close()
}
