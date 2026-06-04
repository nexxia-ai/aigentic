package ctxt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	catalogStorageVersion = 3
	catalogLogMaxBytes    = 2 * 1024 * 1024
	catalogLogMaxEvents   = 4096
)

type catalogEntry struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	Summary           string                 `json:"summary"`
	Path              string                 `json:"path"`
	RunState          string                 `json:"run_state"`
	Meta              map[string]interface{} `json:"meta,omitempty"`
	TurnCount         int                    `json:"turn_count"`
	UpdatedAt         time.Time              `json:"updated_at"`
	RunStorageVersion int                    `json:"run_storage_version,omitempty"`
}

type catalogEvent struct {
	Version int          `json:"version"`
	Type    string       `json:"type"`
	Run     catalogEntry `json:"run"`
}

type catalogSnapshot struct {
	Version int            `json:"version"`
	Runs    []catalogEntry `json:"runs"`
}

type catalogCacheKey struct {
	basePath  string
	snapMtime int64
	snapSize  int64
	logMtime  int64
	logSize   int64
}

var catalogListCache struct {
	sync.Mutex
	key  catalogCacheKey
	runs []catalogEntry
}

func runsDir(basePath string) string {
	return filepath.Join(basePath, "runs")
}

func catalogSnapshotPath(basePath string) string {
	return filepath.Join(runsDir(basePath), CatalogSnapshotName)
}

func catalogLogPath(basePath string) string {
	return filepath.Join(runsDir(basePath), CatalogLogName)
}

func invalidateCatalogCache() {
	catalogListCache.Lock()
	catalogListCache.key = catalogCacheKey{}
	catalogListCache.runs = nil
	catalogListCache.Unlock()
}

func catalogCacheKeyFor(basePath string) catalogCacheKey {
	key := catalogCacheKey{basePath: basePath}
	if st, ok := fileStatOf(catalogSnapshotPath(basePath)); ok {
		key.snapMtime = st.mtime
		key.snapSize = st.size
	}
	if st, ok := fileStatOf(catalogLogPath(basePath)); ok {
		key.logMtime = st.mtime
		key.logSize = st.size
	}
	return key
}

func loadCatalogSnapshot(basePath string) (map[string]catalogEntry, error) {
	path := catalogSnapshotPath(basePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]catalogEntry), nil
		}
		return nil, err
	}
	var snap catalogSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	if snap.Version != catalogStorageVersion {
		return nil, fmt.Errorf("unsupported catalog snapshot version %d", snap.Version)
	}
	out := make(map[string]catalogEntry, len(snap.Runs))
	for _, r := range snap.Runs {
		out[r.ID] = r
	}
	return out, nil
}

func applyCatalogLog(basePath string, runs map[string]catalogEntry) error {
	path := catalogLogPath(basePath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	var corrupt bool
	err := readJSONLLines(path, func(line []byte, lineNum int, isLast bool) error {
		var ev catalogEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			if isLast {
				return nil
			}
			corrupt = true
			return fmt.Errorf("catalog log line %d: %w", lineNum, err)
		}
		if ev.Version != catalogStorageVersion {
			return fmt.Errorf("unsupported catalog log version %d", ev.Version)
		}
		if ev.Run.ID == "" {
			if isLast {
				return nil
			}
			corrupt = true
			return fmt.Errorf("catalog log line %d: empty run id", lineNum)
		}
		if ev.Type == "" || ev.Type == "upsert" {
			runs[ev.Run.ID] = ev.Run
		}
		return nil
	})
	if corrupt && err != nil {
		return err
	}
	return err
}

func materializeCatalog(basePath string) ([]catalogEntry, error) {
	key := catalogCacheKeyFor(basePath)
	catalogListCache.Lock()
	if catalogListCache.runs != nil && catalogListCache.key == key {
		out := make([]catalogEntry, len(catalogListCache.runs))
		copy(out, catalogListCache.runs)
		catalogListCache.Unlock()
		return out, nil
	}
	catalogListCache.Unlock()

	runs, err := loadCatalogSnapshot(basePath)
	if err != nil {
		return nil, err
	}
	if err := applyCatalogLog(basePath, runs); err != nil {
		return nil, err
	}
	list := make([]catalogEntry, 0, len(runs))
	for _, r := range runs {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})

	catalogListCache.Lock()
	catalogListCache.key = key
	catalogListCache.runs = list
	out := make([]catalogEntry, len(list))
	copy(out, list)
	catalogListCache.Unlock()
	return out, nil
}

