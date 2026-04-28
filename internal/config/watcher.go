package config

import (
	"crypto/sha256"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors WORKFLOW.md for changes and reloads config dynamically.
// SPEC Section 6.2.
type Watcher struct {
	path     string
	current  atomic.Pointer[WorkflowDefinition]
	mu       sync.Mutex // protects reload to prevent concurrent reloads
	lastHash [32]byte
	stopCh   chan struct{}
	onChange func(*WorkflowDefinition) // optional callback on successful reload
}

// NewWatcher creates a config watcher for the given workflow file path.
// It performs the initial load synchronously.
func NewWatcher(path string) (*Watcher, error) {
	w := &Watcher{
		path:   path,
		stopCh: make(chan struct{}),
	}

	// Initial load.
	if err := w.reload(); err != nil {
		return nil, err
	}

	return w, nil
}

// Current returns the latest valid workflow definition.
func (w *Watcher) Current() *WorkflowDefinition {
	return w.current.Load()
}

// OnChange registers a callback invoked on successful reload.
func (w *Watcher) OnChange(fn func(*WorkflowDefinition)) {
	w.onChange = fn
}

// Start begins watching for file changes. Call Stop to clean up.
func (w *Watcher) Start() {
	// Start fsnotify watcher.
	go w.watchFsnotify()
	// Also poll as a fallback (SPEC: "re-validate/reload defensively").
	go w.pollFallback()
}

// Stop halts the watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

func (w *Watcher) reload() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := os.ReadFile(w.path)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(data)
	if hash == w.lastHash && w.current.Load() != nil {
		return nil // no change
	}

	def, err := LoadWorkflow(w.path)
	if err != nil {
		slog.Error("workflow reload failed, keeping last good config",
			"path", w.path,
			"error", err,
		)
		return err
	}

	w.lastHash = hash
	w.current.Store(def)

	slog.Info("workflow reloaded", "path", w.path)
	if w.onChange != nil {
		w.onChange(def)
	}

	return nil
}

func (w *Watcher) watchFsnotify() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("fsnotify unavailable, relying on polling", "error", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(w.path); err != nil {
		slog.Warn("cannot watch workflow file, relying on polling", "error", err)
		return
	}

	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				// Small delay to let the write complete.
				time.Sleep(100 * time.Millisecond)
				if err := w.reload(); err != nil {
					slog.Error("fsnotify reload failed", "error", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Error("fsnotify error", "error", err)
		}
	}
}

func (w *Watcher) pollFallback() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.reload(); err != nil {
				// Already logged inside reload.
			}
		}
	}
}
