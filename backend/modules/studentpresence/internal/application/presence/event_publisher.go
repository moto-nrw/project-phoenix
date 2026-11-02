package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// EventPublisher announces the presence changes to the live views. The root
// binds it to the realtime transport. Every announcement is fire-and-forget:
// a failed fan-out is logged by the binding and never fails the write.
type EventPublisher interface {
	// VisitCheckedIn and VisitCheckedOut announce a room check-in or
	// check-out to the session topic and, when known, the child's
	// educational group topic.
	VisitCheckedIn(ctx context.Context, change studentpresence.LiveVisitChange)
	VisitCheckedOut(ctx context.Context, change studentpresence.LiveVisitChange)
	// RoomlessAttendanceChanged announces an attendance change without room
	// context to the child's educational group topic only.
	RoomlessAttendanceChanged(ctx context.Context, checkIn bool, change studentpresence.LiveVisitChange)
	// BulkStudentChange announces one batched check-in or check-out to a
	// session topic; groupIDs are the affected educational groups, nil keeps
	// the field absent.
	BulkStudentChange(ctx context.Context, checkIn bool, activeGroupID string, studentIDs, groupIDs []string)
	// BulkStudentChangeToEducationGroup announces the part of a batch that
	// belongs to one educational group on that group's topic.
	BulkStudentChangeToEducationGroup(ctx context.Context, checkIn bool, activeGroupID string, educationGroupID int64, studentIDs []string)
	// SessionStarted and SessionEnded announce a session to its topic.
	SessionStarted(ctx context.Context, session studentpresence.LiveSessionChange)
	SessionEnded(ctx context.Context, session studentpresence.LiveSessionChange)
	// SupervisionRefresh sends the tenant-wide refresh used by attendance and
	// activity changes; it carries no child identity (#2085).
	SupervisionRefresh(ctx context.Context, activeGroupID, reason string, eduGroupIDs []string)
	// DashboardCountsChanged sends the tenant-wide dashboard refresh (#2057).
	DashboardCountsChanged(ctx context.Context, eduGroupIDs []string)
	// SupervisionChanged queues the tenant-wide active_supervision_changed
	// for a session until the surrounding transaction commits (#5017); it
	// carries no child identity.
	SupervisionChanged(ctx context.Context, activeGroupID int64, reason string)
}
