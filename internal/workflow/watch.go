package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	watcher  *fsnotify.Watcher
	path     string
	onChange func()
	mu       sync.Mutex
	lastMod  time.Time
}

func NewWatcher(path string, onChange func()) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create workflow watcher; %w", err)
	}
	if err := w.Add(filepath.Dir(path)); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("watch workflow directory; %w", err)
	}
	return &Watcher{watcher: w, path: filepath.Clean(path), onChange: onChange}, nil
}

func (w *Watcher) Run(ctx context.Context) error {
	defer w.watcher.Close()
	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Clean(evt.Name) != w.path {
				continue
			}
			if evt.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			w.fireDebounced()
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}

func (w *Watcher) fireDebounced() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if now.Sub(w.lastMod) < 250*time.Millisecond {
		return
	}
	w.lastMod = now
	if w.onChange != nil {
		w.onChange()
	}
}
