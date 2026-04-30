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
	Meta    map[string]interface{}
	Turns   int
}

func deriveBasePath(runDir string) string {
	parentDir := filepath.Dir(runDir)
	grandparentDir := filepath.Dir(parentDir)
	if filepath.Base(parentDir) == "runs" {
		return grandparentDir
	}
	if filepath.Base(grandparentDir) == "runs" {
		return filepath.Dir(grandparentDir)
	}
	return parentDir
}

func sessionRunDirs(baseDir string) ([]string, error) {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	runDirs := make([]string, 0)
	runsDir := filepath.Join(absBaseDir, "runs")
	if entries, err := os.ReadDir(runsDir); err == nil {
		for _, shardEntry := range entries {
			if !shardEntry.IsDir() {
				continue
			}
			shardDir := filepath.Join(runsDir, shardEntry.Name())
			if _, err := os.Stat(filepath.Join(shardDir, aigenticDirName, "context.json")); err == nil {
				runDirs = append(runDirs, shardDir)
				continue
			}
			runEntries, err := os.ReadDir(shardDir)
			if err != nil {
				continue
			}
			for _, runEntry := range runEntries {
				if !runEntry.IsDir() {
					continue
				}
				runDirs = append(runDirs, filepath.Join(shardDir, runEntry.Name()))
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read runs directory: %w", err)
	}

	return runDirs, nil
}

func ListSessions(baseDir string) ([]Session, error) {
	runDirs, err := sessionRunDirs(baseDir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, runDir := range runDirs {
		session, err := loadSession(runDir)
		if err != nil {
			continue
		}
		sessions = append(sessions, *session)
	}

	return sessions, nil
}

func FindSession(baseDir, runID string) (*Session, error) {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	if shard := RunIDShard(runID); shard != "" {
		session, err := loadSession(RunDir(absBaseDir, runID))
		if err == nil && session.ID == runID {
			return session, nil
		}
	}

	sessions, err := ListSessions(absBaseDir)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if session.ID == runID {
			return &session, nil
		}
	}
	return nil, fmt.Errorf("run not found: %s", runID)
}

func LoadContext(runDir string) (*AgentContext, error) {
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	ws, err := loadWorkspace(absRunDir)
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

	basePath := deriveBasePath(absRunDir)

	ctx := &AgentContext{
		id:          data.ID,
		name:        data.Name,
		summary:     data.Summary,
		systemParts: data.SystemParts,
		workspace:   ws,
		basePath:    basePath,
		ledger:      NewLedger(basePath),
		enableTrace: data.EnableTrace,
	}

	loadRunMeta(ctx, ws.PrivateDir)
	conversationPath := filepath.Join(ws.PrivateDir, "conversation.json")
	ctx.conversationHistory = NewConversationHistory(ctx.ledger, conversationPath)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	ctx.currentTurn = NewTurn(ctx, "", "", "", "")

	return ctx, nil
}

// skipJSONValue advances the decoder past one complete JSON value.
func skipJSONValue(d *json.Decoder) error {
	tok, err := d.Token()
	if err != nil {
		return err
	}
	del, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch del {
	case '{':
		for d.More() {
			keyTok, err := d.Token()
			if err != nil {
				return err
			}
			if _, ok := keyTok.(string); !ok {
				return fmt.Errorf("ctxt: context.json: expected string object key")
			}
			if err := skipJSONValue(d); err != nil {
				return err
			}
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		if del, ok := tok.(json.Delim); !ok || del != '}' {
			return fmt.Errorf("ctxt: context.json: expected end of object")
		}
		return nil
	case '[':
		for d.More() {
			if err := skipJSONValue(d); err != nil {
				return err
			}
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		if del, ok := tok.(json.Delim); !ok || del != ']' {
			return fmt.Errorf("ctxt: context.json: expected end of array")
		}
		return nil
	default:
		return fmt.Errorf("ctxt: skipJSONValue unexpected delimiter %q", del)
	}
}

func decodeContextJSONForSession(d *json.Decoder) (id, name, summary string, err error) {
	tok, err := d.Token()
	if err != nil {
		return "", "", "", err
	}
	del, ok := tok.(json.Delim)
	if !ok || del != '{' {
		return "", "", "", fmt.Errorf("ctxt: context.json: expected object")
	}
	for d.More() {
		keyTok, err := d.Token()
		if err != nil {
			return "", "", "", err
		}
		key, ok := keyTok.(string)
		if !ok {
			return "", "", "", fmt.Errorf("ctxt: context.json: expected string object key")
		}
		switch key {
		case "id":
			var s string
			if err := d.Decode(&s); err != nil {
				return "", "", "", err
			}
			id = s
		case "name":
			var s string
			if err := d.Decode(&s); err != nil {
				return "", "", "", err
			}
			name = s
		case "summary":
			var s string
			if err := d.Decode(&s); err != nil {
				return "", "", "", err
			}
			summary = s
		case "enable_trace":
			var disc bool
			if err := d.Decode(&disc); err != nil {
				return "", "", "", err
			}
		default:
			if err := skipJSONValue(d); err != nil {
				return "", "", "", err
			}
		}
	}
	tok, err = d.Token()
	if err != nil {
		return "", "", "", err
	}
	del, ok = tok.(json.Delim)
	if !ok || del != '}' {
		return "", "", "", fmt.Errorf("ctxt: context.json: expected end of object")
	}
	return id, name, summary, nil
}

func loadSession(runDir string) (*Session, error) {
	privateDir := filepath.Join(runDir, aigenticDirName)
	contextFile := filepath.Join(privateDir, "context.json")
	file, err := os.Open(contextFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	id, name, summary, err := decodeContextJSONForSession(dec)
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:      id,
		Name:    name,
		Summary: summary,
		Path:    runDir,
	}
	if err := loadSessionRunMeta(session, privateDir); err != nil {
		return nil, err
	}
	if refs, err := LoadConversationRefs(filepath.Join(privateDir, "conversation.json")); err != nil {
		return nil, err
	} else {
		session.Turns = len(refs)
	}
	return session, nil
}

func loadSessionRunMeta(session *Session, privateDir string) error {
	if session == nil {
		return fmt.Errorf("session is required")
	}
	data, err := os.ReadFile(filepath.Join(privateDir, "run_meta.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	session.Meta = meta
	return nil
}
