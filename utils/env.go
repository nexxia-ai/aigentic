package utils

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func LoadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		slog.Error("error opening env file", "error", err)
		return fmt.Errorf("error opening env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid env line format: %s", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("error setting env variable %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading env file: %w", err)
	}

	return nil
}
