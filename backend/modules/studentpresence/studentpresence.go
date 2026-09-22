// Package studentpresence owns recorded school attendance and room visits.
package studentpresence

import (
	"context"
	"time"
)

type Query interface {
	SupervisedRoomsOn(context.Context, string) ([]StaffRoomSupervision, error)
	StaffIDsWithSupervisionOn(context.Context, string) ([]int64, error)
	QueryGroupSupervisions(context.Context, GroupSupervisionFilter) ([]GroupSupervision, error)
	ListGroupSupervisions(context.Context, int64) ([]GroupSupervision, error)
	OccupiedActivityGroupIDs(context.Context, []int64) ([]int64, error)
	QueryLiveGroups(context.Context, LiveGroupFilter) ([]LiveGroup, error)
	OpenRoomSessionQuery
	RoomHistoryQuery
	RoomOccupancyQuery
	ListLiveGroups(context.Context, []int64) ([]LiveGroup, error)
	GetCombinedGroup(context.Context, int64) (CombinedGroup, error)
	ListCombinedGroups(context.Context, CombinedGroupFilter) ([]CombinedGroup, error)
	ListGroupMappings(context.Context, GroupMappingFilter) ([]GroupMapping, error)
	RoomUtilization(context.Context, []StudentVisitWindow) ([]RoomUtilization, error)
	LockStaffSupervision(context.Context, int64, string) ([]int64, error)
	UnclaimedGroups(context.Context, string) ([]UnclaimedGroup, error)
	AttendanceQuery
	VisitQuery
	PrivacyConsentQuery
	ListOpenPresence(context.Context, []int64) ([]int64, error)
	LatestPresenceDate(context.Context, int64) (*string, error)
	CountAttendanceRecords(context.Context, int64) (int, error)
	CountStudentVisitsForDeletion(context.Context, int64) (int, error)
}

// LockStaffSupervision returns the supervision IDs active on the given school
// day and holds their row locks until the caller's tenant transaction ends.
// The caller must also serialize staff lifecycle changes with assignment
// writers: these row locks alone do not prevent new supervision rows.
func (m *Module) LockStaffSupervision(ctx context.Context, staffID int64, date string) ([]int64, error) {
	return m.engine.LockStaffSupervision(ctx, staffID, date)
}

type Command interface {
	SetSupervisionEnd(context.Context, int64, string, time.Time) (int64, error)
	EndOpenGroupSupervisions(context.Context, int64, int64, string) (int, error)
	RecordSupervision(context.Context, GroupSupervision) (GroupSupervision, error)
	ReviseSupervision(context.Context, GroupSupervision) (GroupSupervision, error)
	RemoveSupervision(context.Context, int64) error
	EndStaffSupervisionsOn(context.Context, int64, string) (int, error)
	EndSupervisionOn(context.Context, int64, string) (int, error)
	RecordGroup(context.Context, LiveGroup) (LiveGroup, error)
	ReviseGroup(context.Context, LiveGroup) (LiveGroup, error)
	DeleteGroup(context.Context, int64) error
	RecordGroupActivity(context.Context, int64, time.Time) error
	LockRoomSessionWrites(context.Context, int64) error
	RecordCombination(context.Context, time.Time, *time.Time) (CombinedGroup, error)
	ReviseCombination(context.Context, int64, time.Time, *time.Time) (CombinedGroup, error)
	DeleteCombination(context.Context, int64) error
	EndCombination(context.Context, int64, time.Time) error
	RecordGroupMapping(context.Context, int64, int64) (GroupMapping, error)
	DeleteGroupMapping(context.Context, int64) error
	AddGroupToCombination(context.Context, int64, int64) error
	RemoveGroupFromCombination(context.Context, int64, int64) error
	ClaimGroup(context.Context, GroupClaim) (ClaimedSupervision, error)
	AttendanceCommand
	VisitCommand
	GroupRecovery
	GroupSessionCommand
	PrivacyConsentCommand
	LockOpenPresence(context.Context, []int64) error
	CloseOpenPresence(context.Context, []int64, time.Time) (int64, error)
	LockOpenVisits(context.Context, int64) error
	RestoreVisits(context.Context, []int64) error
	LockGroupSupervisions(context.Context) error
}

