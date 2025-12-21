package run

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
	"github.com/nexxia-ai/aigentic/event"
)

var (
	traceSync = sync.Mutex{}
)

type TraceConfig struct {
	Directory         string
	RetentionDuration time.Duration
	MaxTraceFiles     int
}

type Tracer struct {
	config  TraceConfig
	counter int64
}

type TraceRun struct {
	tracer    *Tracer
	startTime time.Time
	endTime   time.Time
	filepath  string
	file      traceWriter
}

type traceWriter interface {
	io.Writer
	Sync() error
	Close() error
}

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

const (
	defaultRetentionDuration = 7 * 24 * time.Hour
	defaultMaxTraceFiles     = 10
)

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

	os.MkdirAll(cfg.Directory, 0755)

	return t
}

func (tr *Tracer) NewTraceRun() *TraceRun {
	timestamp := time.Now().Format("20060102150405")
	counter := atomic.AddInt64(&tr.counter, 1)
	filepath := filepath.Join(tr.config.Directory, fmt.Sprintf("trace-%s.%03d.txt", timestamp, counter))

	tr.cleanup()

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

	sort.Slice(traceFiles, func(i, j int) bool {
		return traceFiles[i].modTime.Before(traceFiles[j].modTime)
	})

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

func (tr *TraceRun) Filepath() string {
	return tr.filepath
}

func (tr *TraceRun) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "\n====> [%s] Start %s (%s) runID: %s\n", time.Now().Format("15:04:05"),
		run.AgentName(), run.Model().ModelName, run.ID())

	for _, message := range messages {
		role, _ := message.Value()

		switch msg := message.(type) {
		case ai.UserMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
			tr.logMessageContent("content", msg.Content)
		case ai.SystemMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
			tr.logMessageContent("content", msg.Content)
		case ai.AIMessage:
			fmt.Fprintf(tr.file, "⬆️  assistant: role=%s\n", msg.Role)
			tr.logAIMessage(msg)
		case ai.ToolMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
			fmt.Fprintf(tr.file, " tool_call_id: %s\n", msg.ToolCallID)
			tr.logMessageContent("content", msg.Content)
		case ai.ResourceMessage:
			fmt.Fprintf(tr.file, "⬆️  %s:\n", role)
			var isFileID bool
			var contentLen int
			var contentPreview string

			if body, ok := msg.Body.([]byte); ok && body != nil {
				isFileID = false
				contentLen = len(body)
				if contentLen > 0 {
					previewLen := 64
					if contentLen < previewLen {
						previewLen = contentLen
					}
					contentPreview = string(body[:previewLen])
				}
			} else {
				isFileID = true
				contentLen = len(msg.Name)
			}

			if isFileID {
				fmt.Fprintf(tr.file, " resource: %s (file ID reference)\n", msg.Name)
			} else {
				fmt.Fprintf(tr.file, " resource: %s (content length: %d)\n", msg.Name, contentLen)
			}

			if msg.URI != "" {
				fmt.Fprintf(tr.file, " uri: %s\n", msg.URI)
			}
			if msg.MIMEType != "" {
				fmt.Fprintf(tr.file, " mime_type: %s\n", msg.MIMEType)
			}
			if msg.Description != "" {
				fmt.Fprintf(tr.file, " description: %s\n", msg.Description)
			}

			if contentPreview != "" {
				tr.logMessageContent("content_preview", contentPreview)
			}
		default:
			_, content := message.Value()
			tr.logMessageContent("content", content)
		}
	}

	tr.file.Sync()
	return messages, tools, nil
}

func (tr *TraceRun) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "⬇️  assistant: role=%s\n", response.Role)
	tr.logAIMessage(response)
	tr.file.Sync()

	fmt.Fprintf(tr.file, "==== [%s] End %s\n\n", time.Now().Format("15:04:05"), run.AgentName())

	return response, nil
}

func (tr *TraceRun) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult) (event.ValidationResult, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "\n---- Tool START: %s (callID=%s) agent=%s\n", toolName, toolCallID, run.AgentName())

	argsJSON, _ := json.Marshal(validationResult)
	fmt.Fprintf(tr.file, " args: %s\n", string(argsJSON))
	tr.file.Sync()

	return validationResult, nil
}

func (tr *TraceRun) AfterToolCall(run *AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
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
	fmt.Fprintf(tr.file, "🛠️️  %s tool response:\n", run.AgentName())
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

func (tr *TraceRun) LLMToolResponse(agentName string, toolCall *ai.ToolCall, content string) error {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "🛠️️  %s tool response:\n", agentName)
	fmt.Fprintf(tr.file, "   • %s(%s)\n",
		toolCall.Name,
		toolCall.Args)

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(tr.file, "     %s\n", line)
		}
	}
	tr.file.Sync()
	return nil
}

func (tr *TraceRun) RecordError(err error) error {
	traceSync.Lock()
	defer traceSync.Unlock()

	fmt.Fprintf(tr.file, "❌ Error: %v\n", err)
	tr.file.Sync()
	return nil
}

func (tr *TraceRun) Close() error {
	traceSync.Lock()
	defer traceSync.Unlock()

	tr.endTime = time.Now()
	fmt.Fprintf(tr.file, "End Time: %s\n", tr.endTime.Format(time.RFC3339))
	tr.file.Sync()

	return tr.file.Close()
}
