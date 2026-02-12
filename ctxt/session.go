package ctxt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

		sessionDir := filepath.Join(absBaseDir, entry.Name())
		contextFile := filepath.Join(sessionDir, "_private", "main", "context.json")

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

	ws, err := loadWorkspace(absSessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace: %w", err)
	}

	contextFile := filepath.Join(ws.PrivateDir, "context.json")
	file, err := os.Open(contextFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open context file: %w", err)
	}
	defer file.Close()

	var data contextData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode context: %w", err)
	}

	ws.MemoryDir = data.MemoryDir

	ctx := &AgentContext{
		id:                 data.ID,
		description:        data.Description,
		instructions:       data.Instructions,
		name:               data.Name,
		summary:            data.Summary,
		outputInstructions: data.OutputInstructions,
		turnCounter:        data.TurnCounter,
		workspace:          ws,
		enableTrace:        data.EnableTrace,
	}

	loadRunMeta(ctx, ws.PrivateDir)
	ctx.conversationHistory = NewConversationHistory(ctx.workspace)
	ctx.UpdateSystemTemplate(DefaultSystemTemplate)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	ctx.currentTurn = ctx.newTurn()

	return ctx, nil
}
