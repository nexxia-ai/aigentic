package trace

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/nexxia-ai/aigentic/event"
	"github.com/nexxia-ai/aigentic/run"
)

type TraceRun struct {
	tracer    *Tracer
	startTime time.Time
	endTime   time.Time
	filepath  string
}

func (tr *TraceRun) Filepath() string {
	return tr.filepath
}

func (tr *TraceRun) writeToFile(fn func(io.Writer)) {
	file, err := os.OpenFile(tr.filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Failed to open trace file for writing", "file", tr.filepath, "error", err)
		return
	}
	defer file.Close()

	fn(file)
	file.Sync()
}

func (tr *TraceRun) BeforeCall(run *run.AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "\n====> [%s] Start %s (%s) runID: %s\n", time.Now().Format("15:04:05"),
			run.AgentName(), run.Model().ModelName, run.ID())

		for _, message := range messages {
			role, _ := message.Value()

			switch msg := message.(type) {
			case ai.UserMessage:
				fmt.Fprintf(w, "⬆️  %s:\n", role)
				tr.logMessageContentToWriter(w, "content", msg.Content)
			case ai.SystemMessage:
				fmt.Fprintf(w, "⬆️  %s:\n", role)
				tr.logMessageContentToWriter(w, "content", msg.Content)
			case ai.AIMessage:
				fmt.Fprintf(w, "⬆️  assistant: role=%s\n", msg.Role)
				tr.logAIMessageToWriter(w, msg)
			case ai.ToolMessage:
				fmt.Fprintf(w, "⬆️  %s:\n", role)
				fmt.Fprintf(w, " tool_call_id: %s\n", msg.ToolCallID)
				tr.logMessageContentToWriter(w, "content", msg.Content)
			case ai.ResourceMessage:
				fmt.Fprintf(w, "⬆️  %s:\n", role)
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
					fmt.Fprintf(w, " resource: %s (file ID reference)\n", msg.Name)
				} else {
					fmt.Fprintf(w, " resource: %s (content length: %d)\n", msg.Name, contentLen)
				}

				if msg.URI != "" {
					fmt.Fprintf(w, " uri: %s\n", msg.URI)
				}
				if msg.MIMEType != "" {
					fmt.Fprintf(w, " mime_type: %s\n", msg.MIMEType)
				}
				if msg.Description != "" {
					fmt.Fprintf(w, " description: %s\n", msg.Description)
				}

				if contentPreview != "" {
					tr.logMessageContentToWriter(w, "content_preview", contentPreview)
				}
			default:
				_, content := message.Value()
				tr.logMessageContentToWriter(w, "content", content)
			}
		}
	})

	return messages, tools, nil
}

func (tr *TraceRun) AfterCall(run *run.AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "⬇️  assistant: role=%s\n", response.Role)
		tr.logAIMessageToWriter(w, response)
		fmt.Fprintf(w, "==== [%s] End %s\n\n", time.Now().Format("15:04:05"), run.AgentName())
	})

	return response, nil
}

func (tr *TraceRun) BeforeToolCall(run *run.AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult) (event.ValidationResult, error) {
	traceSync.Lock()
	defer traceSync.Unlock()

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "\n---- Tool START: %s (callID=%s) agent=%s\n", toolName, toolCallID, run.AgentName())
		argsJSON, _ := json.Marshal(validationResult)
		fmt.Fprintf(w, " args: %s\n", string(argsJSON))
	})

	return validationResult, nil
}

func (tr *TraceRun) AfterToolCall(run *run.AgentRun, toolName string, toolCallID string, validationResult event.ValidationResult, result *ai.ToolResult) (*ai.ToolResult, error) {
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

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, " result: %s\n", response)
		fmt.Fprintf(w, "---- Tool END: %s (callID=%s)\n", toolName, toolCallID)

		argsJSON, _ := json.Marshal(validationResult)
		fmt.Fprintf(w, "🛠️️  %s tool response:\n", run.AgentName())
		fmt.Fprintf(w, "   • %s(%s)\n", toolName, string(argsJSON))

		lines := strings.Split(response, "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Fprintf(w, "     %s\n", line)
			}
		}
	})

	return result, nil
}

func (tr *TraceRun) logMessageContentToWriter(w io.Writer, contentType, content string) {
	if content == "" {
		fmt.Fprintf(w, " %s: (empty)\n", contentType)
		return
	}

	fmt.Fprintf(w, " %s:\n", contentType)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Fprintf(w, "   %s\n", line)
		}
	}
}

func (tr *TraceRun) logAIMessageToWriter(w io.Writer, msg ai.AIMessage) {
	tr.logMessageContentToWriter(w, "content", msg.Content)
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(w, " tool request:\n")
			fmt.Fprintf(w, "   tool_call_id: %s\n", tc.ID)
			fmt.Fprintf(w, "   tool_name: %s\n", tc.Name)
			fmt.Fprintf(w, "   tool_args: %s\n", tc.Args)
		}
	}
}

func (tr *TraceRun) RecordError(err error) error {
	traceSync.Lock()
	defer traceSync.Unlock()

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "❌ Error: %v\n", err)
	})
	return nil
}

func (tr *TraceRun) Close() error {
	traceSync.Lock()
	defer traceSync.Unlock()

	tr.endTime = time.Now()
	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "End Time: %s\n", tr.endTime.Format(time.RFC3339))
	})
	return nil
}
