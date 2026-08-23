// Package logx sets up hetud's slog logger: file (rotating, single old
// generation) and optional stderr, level via HETU_LOG_LEVEL.
package logx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

var MaxBytes int64 = 5 * 1024 * 1024

func LevelFromEnv() slog.Level {
	switch os.Getenv("HETU_LOG_LEVEL") {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type Options struct {
	Path   string
	Level  slog.Level
	Stderr bool
}

type rotatingWriter struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
}

func newRotatingWriter(path string) (*rotatingWriter, error) {
	w := &rotatingWriter{path: path}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.f, w.size = f, fi.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > MaxBytes {
		_ = w.f.Close()
		_ = os.Rename(w.path, w.path+".1")
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) Close() error { return w.f.Close() }

// Setup builds the logger. The returned *os.File is the active log file
// (close it on shutdown); nil when Path is empty.
func Setup(o Options) (*slog.Logger, *os.File, error) {
	var w io.Writer
	var f *os.File
	if o.Path != "" {
		rw, err := newRotatingWriter(o.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("log file: %w", err)
		}
		w, f = rw, rw.f
	}
	if o.Stderr {
		if w == nil {
			w = os.Stderr
		} else {
			w = io.MultiWriter(w, os.Stderr)
		}
	}
	if w == nil {
		w = io.Discard
	}
	lg := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: o.Level}))
	return lg, f, nil
}
