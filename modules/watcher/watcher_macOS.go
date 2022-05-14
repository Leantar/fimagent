//go:build macOS

package watcher

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsevents"
	"path/filepath"
	"time"
)

type Watcher struct {
	Events  chan Event
	watcher *fsevents.EventStream
}

func New() *Watcher {

	dev, err := fsevents.DeviceForPath(path)
	if err != nil {
		panic(fmt.Errorf("failed to retrieve device for path: %v", err))
	}

	w, err := &fsevents.EventStream{
		Paths:   []string{},
		Latency: 1 * time.Millisecond,
		Device:  dev,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot}
	if err != nil {
		panic(fmt.Errorf("failed to initialize watcher: %w", err))
	}
	w.Start()

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
					Path:         event.Path,
					Mask:         uint32(event.Flags),
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

	return &Watcher{{
		Events:  eventsChan,
		watcher: w,
	}}
}

func (w *Watcher) AddRecursiveWatch(p string) error {
	ap, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	streamPaths = w.GetStreamRefPaths()

	return filepath.WalkDir(ap, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		for _, p := range streamPaths {
			if p == path {
				return errors.New("path is already watched")
			}
		}

		w.Stop()
		w.Paths = append(streamPaths, path)
		w.Start()

		return nil
	})
}

func (w *Watcher) Close() error {
	return w.Stop()
}
