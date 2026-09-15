package active

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// TimeTrackingAuditReader supplies the merged feed and its accepted sources.
type TimeTrackingAuditReader interface {
	ListEntries(context.Context, TimeTrackingAuditFilter) ([]*TimeTrackingAuditEntry, error)
	ValidSources() []string
}

type TimeTrackingAuditEntry struct {
	OccurredAt    time.Time
	Source        string
	EntryID       int64
	StaffID       *int64
	ActorStaffID  *int64
	ActorIsSystem bool
	Reason        string
	Detail        json.RawMessage
}

type TimeTrackingAuditCursor struct {
	OccurredAt time.Time
	Source     string
	EntryID    int64
}

type TimeTrackingAuditFilter struct {
	From         *timezone.Date
	To           *timezone.Date
	StaffID      int64
	ActorStaffID int64
	Sources      []string
	Cursor       *TimeTrackingAuditCursor
	Limit        int
}
