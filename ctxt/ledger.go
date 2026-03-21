package ctxt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const ledgerDir = "ledger"

type Ledger struct {
	basePath string
}

func NewLedger(basePath string) *Ledger {
	return &Ledger{basePath: basePath}
}

func (l *Ledger) ledgerRoot() string {
	return filepath.Join(l.basePath, ledgerDir)
}

// utcDateShard returns the UTC calendar date as yyyymmdd for ledger path segments.
func utcDateShard(t time.Time) string {
	return t.In(time.UTC).Format("20060102")
}

func (l *Ledger) PrepareTurn(timestamp time.Time) (turnID, dirPath string, err error) {
	shard := utcDateShard(timestamp)
	shortID := uuid.New().String()[:8]
	turnID = shard + "-" + shortID

	dirPath = filepath.Join(l.ledgerRoot(), shard, turnID)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", "", fmt.Errorf("create turn dir: %w", err)
	}
	return turnID, dirPath, nil
}

func (l *Ledger) Append(turn *Turn) error {
	if turn.TurnID == "" {
		return fmt.Errorf("turn has no turnID (call PrepareTurn first)")
	}
	shard := turnIDShard(turn.TurnID)
	if shard == "" {
		return fmt.Errorf("invalid turnID format: %s", turn.TurnID)
	}
	dirPath := filepath.Join(l.ledgerRoot(), shard, turn.TurnID)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("create turn dir: %w", err)
	}
	path := filepath.Join(dirPath, "turn.json")
	data, err := json.MarshalIndent(turn, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal turn: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write turn file: %w", err)
	}
	turn.SetLedgerDir(dirPath)
	if err := turn.saveMeta(); err != nil {
		return fmt.Errorf("write turn meta file: %w", err)
	}
	return nil
}

func turnIDShard(turnID string) string {
	if len(turnID) < 9 || turnID[8] != '-' {
		return ""
	}
	return turnID[:8]
}

func (l *Ledger) Get(turnID string) (*Turn, error) {
	shard := turnIDShard(turnID)
	if shard == "" {
		return nil, fmt.Errorf("invalid turnID format: %s", turnID)
	}
	path := filepath.Join(l.ledgerRoot(), shard, turnID, "turn.json")
	var t Turn
	if err := t.loadFromFile(path); err != nil {
		return nil, err
	}
	t.TurnID = turnID
	turnDir := filepath.Dir(path)
	t.TraceFile = filepath.Join(turnDir, "trace.txt")
	return &t, nil
}

func (l *Ledger) Exists(turnID string) bool {
	shard := turnIDShard(turnID)
	if shard == "" {
		return false
	}
	path := filepath.Join(l.ledgerRoot(), shard, turnID, "turn.json")
	_, err := os.Stat(path)
	return err == nil
}

func (l *Ledger) TurnDir(turnID string) string {
	shard := turnIDShard(turnID)
	if shard == "" {
		return ""
	}
	return filepath.Join(l.ledgerRoot(), shard, turnID)
}

// NewRunID returns a run ID in format {yyyymmdd}-{short_uuid} for date-sharded storage.
// The yyyymmdd prefix is the UTC calendar date of timestamp, except the zero time uses 00000000
// (e.g. single-instance runs). On-disk layout: runs/{yyyymmdd}/{runID}/.
func NewRunID(timestamp time.Time) string {
	shard := "00000000"
	if !timestamp.IsZero() {
		shard = utcDateShard(timestamp)
	}
	shortID := uuid.New().String()[:8]
	return shard + "-" + shortID
}

// RunIDShard returns the UTC date shard (yyyymmdd) from a runID in format {yyyymmdd}-{short_uuid}, or "" if invalid.
// The shard is a path segment under runs/ and ledger/; it encodes the UTC calendar day used when the ID was created.
func RunIDShard(runID string) string {
	if len(runID) < 9 || runID[8] != '-' {
		return ""
	}
	return runID[:8]
}

// RunDir returns the run directory path for a given runID.
// RunID is expected in format {yyyymmdd}-{short_uuid} where yyyymmdd is the UTC date shard.
func RunDir(basePath, runID string) string {
	return filepath.Join(basePath, "runs", RunIDShard(runID), runID)
}
