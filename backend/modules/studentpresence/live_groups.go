package studentpresence

import (
	"context"
	"time"
)

type StaffRoomSupervision struct{ StaffID, RoomID int64 }

// SupervisedRoomsOn returns distinct staff/room pairs for date-active supervision in open sessions.
func (m *Module) SupervisedRoomsOn(ctx context.Context, date string) ([]StaffRoomSupervision, error) {
	return m.engine.SupervisedRoomsOn(ctx, date)
}

// ListLiveGroups returns tenant-owned room sessions for the supplied IDs, including ended sessions.
func (m *Module) ListLiveGroups(ctx context.Context, ids []int64) ([]LiveGroup, error) {
	return m.engine.ListLiveGroups(ctx, ids)
}

// LiveGroupFilter selects room sessions. Nil ID slices do not restrict IDs;
// non-nil empty slices match nothing. Time bounds overlap inclusively.
type LiveGroupFilter struct {
	EndedOnly          bool
	Limit, Offset      int
	DeviceManagedOnly  bool
	LastActivityBefore *time.Time
	IDs                []int64
	RoomID             *int64
	DeviceID           *int64
	ActivityGroupIDs   []int64
	OpenOnly           bool
	From, Until        *time.Time
}

func (m *Module) QueryLiveGroups(ctx context.Context, filter LiveGroupFilter) ([]LiveGroup, error) {
	return m.engine.QueryLiveGroups(ctx, filter)
}

// SupervisedLiveGroupQuery answers which room sessions a staff member runs.
type SupervisedLiveGroupQuery interface {
	ListSupervisedLiveGroups(context.Context, int64) ([]LiveGroup, error)
	ListOpenLiveGroupsForActivities(context.Context, []int64) ([]LiveGroup, error)
}

// ListSupervisedLiveGroups returns the open room sessions the staff member
// supervises today. The owner decides "today" in the school calendar and
// counts a supervision from its start date up to, not including, its end date.
// Room display facts stay with Facilities; the session carries its RoomID.
func (m *Module) ListSupervisedLiveGroups(ctx context.Context, staffID int64) ([]LiveGroup, error) {
	return m.engine.ListSupervisedLiveGroups(ctx, staffID)
}

// ListOpenLiveGroupsForActivities returns the open room sessions of the given
// activity groups. Unlike LiveGroupFilter, nil or empty IDs match nothing.
func (m *Module) ListOpenLiveGroupsForActivities(ctx context.Context, activityGroupIDs []int64) ([]LiveGroup, error) {
	return m.engine.ListOpenLiveGroupsForActivities(ctx, activityGroupIDs)
}

func (m *Module) OccupiedActivityGroupIDs(ctx context.Context, ids []int64) ([]int64, error) {
	return m.engine.OccupiedActivityGroupIDs(ctx, ids)
}

type GroupSupervision struct {
	ID, TenantID, GroupID, StaffID int64
	CreatedAt, UpdatedAt           time.Time
	Role                           string
	StartDate                      string
	EndDate                        *string
}

func (m *Module) ListGroupSupervisions(ctx context.Context, groupID int64) ([]GroupSupervision, error) {
	return m.engine.ListGroupSupervisions(ctx, groupID)
}

// GroupSupervisionFilter combines its predicates with AND. Nil ID slices are
// unrestricted; non-nil empty slices match nothing. EndedBy includes that date.
type GroupSupervisionFilter struct {
	StaffIDs      []int64
	EndedBy       *string
	Limit, Offset int
	IDs           []int64
	OpenOnly      bool
	StartedBefore *string
	// ForUpdate requires a caller-owned tenant transaction.
	ForUpdate bool
	GroupIDs  []int64
	StaffID   *int64
	ActiveOn  *string
}

func (m *Module) QueryGroupSupervisions(ctx context.Context, filter GroupSupervisionFilter) ([]GroupSupervision, error) {
	return m.engine.QueryGroupSupervisions(ctx, filter)
}

// StaffIDsWithSupervisionOn includes supervision starting or ending on the date.
func (m *Module) StaffIDsWithSupervisionOn(ctx context.Context, date string) ([]int64, error) {
	return m.engine.StaffIDsWithSupervisionOn(ctx, date)
}
