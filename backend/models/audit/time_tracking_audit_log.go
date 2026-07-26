package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// Time-tracking audit log sources (#1417). One value per change trail that
// feeds the cross-staff audit view.
const (
	AuditLogSourceSessionEdit = "session_edit"
	AuditLogSourceAbsence     = "absence"
	AuditLogSourceAdjustment  = "adjustment"
	AuditLogSourceMonthClose  = "month_close"
	AuditLogSourceMonthReopen = "month_reopen"
	AuditLogSourceDeletion    = "deletion"
)

// ValidAuditLogSources lists every accepted `sources` filter value.
var ValidAuditLogSources = []string{
	AuditLogSourceSessionEdit,
	AuditLogSourceAbsence,
	AuditLogSourceAdjustment,
	AuditLogSourceMonthClose,
	AuditLogSourceMonthReopen,
	AuditLogSourceDeletion,
}

// TimeTrackingAuditLogEntry is one event in the merged feed: the common
// envelope (when, who was affected, who acted, why) plus a source-typed
// detail payload. Session edits of one save action and school-wide month
// closes arrive pre-grouped as single events.
type TimeTrackingAuditLogEntry struct {
	OccurredAt    time.Time       `bun:"occurred_at"`
	Source        string          `bun:"source"`
	EntryID       int64           `bun:"entry_id"`
	StaffID       *int64          `bun:"staff_id"`
	ActorStaffID  *int64          `bun:"actor_staff_id"`
	ActorIsSystem bool            `bun:"actor_is_system"`
	Reason        string          `bun:"reason"`
	Detail        json.RawMessage `bun:"detail"`
}

// TimeTrackingAuditLogCursor is the keyset cursor: strictly-before position
// in (occurred_at, source, entry_id) descending order.
type TimeTrackingAuditLogCursor struct {
	OccurredAt time.Time
	Source     string
	EntryID    int64
}

// TimeTrackingAuditLogFilter narrows the feed. Zero values mean "no filter".
type TimeTrackingAuditLogFilter struct {
	From    *timezone.Date
	To      *timezone.Date
	StaffID int64
	// ActorStaffID filters by acting person (staff id). The absence trail
	// stores account ids; the query resolves them to staff ids so this
	// filter is uniform across sources.
	ActorStaffID int64
	Sources      []string
	Cursor       *TimeTrackingAuditLogCursor
	Limit        int
}

// TimeTrackingAuditLogRepository merges the change trails into one
// keyset-paginated feed.
type TimeTrackingAuditLogRepository interface {
	ListEntries(ctx context.Context, filter TimeTrackingAuditLogFilter) ([]*TimeTrackingAuditLogEntry, error)
}
