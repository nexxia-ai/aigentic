package ctxt

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const storageVersion = 1

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

func writeAtomicJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, 0644)
}

func readJSONWithVersion(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var wrapper struct {
		Version int             `json:"version"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	if wrapper.Version != storageVersion {
		return fmt.Errorf("ctxt: unsupported version %d in %s", wrapper.Version, path)
	}
	if len(wrapper.Payload) == 0 {
		return fmt.Errorf("ctxt: empty payload in %s", path)
	}
	return json.Unmarshal(wrapper.Payload, v)
}

func appendJSONL(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	if syncErr := f.Sync(); syncErr != nil && werr == nil {
		werr = syncErr
	}
	if closeErr := f.Close(); closeErr != nil && werr == nil {
		werr = closeErr
	}
	return werr
}

type fileStat struct {
	mtime int64
	size  int64
}

func fileStatOf(path string) (fileStat, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStat{}, false
	}
	return fileStat{mtime: fi.ModTime().UnixNano(), size: fi.Size()}, true
}

func readJSONLLines(path string, parse func(line []byte, lineNum int, isLast bool) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var lines [][]byte
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		cp := make([]byte, len(line))
		copy(cp, line)
		lines = append(lines, cp)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	for i, line := range lines {
		if err := parse(line, i+1, i == len(lines)-1); err != nil {
			return err
		}
	}
	return nil
}
