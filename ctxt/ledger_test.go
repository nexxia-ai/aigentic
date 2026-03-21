package ctxt

import (
	"strings"
	"testing"
	"time"
)

func TestNewRunIDUsesUTCCalendarDate(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip(err)
	}
	// 05:00 UTC on 15 June is still 14 June evening in Los Angeles.
	utc := time.Date(2025, 6, 15, 5, 0, 0, 0, time.UTC)
	local := utc.In(loc)
	if got := local.Format("20060102"); got != "20250614" {
		t.Fatalf("local calendar date sanity: got %s want 20250614", got)
	}
	runID := NewRunID(local)
	wantPrefix := "20250615-"
	if len(runID) < len(wantPrefix) || runID[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("NewRunID(%v) = %q, want prefix %q", local, runID, wantPrefix)
	}
}

func TestPrepareTurnUsesUTCCalendarDateShard(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip(err)
	}
	utc := time.Date(2025, 6, 15, 5, 0, 0, 0, time.UTC)
	local := utc.In(loc)
	ledger := NewLedger(t.TempDir())
	turnID, _, err := ledger.PrepareTurn(local)
	if err != nil {
		t.Fatalf("PrepareTurn: %v", err)
	}
	if !strings.HasPrefix(turnID, "20250615-") {
		t.Fatalf("turnID = %q, want prefix 20250615-", turnID)
	}
}
