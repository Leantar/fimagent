//go:build linux

package watcher

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	InitFlags = unix.FAN_CLOEXEC |
		unix.FAN_REPORT_DFID_NAME |
		unix.FAN_UNLIMITED_QUEUE
	InitEventFlags = unix.O_CLOEXEC |
		unix.O_RDONLY |
		unix.O_LARGEFILE
	MarkOpenFlags = unix.FAN_MARK_ADD |
		unix.FAN_MARK_FILESYSTEM
	MarkCloseFlags = unix.FAN_MARK_FLUSH |
		unix.FAN_MARK_FILESYSTEM
	MarkEventFlags = unix.FAN_MODIFY |
		unix.FAN_CREATE |
		unix.FAN_DELETE |
		unix.FAN_MOVE |
		unix.FAN_ATTRIB |
		unix.FAN_ONDIR
	MountFDMode = unix.O_DIRECTORY |
		unix.O_RDONLY
)

type fanotifyEventInfoHeader struct {
	InfoType uint8
	Pad      uint8
	Len      uint16
}

type fanotifyEventInfoFid struct {
	fanotifyEventInfoHeader
	FSID uint64
}

type Watcher struct {
	Events  chan Event
	fd      int
	mountFd int
	watches map[string]struct{}
	mu      *sync.Mutex
}

func New() *Watcher {
	fd, err := unix.FanotifyInit(InitFlags, InitEventFlags)
	if err != nil {
		panic(fmt.Errorf("failed to initialize watcher: %w", err))
	}

	mountFd, err := unix.Open("/", MountFDMode, 0)
	if err != nil {
		_ = unix.Close(fd)
		panic(fmt.Errorf("failed to open '/' : %w", err))
	}

	w := Watcher{
		Events:  make(chan Event),
		fd:      fd,
		mountFd: mountFd,
		watches: make(map[string]struct{}),
		mu:      &sync.Mutex{},
	}

	go w.readEvents()

	return &w
}

func (w *Watcher) AddRecursiveWatch(p string) error {
	path, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Trim trailing slash to be able to detect events inside folders
	// Otherwise isWatched does not return true if an event happens inside a folder
	path = strings.TrimSuffix(path, "/")

	if w.isWatched(path) {
		return errors.New("path is already watched")
	}

	w.mu.Lock()
	w.watches[path] = struct{}{}
	w.mu.Unlock()

	err = unix.FanotifyMark(w.fd,
		MarkOpenFlags,
		MarkEventFlags,
		unix.AT_FDCWD,
		p)
	if err != nil {
		return fmt.Errorf("failed to create fanotify mark for path %s: %w", p, err)
	}

	return nil
}

func (w *Watcher) Close() error {
	err := unix.FanotifyMark(w.fd, MarkCloseFlags, 0, unix.AT_FDCWD, "/")
	if err != nil {
		return fmt.Errorf("failed to remove marks: %w", err)
	}

	_ = unix.Close(w.fd)
	_ = unix.Close(w.mountFd)

	return nil
}

func (w *Watcher) isWatched(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	for path != "/" {
		if _, ok := w.watches[path]; ok {
			return true
		}

		path = filepath.Dir(path)
	}

	return false
}

// Partly copied from LXD (https://github.com/lxc/lxd), but was mostly rewritten to fix bugs and adapt the use case
func (w *Watcher) readEvents() {
	buf := make([]byte, 4096)

	for {
		n, err := unix.Read(w.fd, buf)
		if err != nil {
			log.Error().Caller().Err(err).Msgf("failed to read event")
			return
		}

		rd := bytes.NewReader(buf)
		var offset int64

		for offset < int64(n) {
			var event unix.FanotifyEventMetadata

			err = binary.Read(rd, binary.LittleEndian, &event)
			if err != nil {
				log.Error().Caller().Err(err).Msgf("failed to read event metadata")
				break
			}

			// Read event info fid
			var fid fanotifyEventInfoFid

			err = binary.Read(rd, binary.LittleEndian, &fid)
			if err != nil {
				log.Error().Caller().Err(err).Msg("failed to read event fid")
				offset, _ = rd.Seek(offset+int64(event.Event_len), io.SeekStart)
				continue
			}

			// Although unix.FileHandle exists, it cannot be used with binary.Read() as the
			// variables inside are not exported.
			type fileHandleInfo struct {
				Bytes uint32
				Type  int32
			}

			// Read file handle information
			var fhInfo fileHandleInfo

			err = binary.Read(rd, binary.LittleEndian, &fhInfo)
			if err != nil {
				log.Error().Caller().Err(err).Msg("failed to read file handle info")
				offset, _ = rd.Seek(offset+int64(event.Event_len), io.SeekStart)
				continue
			}

			// Read file handle
			fileHandle := make([]byte, fhInfo.Bytes)

			err = binary.Read(rd, binary.LittleEndian, &fileHandle)
			if err != nil {
				log.Error().Caller().Err(err).Msg("failed to read file handle")
				offset, _ = rd.Seek(offset+int64(event.Event_len), io.SeekStart)
				continue
			}

			fh := unix.NewFileHandle(fhInfo.Type, fileHandle)

			fd, err := unix.OpenByHandleAt(w.mountFd, fh, os.O_RDONLY)
			if err != nil {
				if !errors.Is(err, unix.ESTALE) {
					// This is a common error when removing a folder containing multiple files at once.
					// It can be safely ignored, because the more important underlying folder event does not produce such an error
					log.Error().Caller().Err(err).Msg("failed to open file handle")
				}
				offset, _ = rd.Seek(offset+int64(event.Event_len), io.SeekStart)
				continue
			}

			// Determine the directory of the created or deleted file.
			dir, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
			if err != nil {
				log.Error().Caller().Err(err).Msg("failed to read symlink")
				offset, _ = rd.Seek(offset+int64(event.Event_len), io.SeekStart)
				continue
			}

			// Close fd to not run into "too many open files" error
			_ = unix.Close(fd)

			// If the target file has been deleted, the returned value might contain a " (deleted)" suffix.
			// This needs to be removed.
			dir = strings.TrimSuffix(dir, " (deleted)")

			// Get start and end index of filename string
			start, _ := rd.Seek(0, io.SeekCurrent)
			end := offset + int64(event.Event_len)

			// Read filename from buf and remove NULL terminator
			filename := unix.ByteSliceToString(buf[start:end])
			eventPath := filepath.Join(dir, filename)

			// Set the offset to the start of the next event
			offset, err = rd.Seek(end, io.SeekStart)
			if err != nil {
				log.Printf("failed to set new offset: %v", err)
				break
			}

			if w.isWatched(eventPath) {
				t := time.Now()
				w.Events <- Event{
					Path:         eventPath,
					Mask:         event.Mask,
					Created:      t,
					LastModified: t,
				}
			}
		}
	}
}
