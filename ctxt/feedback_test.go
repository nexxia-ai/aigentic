package ctxt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexxia-ai/aigentic/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTurnArtifactsWithFeedback_OnlyIncludesTurnsWithFeedback(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	// Turn with feedback - included
	mkTurn(t, ledgerShard, "20250318-aaaaaaaa", map[string]string{"feedback": "thumbs_up"}, "agent-a", nil)
	// Turn with feedback_comment - included
	mkTurn(t, ledgerShard, "20250318-bbbbbbbb", map[string]string{"feedback_comment": "needs work"}, "agent-b", nil)
	// Turn without feedback - excluded
	mkTurn(t, ledgerShard, "20250318-cccccccc", map[string]string{}, "agent-c", nil)
	// Missing meta - excluded
	mkTurnNoMeta(t, ledgerShard, "20250318-dddddddd", "agent-d", nil)

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	assert.Len(t, artifacts, 2)
	ids := make(map[string]bool)
	for _, a := range artifacts {
		ids[a.TurnID] = true
	}
	assert.True(t, ids["20250318-aaaaaaaa"])
	assert.True(t, ids["20250318-bbbbbbbb"])
	assert.False(t, ids["20250318-cccccccc"])
	assert.False(t, ids["20250318-dddddddd"])
}

func TestListTurnArtifactsWithFeedback_ReturnsTurnIDAgentNameAndFilteredFiles(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	f2 := FileRef{Path: "output/b.txt", IncludeInPrompt: false}
	f2.SetMeta(map[string]string{"visible_to_user": "true"})
	files := []FileRef{
		{Path: "uploads/a.pdf", IncludeInPrompt: true},
		f2,
	}
	mkTurn(t, ledgerShard, "20250318-aaaaaaaa", map[string]string{"feedback": "thumbs_up"}, "my-agent", files)

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	assert.Equal(t, "20250318-aaaaaaaa", artifacts[0].TurnID)
	assert.Equal(t, "my-agent", artifacts[0].AgentName)
	assert.Len(t, artifacts[0].Files, 2)
}

func TestListTurnArtifactsWithFeedback_ExcludesEphemeralFiles(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	files := []FileRef{
		{Path: "uploads/keep.pdf", IncludeInPrompt: true},
		{Path: "output/ephemeral.txt", IncludeInPrompt: true, Ephemeral: true},
	}
	mkTurn(t, ledgerShard, "20250318-aaaaaaaa", map[string]string{"feedback": "thumbs_up"}, "agent", files)

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	assert.Len(t, artifacts[0].Files, 1)
	assert.Equal(t, "uploads/keep.pdf", artifacts[0].Files[0].Path)
}

func TestListTurnArtifactsWithFeedback_IncludesIncludeInPromptAndVisibleToUser(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	f1 := FileRef{Path: "uploads/prompt.pdf", IncludeInPrompt: true}
	f2 := FileRef{Path: "output/visible.txt", IncludeInPrompt: false}
	f2.SetMeta(map[string]string{"visible_to_user": "true"})
	f3 := FileRef{Path: "output/excluded.txt", IncludeInPrompt: false}
	mkTurn(t, ledgerShard, "20250318-aaaaaaaa", map[string]string{"feedback": "thumbs_up"}, "agent", []FileRef{f1, f2, f3})

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	assert.Len(t, artifacts[0].Files, 2)
	paths := make(map[string]bool)
	for _, f := range artifacts[0].Files {
		paths[f.Path] = true
	}
	assert.True(t, paths["uploads/prompt.pdf"])
	assert.True(t, paths["output/visible.txt"])
	assert.False(t, paths["output/excluded.txt"])
}

func TestListTurnArtifactsWithFeedback_ExcludesOtherShards(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	// Valid turn for this day
	mkTurn(t, ledgerShard, "20250318-aaaaaaaa", map[string]string{"feedback": "thumbs_up"}, "agent", nil)
	// Wrong shard dir - different date
	otherShard := filepath.Join(baseDir, ledgerDir, "20250319")
	require.NoError(t, os.MkdirAll(otherShard, 0755))
	mkTurn(t, otherShard, "20250319-bbbbbbbb", map[string]string{"feedback": "thumbs_up"}, "agent", nil)

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	assert.Len(t, artifacts, 1)
	assert.Equal(t, "20250318-aaaaaaaa", artifacts[0].TurnID)
}

