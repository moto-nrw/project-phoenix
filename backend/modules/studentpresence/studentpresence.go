// Package studentpresence owns recorded school attendance and room visits.
package studentpresence

import (
	"context"
	"time"
)

type Query interface {
	GroupSupervisionQuery
	LiveGroupQuery
	OpenRoomSessionQuery
	RoomHistoryQuery
	RoomOccupancyQuery
	CombinedGroupQuery
	AttendanceQuery
	ActivitySessionQuery
	SessionAttendanceQuery
	VisitQuery
	PrivacyConsentQuery
	PresenceRecordQuery
}

// GroupSupervisionQuery reads the staff supervisions of live groups.
type GroupSupervisionQuery interface {
	SupervisedRoomsOn(context.Context, string) ([]StaffRoomSupervision, error)
	StaffIDsWithSupervisionOn(context.Context, string) ([]int64, error)
	QueryGroupSupervisions(context.Context, GroupSupervisionFilter) ([]GroupSupervision, error)
	ListGroupSupervisions(context.Context, int64) ([]GroupSupervision, error)
	LockStaffSupervision(context.Context, int64, string) ([]int64, error)
}

// LiveGroupQuery reads the live room sessions and their room usage.
type LiveGroupQuery interface {
	OccupiedActivityGroupIDs(context.Context, []int64) ([]int64, error)
	QueryLiveGroups(context.Context, LiveGroupFilter) ([]LiveGroup, error)
	ListLiveGroups(context.Context, []int64) ([]LiveGroup, error)
	UnclaimedGroups(context.Context, string) ([]UnclaimedGroup, error)
	RoomUtilization(context.Context, []StudentVisitWindow) ([]RoomUtilization, error)
}

// CombinedGroupQuery reads combined groups and their group mappings.
type CombinedGroupQuery interface {
	GetCombinedGroup(context.Context, int64) (CombinedGroup, error)
	ListCombinedGroups(context.Context, CombinedGroupFilter) ([]CombinedGroup, error)
	ListGroupMappings(context.Context, GroupMappingFilter) ([]GroupMapping, error)
}

// PresenceRecordQuery reads a student's recorded presence for care exits
// and the deletion preview.
type PresenceRecordQuery interface {
	ListOpenPresence(context.Context, []int64) ([]int64, error)
	LatestPresenceDate(context.Context, int64) (*string, error)
	CountAttendanceRecords(context.Context, int64) (int, error)
	CountStudentVisitsForDeletion(context.Context, int64) (int, error)
}

type Command interface {
	GroupSupervisionCommand
	LiveGroupCommand
	CombinedGroupCommand
	AttendanceCommand
	ActivitySessionCommand
	SessionAttendanceCommand
	SessionAttendanceRules
	VisitCommand
	GroupRecovery
	GroupSessionCommand
	PrivacyConsentCommand
	PresenceRecordCommand
}

// GroupSupervisionCommand writes the staff supervisions of live groups.
type GroupSupervisionCommand interface {
	SetSupervisionEnd(context.Context, int64, string, time.Time) (int64, error)
	EndOpenGroupSupervisions(context.Context, int64, int64, string) (int, error)
	RecordSupervision(context.Context, GroupSupervision) (GroupSupervision, error)
	ReviseSupervision(context.Context, GroupSupervision) (GroupSupervision, error)
	RemoveSupervision(context.Context, int64) error
	EndStaffSupervisionsOn(context.Context, int64, string) (int, error)
	EndSupervisionOn(context.Context, int64, string) (int, error)
	ClaimGroup(context.Context, GroupClaim) (ClaimedSupervision, error)
	LockGroupSupervisions(context.Context) error
}

// LiveGroupCommand writes the live room sessions.
type LiveGroupCommand interface {
	RecordGroup(context.Context, LiveGroup) (LiveGroup, error)
	ReviseGroup(context.Context, LiveGroup) (LiveGroup, error)
	DeleteGroup(context.Context, int64) error
	RecordGroupActivity(context.Context, int64, time.Time) error
	LockRoomSessionWrites(context.Context, int64) error
}

// CombinedGroupCommand writes combined groups and their group mappings.
type CombinedGroupCommand interface {
	RecordCombination(context.Context, time.Time, *time.Time) (CombinedGroup, error)
	ReviseCombination(context.Context, int64, time.Time, *time.Time) (CombinedGroup, error)
	DeleteCombination(context.Context, int64) error
	EndCombination(context.Context, int64, time.Time) error
	RecordGroupMapping(context.Context, int64, int64) (GroupMapping, error)
	DeleteGroupMapping(context.Context, int64) error
	AddGroupToCombination(context.Context, int64, int64) error
	RemoveGroupFromCombination(context.Context, int64, int64) error
}

// PresenceRecordCommand locks, closes and restores recorded presence for
// care exits and session recovery.
type PresenceRecordCommand interface {
	LockOpenPresence(context.Context, []int64) error
	CloseOpenPresence(context.Context, []int64, time.Time) (int64, error)
	LockOpenVisits(context.Context, int64) error
	RestoreVisits(context.Context, []int64) error
}

type Capability interface {
	Query
	SupervisedLiveGroupQuery
	Command
}

// Owner is the Student Presence capability its composition hands out,
// together with the attendance retention, the morning attendance (#4790) and
// its retention, and the demo snapshot.
type Owner interface {
	Capability
	AttendanceRetention
	MorningAttendance
	MorningAttendanceRetention
	DemoVisits
}

// StudentVisitWindow is an eligible half-open enrollment interval supplied by
// the report. Windows for each student must be chronological and non-overlapping.
// The query independently applies tenant scope and recorded retention consent.
type StudentVisitWindow struct {
	StudentID int64     `json:"student_id"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
}
type RoomUtilization struct {
	RoomID           int64
	DaysUsed         int
	DistinctStudents int
	StudentMinutes   int
	PeakOccupancy    int
}

type GroupMapping struct {
	ID, TenantID                         int64
	CreatedAt, UpdatedAt                 time.Time
	ActiveCombinedGroupID, ActiveGroupID int64
}
type GroupMappingFilter struct {
	CombinedGroupID *int64
	ActiveGroupID   *int64
}
