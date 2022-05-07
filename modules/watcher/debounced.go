package watcher

import (
	"path/filepath"
	"sync"
	"time"
)

type DebouncedWatcher struct {
	Events chan Event
	w      *Watcher
	events map[string]Event
	mu     *sync.Mutex
	done   chan struct{}
}

func NewDebounced() *DebouncedWatcher {
	d := DebouncedWatcher{
		Events: make(chan Event),
		w:      New(),
		events: make(map[string]Event),
		mu:     &sync.Mutex{},
		done:   make(chan struct{}),
	}
	go d.receiveEvents()
	go d.sendEvents()

	return &d
}

func (d *DebouncedWatcher) AddRecursiveWatch(p string) error {
	return d.w.AddRecursiveWatch(p)
}

func (d *DebouncedWatcher) Close() error {
	d.done <- struct{}{}

	return d.w.Close()
}

func (d *DebouncedWatcher) receiveEvents() {
	for {
		select {
		case event := <-d.w.Events:
			if event.Kind() == KindDelete {
				d.removeSuperseded(event)
			}

			d.mu.Lock()

			if e, ok := d.events[event.Path]; ok {
				// An event for this path already existed. We have to debounce it
				d.events[event.Path] = debounceEvent(e, event)
			} else {
				d.events[event.Path] = event
			}

			d.mu.Unlock()

		case <-d.done:
			return
		}
	}
}

func (d *DebouncedWatcher) sendEvents() {
	t := time.NewTicker(4 * time.Second)

	for {
		select {
		case <-t.C:
			d.mu.Lock()

			for _, e := range d.events {
				// Forward event to user
				due := e.LastModified.Add(10 * time.Second)
				if time.Now().After(due) {
					d.Events <- e
					delete(d.events, e.Path)
				}
			}

			d.mu.Unlock()
		case <-d.done:
			return
		}
	}
}

func (d *DebouncedWatcher) removeSuperseded(event Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, e := range d.events {
		path := e.Path

		for path != "/" {
			// Discard other events if they occurred in this event folder
			if event.Path == path {
				delete(d.events, e.Path)
			}

			path = filepath.Dir(path)
		}
	}
}
