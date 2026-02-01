package ctxt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexxia-ai/aigentic/document"
)

type Session struct {
	ID      string
	Name    string
	Summary string
	Path    string
}

func ListSessions(baseDir string) ([]Session, error) {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	entries, err := os.ReadDir(absBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if !strings.HasPrefix(entry.Name(), "agent-") {
			continue
		}

		sessionDir := filepath.Join(absBaseDir, entry.Name())
		contextFile := filepath.Join(sessionDir, "_private", "context.json")

		if _, err := os.Stat(contextFile); os.IsNotExist(err) {
			continue
		}

		var data contextData
		file, err := os.Open(contextFile)
		if err != nil {
			continue
		}

		if err := json.NewDecoder(file).Decode(&data); err != nil {
			file.Close()
			continue
		}
		file.Close()

		sessions = append(sessions, Session{
			ID:      data.ID,
			Name:    data.Name,
			Summary: data.Summary,
			Path:    sessionDir,
		})
	}

	return sessions, nil
}

func LoadContext(sessionDir string) (*AgentContext, error) {
	absSessionDir, err := filepath.Abs(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	execEnv, err := loadExecutionEnvironment(absSessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load execution environment: %w", err)
	}

	contextFile := filepath.Join(execEnv.PrivateDir, "context.json")
	file, err := os.Open(contextFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open context file: %w", err)
	}
	defer file.Close()

	var data contextData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode context: %w", err)
	}

	execEnv.MemoryDir = data.MemoryDir

	ctx := &AgentContext{
		id:                 data.ID,
		description:        data.Description,
		instructions:       data.Instructions,
		name:               data.Name,
		summary:            data.Summary,
		outputInstructions: data.OutputInstructions,
		memories:           data.Memories,
		turnCounter:        data.TurnCounter,
		documentReferences: make([]*document.Document, 0),
		execEnv:            execEnv,
	}

	loadRunMeta(ctx, execEnv.PrivateDir)
	ctx.conversationHistory = NewConversationHistory(ctx.execEnv)
	ctx.UpdateSystemTemplate(DefaultSystemTemplate)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	ctx.currentTurn = ctx.newTurn()

	return ctx, nil
}
