package watcher

import (
	"time"
)

const (
	KindCreate  = "CREATE"
	KindDelete  = "DELETE"
	KindChange  = "CHANGE"
	KindUnknown = "UNKNOWN"
)

type Event struct {
	Path         string
	Mask         uint64
	Created      time.Time
	LastModified time.Time
}
