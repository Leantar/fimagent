//go:build windows

package watcher

import (
	"github.com/fsnotify/fsnotify"
)

func (e Event) Kind() string {
	if e.Mask&uint64(fsnotify.Create) == uint64(fsnotify.Create) {
		return KindCreate
	} else if e.Mask&uint64(fsnotify.Remove) == uint64(fsnotify.Remove) {
		return KindDelete
	} else if e.Mask&uint64(fsnotify.Write) == uint64(fsnotify.Write) {
		return KindChange
	} else if e.Mask&uint64(fsnotify.Rename) == uint64(fsnotify.Rename) {
		return KindDelete
	} else if e.Mask&uint64(fsnotify.Chmod) == uint64(fsnotify.Chmod) {
		return KindChange
	}

	return KindUnknown
}

func debounceEvent(old, new Event) Event {
	switch new.Kind() {
	case KindCreate:
		if old.Kind() == KindDelete {
			// A previously deleted file was recreated. Therefore, the event must be rewritten to a change type
			old.Mask = uint64(fsnotify.Write)
		} else {
			old.Mask = new.Mask
		}
		old.LastModified = new.Created
	case KindDelete:
		old.Mask = new.Mask
		old.LastModified = new.Created
	case KindChange:
		// Sometimes on creation of a file a "CHANGE" event gets emitted instead of a "CREATE".
		// We handle it like in the "CREATE" case
		if old.Kind() == KindDelete {
			old.Mask = uint64(fsnotify.Write)
		}
		old.LastModified = new.Created
	case KindUnknown:
		old.LastModified = new.Created
	}

	return old
}
