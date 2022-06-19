//go:build darwin

package watcher

import (
	"fmt"
	"github.com/fsnotify/fsevents"
	"log"
	"path/filepath"
	"time"
)

type Watcher struct {
	Events   chan Event
	watchers []*fsevents.EventStream
}

func New() *Watcher {
	return &Watcher{
		watchers: []*fsevents.EventStream{},
		Events:   make(chan Event),
	}
}

func (w *Watcher) AddRecursiveWatch(p string) error {

	ap, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	dev, err := fsevents.DeviceForPath(ap)
	if err != nil {
		log.Fatalf("failed to retrieve device for path: %v", err)
	}

	wa := &fsevents.EventStream{
		Paths:   []string{ap},
		Latency: 100 * time.Millisecond,
		Device:  dev,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot}
	wa.Start()

	w.watchers = append(w.watchers, wa)

	go func() {
		for msg := range wa.Events {
			for _, event := range msg {
				print(event.Path)
				t := time.Now()
				w.Events <- Event{
					Path:         event.Path,
					Mask:         event.ID,
					Created:      t,
					LastModified: t,
				}
			}
		}
	}()

	return nil
}

func (w *Watcher) Close() error {
	for _, watcher := range w.watchers {
		watcher.Stop()
	}
	return nil
}
