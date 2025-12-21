package trace

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nexxia-ai/aigentic/run"
)

var (
	traceSync = sync.Mutex{}
)

type TraceConfig struct {
	Directory         string
	RetentionDuration time.Duration
	MaxTraceFiles     int
}

type Tracer struct {
	config  TraceConfig
	counter int64
}

const (
	defaultRetentionDuration = 7 * 24 * time.Hour
	defaultMaxTraceFiles     = 10
)

func NewTracer(config ...TraceConfig) run.Trace {
	defaultDir := filepath.Join(os.TempDir(), "aigentic-traces")

	cfg := TraceConfig{
		Directory:         defaultDir,
		RetentionDuration: defaultRetentionDuration,
		MaxTraceFiles:     defaultMaxTraceFiles,
	}

	if len(config) > 0 {
		if config[0].Directory != "" {
			cfg.Directory = config[0].Directory
		}
		if config[0].RetentionDuration > 0 {
			cfg.RetentionDuration = config[0].RetentionDuration
		}
		if config[0].MaxTraceFiles > 0 {
			cfg.MaxTraceFiles = config[0].MaxTraceFiles
		}
	}

	t := &Tracer{
		config:  cfg,
		counter: 0,
	}

	os.MkdirAll(cfg.Directory, 0755)

	return t.NewTraceRun()
}

func (tr *Tracer) NewTraceRun() *TraceRun {
	timestamp := time.Now().Format("20060102150405")
	counter := atomic.AddInt64(&tr.counter, 1)
	filepath := filepath.Join(tr.config.Directory, fmt.Sprintf("trace-%s.%03d.txt", timestamp, counter))

	tr.cleanup()

	traceRun := &TraceRun{
		tracer:    tr,
		startTime: time.Now(),
		filepath:  filepath,
	}

	var file traceWriter
	osFile, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Failed to open trace file, using io.Discard", "file", filepath, "error", err)
		file = &discardWriter{}
	} else {
		file = osFile
	}

	traceRun.file = file
	return traceRun
}

func (tr *Tracer) cleanup() {
	entries, err := os.ReadDir(tr.config.Directory)
	if err != nil {
		slog.Error("Failed to read trace directory", "error", err)
		return
	}

	var traceFiles []struct {
		path    string
		modTime time.Time
	}

	cutoffTime := time.Now().Add(-tr.config.RetentionDuration)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "trace-") || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		filePath := filepath.Join(tr.config.Directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		traceFiles = append(traceFiles, struct {
			path    string
			modTime time.Time
		}{
			path:    filePath,
			modTime: info.ModTime(),
		})
	}

	sort.Slice(traceFiles, func(i, j int) bool {
		return traceFiles[i].modTime.Before(traceFiles[j].modTime)
	})

	if tr.config.RetentionDuration > 0 {
		for _, file := range traceFiles {
			if file.modTime.Before(cutoffTime) {
				if err := os.Remove(file.path); err != nil {
					slog.Error("Failed to remove old trace file", "file", file.path, "error", err)
				} else {
					slog.Debug("Removed old trace file", "file", filepath.Base(file.path))
				}
			}
		}
	}

	if tr.config.MaxTraceFiles > 0 && len(traceFiles) > tr.config.MaxTraceFiles {
		filesToRemove := len(traceFiles) - tr.config.MaxTraceFiles
		for i := 0; i < filesToRemove && i < len(traceFiles); i++ {
			if err := os.Remove(traceFiles[i].path); err != nil {
				slog.Error("Failed to remove excess trace file", "file", traceFiles[i].path, "error", err)
			} else {
				slog.Debug("Removed excess trace file", "file", filepath.Base(traceFiles[i].path))
			}
		}
	}
}
