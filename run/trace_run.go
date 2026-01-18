package run

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
)

type Trace interface {
	Interceptor
	RecordError(err error) error
	Close() error
	Filepath() string
}

type TraceRun struct {
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

func (tr *TraceRun) BeforeCall(run *AgentRun, messages []ai.Message, tools []ai.Tool) ([]ai.Message, []ai.Tool, error) {

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "\n====> [%s] Start %s (%s) runID: %s\n", time.Now().Format("15:04:05"),
			run.AgentName(), run.Model().ModelName, run.ID())

		for _, message := range messages {
			role, _ := message.Value()

			switch msg := message.(type) {
			case ai.UserMessage:
				fmt.Fprintf(w, "⬆️  %s:\n", role)
				if len(msg.Parts) > 0 {
					for i, part := range msg.Parts {
						fmt.Fprintf(w, " part[%d]: type=%s", i, part.Type)
						if part.Type == ai.ContentPartText {
							tr.logMessageContentToWriter(w, " text", part.Text)
						} else if part.Name != "" {
							fmt.Fprintf(w, " name=%s", part.Name)
						}
						fmt.Fprintf(w, "\n")
					}
				} else {
					tr.logMessageContentToWriter(w, "content", msg.Content)
				}
			case ai.SystemMessage:
				fmt.Fprintf(w, "⬆️  %s:\n", role)
				if len(msg.Parts) > 0 {
					for i, part := range msg.Parts {
						fmt.Fprintf(w, " part[%d]: type=%s", i, part.Type)
						if part.Type == ai.ContentPartText {
							tr.logMessageContentToWriter(w, " text", part.Text)
						} else if part.Name != "" {
							fmt.Fprintf(w, " name=%s", part.Name)
						}
						fmt.Fprintf(w, "\n")
					}
				} else {
					tr.logMessageContentToWriter(w, "content", msg.Content)
				}
			case ai.AIMessage:
				fmt.Fprintf(w, "⬆️  assistant: role=%s\n", msg.Role)
				tr.logAIMessageToWriter(w, msg)
			case ai.ToolMessage:
				fmt.Fprintf(w, "⬆️  %s:\n", role)
				fmt.Fprintf(w, " tool_call_id: %s\n", msg.ToolCallID)
				tr.logMessageContentToWriter(w, "content", msg.Content)
			default:
				_, content := message.Value()
				tr.logMessageContentToWriter(w, "content", content)
			}
		}
	})

	return messages, tools, nil
}

func (tr *TraceRun) AfterCall(run *AgentRun, request []ai.Message, response ai.AIMessage) (ai.AIMessage, error) {

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "⬇️  assistant: role=%s\n", response.Role)
		tr.logAIMessageToWriter(w, response)
		fmt.Fprintf(w, "==== [%s] End %s\n\n", time.Now().Format("15:04:05"), run.AgentName())
	})

	return response, nil
}

func (tr *TraceRun) BeforeToolCall(run *AgentRun, toolName string, toolCallID string, args map[string]any) (map[string]any, error) {

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "\n---- Tool START: %s (callID=%s) agent=%s\n", toolName, toolCallID, run.AgentName())
		argsJSON, _ := json.Marshal(args)
		fmt.Fprintf(w, " args: %s\n", string(argsJSON))
	})

	return args, nil
}

func (tr *TraceRun) AfterToolCall(run *AgentRun, toolName string, toolCallID string, args map[string]any, result *ai.ToolResult) (*ai.ToolResult, error) {

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

		// argsJSON, _ := json.Marshal(args)
		// fmt.Fprintf(w, "🛠️️  %s tool response:\n", run.AgentName())
		// fmt.Fprintf(w, "   • %s(%s)\n", toolName, string(argsJSON))

		// lines := strings.Split(response, "\n")
		// for _, line := range lines {
		// 	if line != "" {
		// 		fmt.Fprintf(w, "     %s\n", line)
		// 	}
		// }
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

	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "❌ Error: %v\n", err)
	})
	return nil
}

func (tr *TraceRun) Close() error {

	tr.endTime = time.Now()
	tr.writeToFile(func(w io.Writer) {
		fmt.Fprintf(w, "End Time: %s\n", tr.endTime.Format(time.RFC3339))
	})
	return nil
}
