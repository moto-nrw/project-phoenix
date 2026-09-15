package domain

import "time"

type FeedCancellation struct {
	Source      string
	SourceID    int64
	Title       string
	EventDate   string
	StartTime   time.Time
	EndTime     time.Time
	CancelledAt time.Time
}