// LatestPresenceDate returns the last attendance or visit day as YYYY-MM-DD.
// Visit instants are interpreted in the school's Europe/Berlin calendar.
func (m *Module) LatestPresenceDate(ctx context.Context, studentID int64) (*string, error) {
	return m.engine.LatestPresenceDate(ctx, studentID)
}

// CountAttendanceRecords counts the student's attendance rows plus scheduled
// checkouts; the deletion preview reports both as attendance records.
func (m *Module) CountAttendanceRecords(ctx context.Context, studentID int64) (int, error) {
	return m.engine.CountAttendanceRecords(ctx, studentID)
}

// CountStudentVisitsForDeletion counts every visit of the child, including
// holiday-care visits hosted by another tenant that the home tenant's RLS
// hides. The permanent-deletion preview reports them because the cascade
// removes them with the child.
func (m *Module) CountStudentVisitsForDeletion(ctx context.Context, studentID int64) (int, error) {
	return m.engine.CountStudentVisitsForDeletion(ctx, studentID)
}

func (m *Module) LockOpenPresence(ctx context.Context, studentIDs []int64) error {
	return m.engine.LockOpenPresence(ctx, studentIDs)
}

// LockGroupSupervisions takes a SHARE ROW EXCLUSIVE table lock on the live
// group supervision rows for the caller's transaction, so a caregiver
// capability re-check cannot race a concurrent supervision write. Callers
// serializing staff lifecycle changes take it alongside the other binding
// owners' locks.
func (m *Module) LockGroupSupervisions(ctx context.Context) error {
	return m.engine.LockGroupSupervisions(ctx)
}

// CloseOpenPresence closes all recorded attendance and visits for a care exit.
// It joins the caller's transaction so roster and care-plan writes remain atomic.
func (m *Module) CloseOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	return m.engine.CloseOpenPresence(ctx, studentIDs, at)
}

type Capability interface {
	Query
	SupervisedLiveGroupQuery
	Command
}

// Module exposes presence operations without leaking persistence models.
type Module struct{ engine Capability }

func NewModule(engine Capability) *Module {
	if engine == nil {
		panic("student presence: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) ListOpenPresence(ctx context.Context, studentIDs []int64) ([]int64, error) {
	return m.engine.ListOpenPresence(ctx, studentIDs)
}

// LockOpenVisits holds the current group's visit locks until the surrounding
// workflow commits. It requires an existing tenant transaction.
func (m *Module) LockOpenVisits(ctx context.Context, activeGroupID int64) error {
	return m.engine.LockOpenVisits(ctx, activeGroupID)
}

// RestoreVisits reopens exactly the closed visits in a completion snapshot.
// A mismatch fails the surrounding recovery transaction.
func (m *Module) RestoreVisits(ctx context.Context, visitIDs []int64) error {
	return m.engine.RestoreVisits(ctx, visitIDs)
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

func (m *Module) RoomUtilization(ctx context.Context, windows []StudentVisitWindow) ([]RoomUtilization, error) {
	return m.engine.RoomUtilization(ctx, windows)
}

func (m *Module) AddGroupToCombination(ctx context.Context, combinedID, groupID int64) error {
	return m.engine.AddGroupToCombination(ctx, combinedID, groupID)
}
func (m *Module) RemoveGroupFromCombination(ctx context.Context, combinedID, groupID int64) error {
	return m.engine.RemoveGroupFromCombination(ctx, combinedID, groupID)
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

func (m *Module) ListGroupMappings(ctx context.Context, filter GroupMappingFilter) ([]GroupMapping, error) {
	return m.engine.ListGroupMappings(ctx, filter)
}

func (m *Module) RecordGroupMapping(ctx context.Context, combinedID, groupID int64) (GroupMapping, error) {
	return m.engine.RecordGroupMapping(ctx, combinedID, groupID)
}
func (m *Module) DeleteGroupMapping(ctx context.Context, id int64) error {
	return m.engine.DeleteGroupMapping(ctx, id)
}
