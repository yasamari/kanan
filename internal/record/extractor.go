package record

import "time"

type Info struct {
	Path               string
	BroadcastStartTime time.Time
	BroadcastDuration  time.Duration
	ServiceID          int
}

type InfoExtractor interface {
	Extract(path string) (Info, error)
}
