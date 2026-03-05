package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func New(logsRoot string) (*slog.Logger, error) {
	if err := os.MkdirAll(logsRoot, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir logs root; %w", err)
	}
	path := filepath.Join(logsRoot, "symphony.log")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file; %w", err)
	}
	writer := io.MultiWriter(os.Stderr, file)
	h := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h), nil
}
