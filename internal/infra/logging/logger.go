package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu   sync.Mutex
	file *os.File
	path string
}

func New(configDir string) (*Logger, error) {
	logDir := filepath.Join(configDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(logDir, "application.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return &Logger{
		file: file,
		path: path,
	}, nil
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}

	return l.file.Close()
}

func (l *Logger) Path() string {
	return l.path
}

func (l *Logger) Infof(format string, args ...any) {
	l.write("INFO", format, args...)
}

func (l *Logger) Warnf(format string, args ...any) {
	l.write("WARN", format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.write("ERROR", format, args...)
}

func (l *Logger) write(level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return
	}

	line := fmt.Sprintf(
		"%s [%s] %s\n",
		time.Now().Format(time.RFC3339),
		level,
		fmt.Sprintf(format, args...),
	)
	_, _ = l.file.WriteString(line)
}
