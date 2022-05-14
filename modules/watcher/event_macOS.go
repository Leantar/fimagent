//go:build macOS

package watcher

import (
	"github.com/fsnotify/fsevents"
)

func (e Event) Kind() string {

	if e.Mask&uint64(fsevents.EventFlag.ItemCreated) == uint64(fsevents.EventFlag.ItemCreated) {
		return KindCreate
	} else if e.Mask&uint64(fsevents.EventFlag.ItemRemoved) == uint64(fsevents.EventFlag.ItemRemoved) {
		return KindDelete
	} else if e.Mask&uint64(fsevents.EventFlag.ItemRenamed) == uint64(fsevents.EventFlag.ItemRenamed) {
		return KindDelete
	} else if e.Mask&uint64(fsevents.EventFlag.ItemModified) == uint64(fsevents.EventFlag.ItemModified) {
		return KindChange
	} else if e.Mask&uint64(fsevents.EventFlag.ItemChangeOwner) == uint64(fsevents.EventFlag.ItemChangeOwner) {
		return KindChange
	}

	return KindUnknown
}

func debounceEvent(old, new Event) Event {
	switch new.Kind() {
	case KindCreate:
		if old.Kind() == KindDelete {
			// A previously deleted file was recreated. Therefore, the event must be rewritten to a change type
			old.Mask = ItemModified
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
			old.Mask = ItemCreated
		}
		old.LastModified = new.Created
	case KindUnknown:
		old.LastModified = new.Created
	}

	return old
}
