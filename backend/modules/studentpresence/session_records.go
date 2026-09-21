package studentpresence

import (
	"context"
	"time"
)

// SessionRecords reads and records room sessions for the callers that open,
// lock and inspect them directly: timetable planning, working time, the
// device dashboard, the substitution overview and the caller context. The
// presence commands (SessionCommands, KioskSessions) remain the way to run a
// session with its supervision and visits.
type SessionRecords interface {
	// CreateSession validates and stores the session and copies the stored
	// identity, tenant and timestamps back into group.
	CreateSession(ctx context.Context, group *LiveGroup) error
	// FindSession reads one of the tenant's sessions; a missing one is a
	// not-found error.
	FindSession(ctx context.Context, id int64) (*LiveGroup, error)
	// FindByIDForUpdate locks the session for the caller's transaction and
	// returns nil, without an error, when the tenant has no such session.
	FindByIDForUpdate(ctx context.Context, id int64) (*LiveGroup, error)
	// FindByIDs returns the sessions with their room and activity template.
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*SessionDetail, error)
	FindActiveByRoomID(ctx context.Context, roomID int64) ([]*LiveGroup, error)
	// FindActiveByDeviceIDWithNames returns the device's open session with the
	// names of its room and activity template, or nil when it runs none.
	FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*SessionDetail, error)
	// FindActiveGroups returns every open session, oldest start first.
	FindActiveGroups(ctx context.Context) ([]*LiveGroup, error)
	// CheckRoomConflict reports another open session occupying the room.
	// Independent room stays (device-less sessions of a system activity) do
	// not count as occupancy.
	CheckRoomConflict(ctx context.Context, roomID, excludeGroupID int64) (bool, *LiveGroup, error)
	UpdateLastActivity(ctx context.Context, id int64, at time.Time) error
	GetOccupiedActivityGroupIDs(ctx context.Context, activityGroupIDs []int64) (map[int64]bool, error)
	// SessionRows returns the sessions with their room and activity template
	// in the JSON shape the caller-context routes render.
	SessionRows(ctx context.Context, ids []int64) (any, error)
}

// SupervisionRecords reads and records the supervision of room sessions for
// the same callers. "Today" is the school calendar date of the owner's clock.
type SupervisionRecords interface {
	// CreateSupervision validates and stores the supervision and copies the
	// stored identity, tenant and timestamps back into supervision.
	CreateSupervision(ctx context.Context, supervision *GroupSupervision) error
	// FindByActiveGroupID returns the session's supervision; activeOnly keeps
	// the supervision whose date interval includes today.
	FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*StaffedSupervision, error)
	FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*StaffedSupervision, error)
	// FindActiveByStaffID returns the staff member's supervision of today.
	FindActiveByStaffID(ctx context.Context, staffID int64) ([]*GroupSupervision, error)
	// EndByActiveGroupAndStaffID ends the staff member's open supervision of
	// one session. Zero matches is not an error.
	EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error)
	// EndAllActiveByStaffID ends every supervision of the staff member that
	// is active today and returns how many ended.
	EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error)
	// GetStaffIDsWithSupervisionToday returns the staff members whose
	// supervision started, ended or runs today.
	GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error)
	ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]SupervisionBlocker, error)
	// ListActiveSupervisedRooms returns one staff/room pair per room a staff
	// member supervises in an open session today.
	ListActiveSupervisedRooms(ctx context.Context) ([]StaffRoomSupervision, error)
}

// StaffedSupervision is a supervision together with the display name of its
// staff member. StaffName is empty when the staff member or their person
// record cannot be resolved.
type StaffedSupervision struct {
	GroupSupervision
	StaffName string
}

// SupervisionBlocker is a still-open supervision that blocks removing a
// staff member's caregiver capability. GroupName is filled by the
// composition root from School Structure.
type SupervisionBlocker struct {
	ID        int64
	GroupID   int64
	GroupName string
	StartDate string
}

// TemplateID returns the activity template's ID and true when the session is
// template-backed, or (0, false) for a spontaneous session.
func (g LiveGroup) TemplateID() (int64, bool) {
	if g.ActivityGroupID == nil {
		return 0, false
	}
	return *g.ActivityGroupID, true
}

// IsIndependentRoomSession reports a device-less stay under a system
// activity: the room's own session (#3066), not occupancy of an activity
// running there. activityIsSystem is the template's is_system flag.
func (g LiveGroup) IsIndependentRoomSession(activityIsSystem bool) bool {
	return g.DeviceID == nil && g.ActivityGroupID != nil && activityIsSystem
}
