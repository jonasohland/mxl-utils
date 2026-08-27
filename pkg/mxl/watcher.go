package mxl

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/samber/lo"
)

type FlowReceiver interface {
	AddFlow(domain, id string)
	RemoveFlow(domain, id string)
}

type watcherEntry struct {
	ino   uint64
	flows []string
}

type Watcher struct {
	mu     sync.Mutex
	notify *fsnotify.Watcher
	cache  map[string]*watcherEntry
	recv   []FlowReceiver
}

func NewWatcher(ctx context.Context, wg *sync.WaitGroup, recv []FlowReceiver) (*Watcher, error) {
	notify, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		mu:     sync.Mutex{},
		notify: notify,
		cache:  map[string]*watcherEntry{},
		recv:   recv,
	}

	wg.Go(func() { w.run(ctx) })

	return w, nil
}

func (w *Watcher) AddDomain(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.add(path)
}

func (w *Watcher) add(path string) {
	ino, err := getIno(path)
	if err != nil {
		slog.Error("failed to get directory ino for domain", "path", path, "error", err)
	}

	if err := w.notify.Add(path); err != nil {
		slog.Error("failed to add inotify watch", "path", path, "error", err)
	}

	_, ok := w.cache[path]
	if ok {
		slog.Warn("received domain discovery notification for a known domain")
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		slog.Error("failed to read domain directory entries", "error", err)
	}

	flows := make([]string, 0)
	for _, entry := range entries {
		id, ok := GetFlowIDFromPath(entry.Name())
		if ok {
			flows = append(flows, id)
		}
	}

	w.cache[path] = &watcherEntry{ino: ino, flows: flows}

	for _, id := range flows {
		w.onFlowAdded(path, id)
	}
}

func (w *Watcher) RemoveDomain(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.remove(path)
}

func (w *Watcher) remove(path string) {
	if err := w.notify.Remove(path); err != nil {
		slog.Error("failed to remove inotify watch", "path", path, "error", err)
	}

	entry, ok := w.cache[path]
	if !ok {
		return
	}

	for _, id := range entry.flows {
		w.onFlowRemoved(path, id)
	}

	delete(w.cache, path)
}

func (w *Watcher) run(ctx context.Context) {
	for {
		select {
		case <-time.After(4 * time.Second):
			w.refreshDomains()
		case ev, ok := <-w.notify.Events:
			if !ok {
				return
			}
			if !ev.Op.Has(fsnotify.Create) && !ev.Op.Has(fsnotify.Remove) {
				continue
			}

			if IsFlowDir(ev.Name) {
				if ev.Op.Has(fsnotify.Create) {
					w.flowAddedEvent(ev.Name)
				} else if ev.Op.Has(fsnotify.Remove) {
					w.flowRemovedEvent(ev.Name)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// Handles cases where the domain directory is removed and re-created
func (w *Watcher) refreshDomains() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for path, entry := range w.cache {
		ino, err := getIno(path)
		if err != nil {
			continue
		}

		if entry.ino != ino {
			w.remove(path)
			w.add(path)
		}
	}
}

func (w *Watcher) flowAddedEvent(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	domain := filepath.Dir(path)
	entry, ok := w.cache[domain]
	if !ok {
		return
	}

	id, ok := GetFlowIDFromPath(path)
	if !ok {
		return
	}

	if !lo.Contains(entry.flows, id) {
		entry.flows = append(entry.flows, id)
		w.onFlowAdded(domain, id)
	}
}

func (w *Watcher) flowRemovedEvent(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	domain := filepath.Dir(path)
	entry, ok := w.cache[domain]
	if !ok {
		return
	}

	id, ok := GetFlowIDFromPath(path)
	if !ok {
		return
	}

	if lo.Contains(entry.flows, id) {
		entry.flows = lo.Without(entry.flows, id)
		w.onFlowRemoved(domain, id)
	}
}

func (w *Watcher) onFlowAdded(domain, id string) {
	for _, recv := range w.recv {
		recv.AddFlow(domain, id)
	}
}

func (w *Watcher) onFlowRemoved(domain, id string) {
	for _, recv := range w.recv {
		recv.RemoveFlow(domain, id)
	}
}