func appendCatalogEvent(basePath string, entry catalogEntry) error {
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now().UTC()
	}
	ev := catalogEvent{
		Version: catalogStorageVersion,
		Type:    "upsert",
		Run:     entry,
	}
	logPath := catalogLogPath(basePath)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}
	if err := appendJSONL(logPath, ev); err != nil {
		return err
	}
	invalidateCatalogCache()
	return maybeCompactCatalog(basePath)
}

func maybeCompactCatalog(basePath string) error {
	logPath := catalogLogPath(basePath)
	fi, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Size() < catalogLogMaxBytes {
		lines, _ := countFileLines(logPath)
		if lines < catalogLogMaxEvents {
			return nil
		}
	}
	list, err := materializeCatalog(basePath)
	if err != nil {
		return err
	}
	snap := catalogSnapshot{Version: catalogStorageVersion, Runs: list}
	snapPath := catalogSnapshotPath(basePath)
	if err := writeAtomicJSON(snapPath, snap); err != nil {
		return err
	}
	if err := writeAtomic(logPath, nil, 0644); err != nil {
		return err
	}
	invalidateCatalogCache()
	return nil
}

func countFileLines(path string) (int, error) {
	n := 0
	err := readJSONLLines(path, func([]byte, int, bool) error {
		n++
		return nil
	})
	return n, err
}

func cloneMeta(meta map[string]interface{}) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func catalogEntryFromContext(ac *AgentContext) catalogEntry {
	if ac == nil {
		return catalogEntry{}
	}
	runState := ""
	if v, ok := ac.GetMeta("run_state"); ok {
		if s, ok := v.(string); ok {
			runState = s
		}
	}
	turnCount := 0
	if ac.conversationHistory != nil {
		turnCount = ac.conversationHistory.Len()
	}
	path := ""
	if ac.workspace != nil {
		path = ac.workspace.RootDir
	}
	return catalogEntry{
		ID:                ac.id,
		Name:              ac.name,
		Summary:           ac.summary,
		Path:              path,
		RunState:          runState,
		Meta:              cloneMeta(ac.runMeta),
		TurnCount:         turnCount,
		UpdatedAt:         time.Now().UTC(),
		RunStorageVersion: catalogStorageVersion,
	}
}

// catalogEntryListable decides whether a catalog row may appear in ListSessions.
// Only current-format rows are listed; older catalog versions trigger repair before
// reaching this point, so this stays a pure in-memory check.
func catalogEntryListable(e catalogEntry) bool {
	if e.Path == "" {
		return false
	}
	return e.RunStorageVersion == catalogStorageVersion
}

func upsertCatalogForContext(ac *AgentContext) error {
	if ac == nil || ac.basePath == "" {
		return nil
	}
	return appendCatalogEvent(ac.basePath, catalogEntryFromContext(ac))
}

func catalogEntryToSession(e catalogEntry) Session {
	session := Session{
		ID:      e.ID,
		Name:    e.Name,
		Summary: e.Summary,
		Path:    e.Path,
		Turns:   e.TurnCount,
	}
	session.Meta = cloneMeta(e.Meta)
	if e.RunState != "" {
		if session.Meta == nil {
			session.Meta = map[string]interface{}{}
		}
		if _, ok := session.Meta["run_state"]; !ok {
			session.Meta["run_state"] = e.RunState
		}
	}
	return session
}

func catalogMissing(basePath string) bool {
	_, errSnap := os.Stat(catalogSnapshotPath(basePath))
	_, errLog := os.Stat(catalogLogPath(basePath))
	return os.IsNotExist(errSnap) && os.IsNotExist(errLog)
}
