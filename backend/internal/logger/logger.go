package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"iotsmart/backend/internal/config"
)

type Logger struct {
	*log.Logger
	path string
}

func New(cfg config.LoggingConfig) (*Logger, io.Closer, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, nil, err
	}

	file, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	output := io.MultiWriter(os.Stdout, file)
	base := log.New(output, "", log.LstdFlags|log.Lmicroseconds)
	return &Logger{Logger: base, path: cfg.Path}, file, nil
}

func (l *Logger) Path() string {
	return l.path
}
