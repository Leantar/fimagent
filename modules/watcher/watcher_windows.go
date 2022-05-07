//go:build windows

package watcher

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"io/fs"
	"path/filepath"
	"time"
)

type Watcher struct {
	Events  chan Event
	watcher *fsnotify.Watcher
}

func New() *Watcher {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		panic(fmt.Errorf("failed to create watcher: %w", err))
	}

	eventsChan := make(chan Event)
	go func() {
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}

				t := time.Now()
				eventsChan <- Event{
					Path:         event.Name,
					Mask:         uint64(event.Op),
					Created:      t,
					LastModified: t,
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				panic(fmt.Errorf("watcher returned error: %w", err))
			}
		}
	}()

	return &Watcher{
		watcher: w,
		Events:  eventsChan,
	}
}

func (w *Watcher) AddRecursiveWatch(p string) error {
	return filepath.WalkDir(p, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			err := w.watcher.Add(path)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (w *Watcher) Close() error {
	return w.watcher.Close()
}