func TestListTurnArtifactsWithFeedback_ReturnsErrFeedbackTurnLimitExceeded(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	for i := 0; i < 5; i++ {
		turnID := "20250318-" + string(rune('a'+i)) + "aaaaaaa"
		mkTurn(t, ledgerShard, turnID, map[string]string{"feedback": "thumbs_up"}, "agent", nil)
	}

	_, err := ListTurnArtifactsWithFeedback(baseDir, day, 3)
	assert.True(t, errors.Is(err, ErrFeedbackTurnLimitExceeded))
}

func TestListTurnArtifactsWithFeedback_EmptyOrMissingMetaExcludesTurn(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	// Turn with empty meta.json - no feedback
	turnDir := filepath.Join(ledgerShard, "20250318-aaaaaaaa")
	require.NoError(t, os.MkdirAll(turnDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(turnDir, "meta.json"), []byte("{}"), 0644))
	writeTurnJSON(t, turnDir, "20250318-aaaaaaaa", "agent", nil)

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	assert.Len(t, artifacts, 0)
}

func TestListTurnArtifactsWithFeedback_EmptyAgentNamePreserved(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)
	shard := day.Format("20060102")
	ledgerShard := filepath.Join(baseDir, ledgerDir, shard)
	require.NoError(t, os.MkdirAll(ledgerShard, 0755))

	mkTurn(t, ledgerShard, "20250318-aaaaaaaa", map[string]string{"feedback": "thumbs_up"}, "", nil)

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
	assert.Equal(t, "", artifacts[0].AgentName)
}

func TestListTurnArtifactsWithFeedback_EmptyShardReturnsNil(t *testing.T) {
	baseDir := t.TempDir()
	day := time.Date(2025, 3, 18, 0, 0, 0, 0, time.UTC)

	artifacts, err := ListTurnArtifactsWithFeedback(baseDir, day, 10)
	require.NoError(t, err)
	assert.Nil(t, artifacts)
}

func mkTurn(t *testing.T, ledgerShard, turnID string, meta map[string]string, agentName string, files []FileRef) {
	t.Helper()
	turnDir := filepath.Join(ledgerShard, turnID)
	require.NoError(t, os.MkdirAll(turnDir, 0755))
	metaPath := filepath.Join(turnDir, "meta.json")
	metaData, err := json.MarshalIndent(meta, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(metaPath, metaData, 0644))

	writeTurnJSON(t, turnDir, turnID, agentName, files)
}

func mkTurnNoMeta(t *testing.T, ledgerShard, turnID string, agentName string, files []FileRef) {
	t.Helper()
	turnDir := filepath.Join(ledgerShard, turnID)
	require.NoError(t, os.MkdirAll(turnDir, 0755))
	writeTurnJSON(t, turnDir, turnID, agentName, files)
}

func writeTurnJSON(t *testing.T, turnDir, turnID, agentName string, files []FileRef) {
	t.Helper()
	if files == nil {
		files = []FileRef{}
	}
	type fileEntry struct {
		Path            string            `json:"path"`
		IncludeInPrompt bool              `json:"include_in_prompt"`
		Ephemeral       bool              `json:"ephemeral"`
		Metadata        map[string]string `json:"metadata,omitempty"`
	}
	entries := make([]fileEntry, len(files))
	for i, f := range files {
		entries[i] = fileEntry{
			Path:            f.Path,
			IncludeInPrompt: f.IncludeInPrompt,
			Ephemeral:       f.Ephemeral,
			Metadata:        f.Meta(),
		}
	}
	payload := map[string]interface{}{
		"turn_id":          turnID,
		"run_id":           "20250318-runid123",
		"user_message":     "test",
		"agent_name":       agentName,
		"timestamp":        time.Date(2025, 3, 18, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"files":            entries,
		"system_tags":      []TagEntry{},
		"turn_tags":        []ai.KeyValue{},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(turnDir, "turn.json"), data, 0644))
}
