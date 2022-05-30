//go:build darwin

package watcher

import (
	"github.com/fsnotify/fsevents"
)

func (e Event) Kind() string {

	if e.Mask&uint64(fsevents.EventFlags(fsevents.ItemCreated)) == uint64(fsevents.EventFlags(fsevents.ItemCreated)) {
		return KindCreate
	} else if e.Mask&uint64(fsevents.EventFlags(fsevents.ItemRemoved)) == uint64(fsevents.EventFlags(fsevents.ItemRemoved)) {
		return KindDelete
	} else if e.Mask&uint64(fsevents.EventFlags(fsevents.ItemRenamed)) == uint64(fsevents.EventFlags(fsevents.ItemRenamed)) {
		return KindDelete
	} else if e.Mask&uint64(fsevents.EventFlags(fsevents.ItemModified)) == uint64(fsevents.EventFlags(fsevents.ItemModified)) {
		return KindChange
	} else if e.Mask&uint64(fsevents.EventFlags(fsevents.ItemChangeOwner)) == uint64(fsevents.EventFlags(fsevents.ItemChangeOwner)) {
		return KindChange
	}
	return KindUnknown
}

func debounceEvent(old, new Event) Event {
	switch new.Kind() {
	case KindCreate:
		if old.Kind() == KindDelete {
			// A previously deleted file was recreated. Therefore, the event must be rewritten to a change type
			old.Mask = uint64(fsevents.EventFlags(fsevents.ItemModified))
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
			old.Mask = uint64(fsevents.EventFlags(fsevents.ItemCreated))
		}
		old.LastModified = new.Created
	case KindUnknown:
		old.LastModified = new.Created
	}

	return old
}
