package studentpresence

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StaffRoomSupervision struct{ StaffID, RoomID int64 }

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

// SupervisedLiveGroupQuery answers which room sessions a staff member runs.
type SupervisedLiveGroupQuery interface {
	ListSupervisedLiveGroups(context.Context, int64) ([]LiveGroup, error)
	ListOpenLiveGroupsForActivities(context.Context, []int64) ([]LiveGroup, error)
}

// SupervisionStaffIncludesRemoved states that a supervision row keeps
// naming its staff member after the staff record was removed: the
// supervision history outlives the colleague, so the staff lookup behind
// supervision rows includes soft-deleted staff.
const SupervisionStaffIncludesRemoved = true

type GroupSupervision struct {
	ID, TenantID, GroupID, StaffID int64
	CreatedAt, UpdatedAt           time.Time
	Role                           string
	StartDate                      string
	EndDate                        *string
	// SkipPresenceStamp keeps an external caregiver out of time tracking when
	// they are added to an already-running supervision. It is command-only and
	// never stored with the supervision.
	SkipPresenceStamp bool `json:"-"`
}

// EndedOn reports whether the supervision no longer runs on the day. It
// matches ActiveOn: a planned future end date is still supervising, and a
// supervision without a readable end date has not ended.
func (s GroupSupervision) EndedOn(day calendar.Date) bool {
	if s.EndDate == nil {
		return false
	}
	end, err := calendar.ParseDate(*s.EndDate)
	if err != nil {
		return false
	}
	return !day.Before(end)
}

// StartedOn reports whether the live group started on the day.
func (g LiveGroup) StartedOn(day calendar.Date) bool {
	return calendar.DateFromTime(g.StartTime) == day
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
