package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type Stats struct {
	Queries           int
	Rows              int64
	StatementDuration time.Duration
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats
	Err error
}

type Store interface {
	SetSupervisionEnd(context.Context, int64, Date, time.Time) (int64, Stats, error)
	EndOpenGroupSupervisions(context.Context, int64, int64, Date) (int, Stats, error)
	RecordSupervision(context.Context, GroupSupervision) (GroupSupervision, Stats, error)
	ReviseSupervision(context.Context, GroupSupervision) (GroupSupervision, Stats, error)
	RemoveSupervision(context.Context, int64) (Stats, error)
	EndStaffSupervisionsOn(context.Context, int64, Date) (int, Stats, error)
	EndSupervisionOn(context.Context, int64, Date) (int, Stats, error)
	SupervisedRoomsOn(context.Context, Date) ([]studentpresence.StaffRoomSupervision, Stats, error)
	StaffIDsWithSupervisionOn(context.Context, Date) ([]int64, Stats, error)
	RecordGroup(context.Context, studentpresence.LiveGroup) (studentpresence.LiveGroup, Stats, error)
	ReviseGroup(context.Context, studentpresence.LiveGroup) (studentpresence.LiveGroup, Stats, error)
	DeleteGroup(context.Context, int64) (Stats, error)
	QueryGroupSupervisions(context.Context, GroupSupervisionFilter) ([]GroupSupervision, Stats, error)
	OccupiedActivityGroupIDs(context.Context, []int64) ([]int64, Stats, error)
	RecordGroupActivity(context.Context, int64, time.Time) (Stats, error)
	LockRoomSessionWrites(context.Context, int64) (Stats, error)
	ListOpenRoomSessions(context.Context, []int64) ([]studentpresence.OpenRoomSession, Stats, error)
	ListRoomSessionHistory(context.Context, int64, time.Time, time.Time, *int64) ([]studentpresence.RoomSessionHistory, Stats, error)
	ListRoomOccupancy(context.Context, []int64) ([]studentpresence.RoomOccupancy, Stats, error)
	QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, Stats, error)
	GetCombinedGroup(context.Context, int64) (studentpresence.CombinedGroup, Stats, error)
	RecordCombination(context.Context, time.Time, *time.Time) (studentpresence.CombinedGroup, Stats, error)
	ReviseCombination(context.Context, int64, time.Time, *time.Time) (studentpresence.CombinedGroup, Stats, error)
	DeleteCombination(context.Context, int64) (Stats, error)
	ListCombinedGroups(context.Context, studentpresence.CombinedGroupFilter) ([]studentpresence.CombinedGroup, Stats, error)
	EndCombination(context.Context, int64, time.Time) (Stats, error)
	RecordGroupMapping(context.Context, int64, int64) (studentpresence.GroupMapping, Stats, error)
	DeleteGroupMapping(context.Context, int64) (Stats, error)
	ListGroupMappings(context.Context, studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, Stats, error)
	AddGroupToCombination(context.Context, int64, int64) (Stats, error)
	RemoveGroupFromCombination(context.Context, int64, int64) (Stats, error)
	RoomUtilization(context.Context, []studentpresence.StudentVisitWindow) ([]studentpresence.RoomUtilization, Stats, error)
	LockStaffSupervision(context.Context, int64, Date) ([]int64, Stats, error)
	UnclaimedStore
	AttendanceStore
	MorningAttendanceStore
	ClassHandoverStore
	ActivitySessionStore
	SessionAttendanceStore
	VisitStore
	GroupSessionStore
	PrivacyConsentStore
	LatestPresenceDate(context.Context, int64) (*string, Stats, error)
	CountAttendanceRecords(context.Context, int64) (int, Stats, error)
	CountStudentVisitsForDeletion(context.Context, int64) (int, Stats, error)
	LockOpenPresence(context.Context, []int64) (Stats, error)
	LockVisitSessions(context.Context, int64, int64) (Stats, error)
	CloseOpenPresence(context.Context, []int64, time.Time) (Stats, error)
	ListOpenPresence(context.Context, []int64) ([]int64, Stats, error)
	LockOpenVisits(context.Context, int64) (Stats, error)
	RestoreVisits(context.Context, []int64) (Stats, error)
	LockOpenSupervisors(context.Context, int64) (Stats, error)
	LockSupervisors(context.Context, []int64) (Stats, error)
	LockGroupSupervisions(context.Context) (Stats, error)
	RestoreGroup(context.Context, int64, time.Time) (Stats, error)
	RestoreSupervisors(context.Context, []int64) (Stats, error)
}

type Transaction interface {
	Run(context.Context, func(context.Context) error) error
	Require(context.Context) error
}
