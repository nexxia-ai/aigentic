package ctxt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ErrFeedbackTurnLimitExceeded is returned when a day would exceed the feedback turn limit.
var ErrFeedbackTurnLimitExceeded = errors.New("feedback turn limit exceeded")

// TurnArtifact contains turn ID, run ID, agent name, timestamp, and filtered file refs for feedback export.
type TurnArtifact struct {
	TurnID    string
	RunID     string
	AgentName string
	Timestamp time.Time
	Files     []FileRef
}

// ListTurnArtifactsWithFeedback scans the ledger for a single UTC calendar day and returns turns
// that have feedback or feedback_comment metadata. Operates on one user's baseDir.
// The ledger subdirectory is yyyymmdd for day converted to UTC (same convention as turn IDs).
func ListTurnArtifactsWithFeedback(baseDir string, day time.Time, limit int) ([]TurnArtifact, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	shard := day.In(time.UTC).Format("20060102")
	shardDir := filepath.Join(absBase, ledgerDir, shard)
	entries, err := os.ReadDir(shardDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var artifacts []TurnArtifact
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		turnID := e.Name()
		if turnIDShard(turnID) != shard {
			continue
		}
		metaPath := filepath.Join(shardDir, turnID, "meta.json")
		meta, err := loadMetaFromPath(metaPath)
		if err != nil {
			continue
		}
		if !hasFeedback(meta) {
			continue
		}
		if len(artifacts) >= limit {
			return nil, ErrFeedbackTurnLimitExceeded
		}
		turnPath := filepath.Join(shardDir, turnID, "turn.json")
		t, err := loadTurnFromPath(turnPath, turnID)
		if err != nil {
			continue
		}
		filtered := filterExportableFiles(t.Files)
		artifacts = append(artifacts, TurnArtifact{
			TurnID:    turnID,
			RunID:     t.RunID,
			AgentName: t.AgentName,
			Timestamp: t.Timestamp,
			Files:     filtered,
		})
	}
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Timestamp.Before(artifacts[j].Timestamp)
	})
	return artifacts, nil
}

func loadMetaFromPath(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta map[string]string
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func hasFeedback(meta map[string]string) bool {
	if meta == nil {
		return false
	}
	if v := meta["feedback"]; v != "" {
		return true
	}
	if v := meta["feedback_comment"]; v != "" {
		return true
	}
	return false
}

func loadTurnFromPath(path, turnID string) (*Turn, error) {
	var t Turn
	if err := t.loadFromFile(path); err != nil {
		return nil, err
	}
	t.TurnID = turnID
	return &t, nil
}

func filterExportableFiles(files []FileRef) []FileRef {
	var out []FileRef
	for _, f := range files {
		if f.Ephemeral {
			continue
		}
		if f.IncludeInPrompt {
			out = append(out, f)
			continue
		}
		if f.GetMeta("visible_to_user") == "true" {
			out = append(out, f)
		}
	}
	return out
}
