package timetracking

import (
	"context"
	"encoding/json"
	"time"
)

// TimeTrackingDeletionAudit records the full snapshot before its source is deleted.
type TimeTrackingDeletionAudit interface {
	Create(context.Context, *TimeTrackingDeletionEvent) error
}

type TimeTrackingDeletionEvent struct {
	StaffID    int64
	Source     string
	SourceID   int64
	DeletedBy  int64
	Payload    json.RawMessage
	Note       string
	OccurredAt time.Time
}
