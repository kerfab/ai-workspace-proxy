package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DeniedLogEntry struct {
	Timestamp  string            `json:"timestamp"`
	UserID     string            `json:"user_id,omitempty"`
	Mailbox    string            `json:"mailbox,omitempty"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      string            `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Reason     string            `json:"reason"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
}

type DeniedLogger struct {
	mu         sync.Mutex
	path       string
	maxSize    int64
	totalFiles int
}

func NewDeniedLogger(path string, maxSize int64, totalFiles int) *DeniedLogger {
	return &DeniedLogger{path: path, maxSize: maxSize, totalFiles: totalFiles}
}

func (l *DeniedLogger) Log(entry DeniedLogEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	if err := l.rotateIfNeeded(); err != nil {
		return err
	}
	entry.Timestamp = nowUTC().Format(time.RFC3339Nano)
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func (l *DeniedLogger) rotateIfNeeded() error {
	info, err := os.Stat(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < l.maxSize {
		return nil
	}
	maxRotation := l.totalFiles - 1
	if maxRotation < 1 {
		_ = os.Remove(l.path)
		return nil
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", l.path, maxRotation))
	for i := maxRotation - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", l.path, i)
		newPath := fmt.Sprintf("%s.%d", l.path, i+1)
		if _, err := os.Stat(oldPath); err == nil {
			_ = os.Rename(oldPath, newPath)
		}
	}
	_ = os.Rename(l.path, l.path+".1")
	return nil
}
