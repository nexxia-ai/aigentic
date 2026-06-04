package ctxt

import (
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
	if err := saveTurn(dirPath, turn); err != nil {
		return err
	}
	return appendShardIndex(filepath.Join(l.ledgerRoot(), shard), turn)
}

func turnIDShard(turnID string) string {
	if len(turnID) < 9 || turnID[8] != '-' {
		return ""
	}
	return turnID[:8]
}

func (l *Ledger) turnDir(turnID string) string {
	shard := turnIDShard(turnID)
	if shard == "" {
		return ""
	}
	return filepath.Join(l.ledgerRoot(), shard, turnID)
}

func (l *Ledger) Get(turnID string) (*Turn, error) {
	dir := l.turnDir(turnID)
	if dir == "" {
		return nil, fmt.Errorf("invalid turnID format: %s", turnID)
	}
	return loadTurn(dir, turnID)
}

func (l *Ledger) Head(turnID string) (*Turn, error) {
	dir := l.turnDir(turnID)
	if dir == "" {
		return nil, fmt.Errorf("invalid turnID format: %s", turnID)
	}
	return loadTurnHead(dir, turnID)
}

func (l *Ledger) Exists(turnID string) bool {
	dir := l.turnDir(turnID)
	if dir == "" {
		return false
	}
	return turnHeadExists(dir)
}

func (l *Ledger) TurnDir(turnID string) string {
	return l.turnDir(turnID)
}

func (l *Ledger) refreshShardIndex(turn *Turn) error {
	if turn == nil || turn.TurnID == "" {
		return nil
	}
	shard := turnIDShard(turn.TurnID)
	if shard == "" {
		return nil
	}
	return appendShardIndex(filepath.Join(l.ledgerRoot(), shard), turn)
}

// NewRunID returns a run ID in format {yyyymmdd}-{short_uuid} for date-sharded storage.
func NewRunID(timestamp time.Time) string {
	shard := "00000000"
	if !timestamp.IsZero() {
		shard = utcDateShard(timestamp)
	}
	shortID := uuid.New().String()[:8]
	return shard + "-" + shortID
}

func RunIDShard(runID string) string {
	if len(runID) < 9 || runID[8] != '-' {
		return ""
	}
	return runID[:8]
}

func RunDir(basePath, runID string) string {
	return filepath.Join(basePath, "runs", RunIDShard(runID), runID)
}
