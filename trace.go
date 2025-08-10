package aigentic

import (
	"fmt"
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
	defaultTraceDirectory    = "traces"
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
type Trace struct {
	SessionID         string        // Unique session ID for the entire interaction
	StartTime         time.Time     // Start time of the trace
	EndTime           time.Time     // End time of the trace
	directory         string        // Path to the trace directory
	filename          string        // Path to the trace file
	file              *os.File      // File to write traces to
	RetentionDuration time.Duration // How long to keep traces
	MaxTraceFiles     int           // Maximum number of files to keep
}

func newTraceID() string {
	now := time.Now()
	return now.Format("20060102150405") + fmt.Sprintf("%09d", now.Nanosecond())
}

// NewTrace creates a new Trace instance with default cleanup settings.
func NewTrace(config ...TraceConfig) *Trace {
	cfg := TraceConfig{
		Directory:         defaultTraceDirectory,
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
			return nil
		}
	}

	filename := filepath.Join(cfg.Directory, fmt.Sprintf("trace-%s.txt", cfg.SessionID))

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil
	}

	t := &Trace{
		SessionID:         cfg.SessionID,
		StartTime:         time.Now(),
		filename:          filename,
		file:              file,
		directory:         cfg.Directory,
		RetentionDuration: cfg.RetentionDuration,
		MaxTraceFiles:     cfg.MaxTraceFiles,
	}
	t.Cleanup()

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

// LLMCall records the initial interaction with the LLM (model and messages).
func (t *Trace) LLMCall(modelName, agentName string, messages []ai.Message) error {
	if t == nil {
		return nil
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "\n====> [%s] Start %s (%s) sessionID: %s\n", time.Now().Format("15:04:05"), agentName, modelName, t.SessionID)

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
	return nil
}

// LLMAIResponse records the LLM's response, any tool calls made during the response, and any thinking process.
func (t *Trace) LLMAIResponse(agentName string, msg ai.AIMessage) {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "⬇️  assistant: role=%s\n", msg.Role) // Role might vary by provider
	t.logAIMessage(msg)
	t.file.Sync()
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

// FinishLLMInteraction adds a closing line to mark the end of an LLM interaction
func (t *Trace) FinishLLMInteraction(modelName, agentName string) {
	if t == nil {
		return
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "==== [%s] End %s\n\n", time.Now().Format("15:04:05"), agentName)
}
func (t *Trace) logAIMessage(msg ai.AIMessage) {
	t.logMessageContent("content", msg.Content)
	if msg.Think != "" {
		t.logMessageContent("thinking", msg.Think)
	}
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
	if t == nil {
		return nil
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
	if t == nil {
		return nil
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(t.file, "❌ Error: %v\n", err)
	t.file.Sync()
	return nil
}

// End ends the trace and saves the trace information to a file.
func (t *Trace) Close() error {
	if t == nil {
		return nil
	}

	traceSync.Lock()
	defer traceSync.Unlock()

	t.EndTime = time.Now()

	_, err := fmt.Fprintf(t.file, "End Time: %s\n", t.EndTime.Format(time.RFC3339))
	if err != nil {
		return err
	}

	err = t.file.Close()
	if err != nil {
		return err
	}

	slog.Debug("Trace saved", "file", t.filename)
	return nil
}
