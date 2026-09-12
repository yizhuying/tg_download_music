// Package logging sets up the application logger writing JSON lines to
// daily-rotated files, with a plain-text mirror on stderr.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// dailyWriter appends to <dir>/tunegram-YYYY-MM-DD.log and switches to a new
// file at midnight so log retrieval lines up with calendar days.
type dailyWriter struct {
	dir string

	mu  sync.Mutex
	day string
	f   *os.File
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	day := time.Now().Format("2006-01-02")
	if w.f == nil || day != w.day {
		if w.f != nil {
			_ = w.f.Close()
		}
		f, err := os.OpenFile(filepath.Join(w.dir, "tunegram-"+day+".log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, fmt.Errorf("open log file: %w", err)
		}
		w.f, w.day = f, day
	}
	return w.f.Write(p)
}

func (w *dailyWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	return w.f.Sync()
}

func newEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	return cfg
}

// New creates the logs directory if needed and returns a logger that writes
// JSON to daily files in dir plus a console copy to stderr.
func New(dir string) (*zap.Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir %s: %w", dir, err)
	}

	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(newEncoderConfig()),
			zapcore.AddSync(&dailyWriter{dir: dir}), zapcore.InfoLevel),
		zapcore.NewCore(zapcore.NewConsoleEncoder(newEncoderConfig()),
			zapcore.AddSync(os.Stderr), zapcore.InfoLevel),
	)
	return zap.New(core), nil
}

// NewStderr returns a stderr-only logger for when no log directory is usable.
func NewStderr() *zap.Logger {
	return zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(newEncoderConfig()),
		zapcore.AddSync(os.Stderr), zapcore.InfoLevel))
}
