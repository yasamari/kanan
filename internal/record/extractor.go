package record

import "time"

type Info struct {
	StartTime time.Time
	Duration  time.Duration
	ServiceID int
}

type InfoExtractor interface {
	Extract(path string) (Info, error)
}
