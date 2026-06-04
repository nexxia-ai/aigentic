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

// ListSessionsOptions configures ListSessions. Zero value means omit archived runs
// (run_meta run_state "inactive").
type ListSessionsOptions struct {
	IncludeArchived bool
}

const (
	sessionRunStateInactive = "inactive"
)

func sessionRunMetaIndicatesArchived(privateDir string) bool {
	data, err := os.ReadFile(filepath.Join(privateDir, "run_meta.json"))
	if err != nil {
		return false
	}
	var probe struct {
		RunState string `json:"run_state"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.RunState == sessionRunStateInactive
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

// runHasLegacyConversationJSON is a fast legacy probe (one stat). Legacy runs
// used monolithic conversation.json; current runs use conversation.log.
func runHasLegacyConversationJSON(runDir string) bool {
	_, err := os.Stat(filepath.Join(runDir, aigenticDirName, "conversation.json"))
	return err == nil
}

// runUsesCurrentStorage fully validates current storage (conversation.log refs
// resolve to ledger head.json). Used only by RepairWorkspace catalog rebuild.
func runUsesCurrentStorage(runDir string) bool {
	privateDir := filepath.Join(runDir, aigenticDirName)
	if _, err := os.Stat(filepath.Join(privateDir, "context.json")); err != nil {
		return false
	}
	if runHasLegacyConversationJSON(runDir) {
		return false
	}
	refs, err := LoadConversationRefs(conversationPathForPrivateDir(privateDir))
	if err != nil {
		return false
	}
	if len(refs) == 0 {
		return true
	}
	ledger := NewLedger(deriveBasePath(runDir))
	for _, turnID := range refs {
		dir := ledger.TurnDir(turnID)
		if dir == "" || !turnHeadExists(dir) {
			return false
		}
	}
	return true
}

func sessionRunDirs(baseDir string) ([]string, error) {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	runDirs := make([]string, 0)
	runsDir := filepath.Join(absBaseDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return runDirs, nil
		}
		return nil, fmt.Errorf("failed to read runs directory: %w", err)
	}
	for _, shardEntry := range entries {
		if !shardEntry.IsDir() {
			continue
		}
		shardDir := filepath.Join(runsDir, shardEntry.Name())
		runEntries, err := os.ReadDir(shardDir)
		if err != nil {
			continue
		}
		for _, runEntry := range runEntries {
			if !runEntry.IsDir() {
				continue
			}
			privateDir := filepath.Join(shardDir, runEntry.Name(), aigenticDirName)
			if _, err := os.Stat(filepath.Join(privateDir, "context.json")); err != nil {
				continue
			}
			runDir := filepath.Join(shardDir, runEntry.Name())
			if runHasLegacyConversationJSON(runDir) {
				continue
			}
			runDirs = append(runDirs, runDir)
		}
	}
	return runDirs, nil
}

func listSessionsFromCatalog(baseDir string, includeArchived bool) ([]Session, error) {
	entries, err := materializeCatalog(baseDir)
	if err != nil {
		return nil, err
	}
	var sessions []Session
	for _, e := range entries {
		if !includeArchived && e.RunState == sessionRunStateInactive {
			continue
		}
		if !catalogEntryListable(e) {
			continue
		}
		sessions = append(sessions, catalogEntryToSession(e))
	}
	return sessions, nil
}

func ListSessions(baseDir string, opts ...ListSessionsOptions) ([]Session, error) {
	includeArchived := false
	if len(opts) > 0 {
		includeArchived = opts[0].IncludeArchived
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	if catalogMissing(absBase) {
		if _, err := os.Stat(runsDir(absBase)); os.IsNotExist(err) {
			return []Session{}, nil
		}
		if err := RepairWorkspace(absBase); err != nil {
			return nil, err
		}
	}
	sessions, err := listSessionsFromCatalog(absBase, includeArchived)
	if err != nil {
		if err2 := RepairWorkspace(absBase); err2 != nil {
			return nil, err
		}
		return listSessionsFromCatalog(absBase, includeArchived)
	}
	return sessions, nil
}

func FindSession(baseDir, runID string) (*Session, error) {
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	if RunIDShard(runID) == "" {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	runDir := RunDir(absBaseDir, runID)
	if _, err := os.Stat(runDir); os.IsNotExist(err) {
		_ = RepairWorkspace(absBaseDir)
		if _, err := os.Stat(runDir); os.IsNotExist(err) {
			return nil, fmt.Errorf("run not found: %s", runID)
		}
	}
	if runHasLegacyConversationJSON(runDir) {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	session, err := loadSession(runDir)
	if err != nil || session.ID != runID {
		return nil, fmt.Errorf("run not found: %s", runID)
	}
	return session, nil
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
	conversationPath := conversationPathForPrivateDir(ws.PrivateDir)
	ctx.conversationHistory = NewConversationHistory(ctx.ledger, conversationPath)
	ctx.UpdateUserTemplate(DefaultUserTemplate)
	ctx.currentTurn = NewTurn(ctx, "", "", "", "")

	return ctx, nil
}

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
		Turns:   conversationTurnCount(privateDir),
	}
	if err := loadSessionRunMeta(session, privateDir); err != nil {
		return nil, err
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
