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

func loadShardIndexLatest(shardDir string) (map[string]shardIndexLine, error) {
	indexPath := filepath.Join(shardDir, ShardIndexFileName)
	latest := make(map[string]shardIndexLine)
	err := readJSONLLines(indexPath, func(line []byte, _ int, _ bool) error {
		var row shardIndexLine
		if err := json.Unmarshal(line, &row); err != nil || row.TurnID == "" {
			return nil
		}
		latest[row.TurnID] = row
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return latest, nil
		}
		return nil, err
	}
	return latest, nil
}

// ListTurnArtifactsWithFeedback scans the ledger for a single UTC calendar day and returns turns
// that have feedback or feedback_comment metadata.
func ListTurnArtifactsWithFeedback(baseDir string, day time.Time, limit int) ([]TurnArtifact, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	shard := day.In(time.UTC).Format("20060102")
	shardDir := filepath.Join(absBase, ledgerDir, shard)
	latest, err := loadShardIndexLatest(shardDir)
	if err != nil {
		return nil, err
	}
	if len(latest) == 0 {
		entries, err := os.ReadDir(shardDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == ShardIndexFileName {
				continue
			}
			turnID := e.Name()
			if turnIDShard(turnID) != shard {
				continue
			}
			turnDir := filepath.Join(shardDir, turnID)
			if !turnHeadExists(turnDir) {
				continue
			}
			t, err := loadTurnHead(turnDir, turnID)
			if err != nil {
				continue
			}
			latest[turnID] = shardIndexLine{
				Version:     storageVersion,
				TurnID:      turnID,
				RunID:       t.RunID,
				Timestamp:   t.Timestamp,
				AgentName:   t.AgentName,
				HasFeedback: hasFeedbackMeta(t.Meta()),
			}
		}
	}
	var artifacts []TurnArtifact
	for turnID, row := range latest {
		if !row.HasFeedback {
			metaPath := filepath.Join(shardDir, turnID, "meta.json")
			meta, err := loadMetaFromPath(metaPath)
			if err != nil || !hasFeedback(meta) {
				continue
			}
		}
		if len(artifacts) >= limit {
			return nil, ErrFeedbackTurnLimitExceeded
		}
		turnDir := filepath.Join(shardDir, turnID)
		t, err := loadTurnHead(turnDir, turnID)
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
	return hasFeedbackMeta(meta)
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
