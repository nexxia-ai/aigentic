package ctxt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// RepairWorkspace rebuilds runs/catalog.snapshot.json, runs/catalog.log, and ledger shard index files
// from run and turn directories. Idempotent. Does not remove llm artifacts.
func RepairWorkspace(basePath string) error {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return err
	}
	if err := repairShardIndexes(abs); err != nil {
		return err
	}
	if err := repairConversationLogs(abs); err != nil {
		return err
	}
	if err := repairRunCatalog(abs); err != nil {
		return err
	}
	invalidateCatalogCache()
	return nil
}

func repairShardIndexes(basePath string) error {
	ledgerRoot := filepath.Join(basePath, ledgerDir)
	shards, err := os.ReadDir(ledgerRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, shardEntry := range shards {
		if !shardEntry.IsDir() {
			continue
		}
		shardDir := filepath.Join(ledgerRoot, shardEntry.Name())
		indexPath := filepath.Join(shardDir, ShardIndexFileName)
		_ = os.Remove(indexPath)
		entries, err := os.ReadDir(shardDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == ShardIndexFileName {
				continue
			}
			turnID := e.Name()
			if turnIDShard(turnID) != shardEntry.Name() {
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
			_ = appendShardIndex(shardDir, t)
		}
	}
	return nil
}

func repairConversationLogs(basePath string) error {
	runDirs, err := sessionRunDirs(basePath)
	if err != nil {
		return err
	}
	ledgerRoot := filepath.Join(basePath, ledgerDir)
	for _, runDir := range runDirs {
		privateDir := filepath.Join(runDir, aigenticDirName)
		f, err := os.Open(filepath.Join(privateDir, "context.json"))
		if err != nil {
			continue
		}
		dec := json.NewDecoder(f)
		runID, _, _, err := decodeContextJSONForSession(dec)
		_ = f.Close()
		if err != nil || runID == "" {
			continue
		}
		type turnRef struct {
			id string
			ts time.Time
		}
		var refs []turnRef
		shards, err := os.ReadDir(ledgerRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, shardEntry := range shards {
			if !shardEntry.IsDir() {
				continue
			}
			shardDir := filepath.Join(ledgerRoot, shardEntry.Name())
			entries, err := os.ReadDir(shardDir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() || e.Name() == ShardIndexFileName {
					continue
				}
				turnID := e.Name()
				if turnIDShard(turnID) != shardEntry.Name() {
					continue
				}
				turnDir := filepath.Join(shardDir, turnID)
				if !turnHeadExists(turnDir) {
					continue
				}
				t, err := loadTurnHead(turnDir, turnID)
				if err != nil || t.Hidden {
					continue
				}
				if t.RunID != runID {
					continue
				}
				refs = append(refs, turnRef{id: turnID, ts: t.Timestamp})
			}
		}
		if len(refs) == 0 {
			continue
		}
		sort.Slice(refs, func(i, j int) bool {
			return refs[i].ts.Before(refs[j].ts)
		})
		logPath := conversationLogPath(privateDir)
		if err := writeAtomic(logPath, nil, 0644); err != nil {
			return err
		}
		for _, ref := range refs {
			if err := appendConversationRef(logPath, ref.id); err != nil {
				return err
			}
		}
	}
	return nil
}

func repairRunCatalog(basePath string) error {
	runDirs, err := sessionRunDirs(basePath)
	if err != nil {
		return err
	}
	var runs []catalogEntry
	for _, runDir := range runDirs {
		privateDir := filepath.Join(runDir, aigenticDirName)
		contextFile := filepath.Join(privateDir, "context.json")
		f, err := os.Open(contextFile)
		if err != nil {
			continue
		}
		dec := json.NewDecoder(f)
		id, name, summary, err := decodeContextJSONForSession(dec)
		_ = f.Close()
		if err != nil || id == "" {
			continue
		}
		runState := ""
		var meta map[string]interface{}
		if data, err := os.ReadFile(filepath.Join(privateDir, "run_meta.json")); err == nil {
			if json.Unmarshal(data, &meta) == nil {
				if s, ok := meta["run_state"].(string); ok {
					runState = s
				}
			} else {
				meta = nil
			}
		}
		if !runUsesCurrentStorage(runDir) {
			continue
		}
		runs = append(runs, catalogEntry{
			ID:                id,
			Name:              name,
			Summary:           summary,
			Path:              runDir,
			RunState:          runState,
			Meta:              cloneMeta(meta),
			TurnCount:         conversationTurnCount(privateDir),
			UpdatedAt:         time.Now().UTC(),
			RunStorageVersion: catalogStorageVersion,
		})
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].UpdatedAt.After(runs[j].UpdatedAt)
	})
	snap := catalogSnapshot{Version: catalogStorageVersion, Runs: runs}
	if err := os.MkdirAll(runsDir(basePath), 0755); err != nil {
		return err
	}
	if err := writeAtomicJSON(catalogSnapshotPath(basePath), snap); err != nil {
		return fmt.Errorf("write catalog snapshot: %w", err)
	}
	return writeAtomic(catalogLogPath(basePath), nil, 0644)
}
