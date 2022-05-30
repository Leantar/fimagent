//go:build darwin

package watcher

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsevents"
	"io/fs"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"
)

type Watcher struct {
	Events  chan Event
	watcher *fsevents.EventStream
}

func New() *Watcher {

	path, err := ioutil.TempDir("", "fimagent")
	if err != nil {
		log.Fatalf("Failed to create TempDir: %v", err)
	}

	dev, err := fsevents.DeviceForPath(path)
	if err != nil {
		log.Fatalf("failed to retrieve device for path: %v", err)
	}

	w := &fsevents.EventStream{
		Paths:   []string{""},
		Latency: 1 * time.Millisecond,
		Device:  dev,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot}
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
					Path:         event[0].Path,
					Mask:         uint64(event[0].Flags),
					Created:      t,
					LastModified: t,
				}
			}
		}
	}()

	return &Watcher{
		Events:  eventsChan,
		watcher: w,
	}
}

func (w *Watcher) AddRecursiveWatch(p string) error {
	ap, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	streamPaths := w.watcher.Paths

	return filepath.WalkDir(ap, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		for _, p := range streamPaths {
			if p == path {
				return errors.New("path is already watched")
			}
		}

		w.watcher.Stop()
		w.watcher.Paths = append(streamPaths, path)
		w.watcher.Start()

		return nil
	})
}

func (w *Watcher) Close() error {
	w.watcher.Stop()
	return nil
}
