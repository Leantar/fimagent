//go:build darwin

package watcher

import (
	"github.com/fsnotify/fsevents"
)

func (e Event) Kind() string {
	if e.Mask&uint64(fsevents.ItemCreated) == uint64(fsevents.ItemCreated) {
		return KindCreate
	} else if e.Mask&uint64(fsevents.ItemRemoved) == uint64(fsevents.ItemRemoved) {
		return KindDelete
	} else if e.Mask&uint64(fsevents.ItemRenamed) == uint64(fsevents.ItemRenamed) {
		return KindDelete
	}
	return KindChange
}

func debounceEvent(old, new Event) Event {
	print("Event triggered")
	switch new.Kind() {
	case KindCreate:
		if old.Kind() == KindDelete {
			// A previously deleted file was recreated. Therefore, the event must be rewritten to a change type
			old.Mask = uint64(fsevents.ItemModified)
		} else {
			old.Mask = new.Mask
		}
		old.LastModified = new.Created
	case KindDelete:
		old.Mask = new.Mask
		old.LastModified = new.Created
	case KindChange:
		if old.Kind() == KindDelete {
			// Sometimes on creation of a file a "CHANGE" event gets emitted instead of a "CREATE".
			// We handle it like in the "CREATE" case
			old.Mask = uint64(fsevents.ItemCreated)
		}
		old.LastModified = new.Created
	}

	return old
}
