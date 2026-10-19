package httpintegration_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The absorption (#2161) and the reopen's student locks were unit tests of
// the retained service's unexported steps with in-memory Student Presence
// doubles. The owner's lifecycle may not name Student Presence from its own
// internal tests, so these suites drive the same steps through the public
// Start and Reopen with the same doubles; the database only supplies the
// tenant transaction and the day lock. Every id below is an in-memory
// sentinel, not a database row.

// newFakeLifecycle composes the lifecycle over the given doubles; every
// collaborator the transition under test does not reach is an unused stub.
func newFakeLifecycle(t *testing.T, deps timetablePorts.InstanceLifecycleDependencies) fixtures.InstanceLifecycle {
	t.Helper()
	deps.IdempotencyRepo = fixtures.IntoInstanceIdempotencyRecords(struct {
		fixtures.
			InstanceIdempotencyRecords
	}{})
	deps.ExceptionRepo = fixtures.IntoActivityExceptionRecords(struct {
		fixtures.
			ActivityExceptionRecords
	}{})
	deps.CalendarPeriodRepo = fixtures.IntoCalendarPeriodRecords(struct {
		fixtures.
			CalendarPeriodRecords
	}{})
	deps.StaffRepo = struct{ timetablePorts.LifecycleStaff }{}
	deps.ActiveService = struct{ timetablePorts.SessionEnder }{}
	deps.CareDays = struct{ timetablePorts.CareDays }{}
	deps.CareDayLocks = struct{ timetablePorts.CareDayLocks }{}
	deps.Protocol = struct {
		timetablePorts.DeviationProtocol
	}{}
	deps.Materialization = struct {
		timetable.MaterializationCapability
	}{}
	deps.RecurrenceLock = struct{ timetable.RecurrenceWriteLock }{}
	deps.SubstituteConflicts = struct {
		timetable.SubstituteConflictQuery
	}{}
	deps.StartConflicts = noStartConflicts{}
	deps.ContentHash = securityruntime.Fingerprint
	deps.Database = compose.NewTenantDatabase(testpkg.SetupTestDB(t))
	deps.Logger = slog.New(slog.DiscardHandler)
	if fixtures.FromInstanceStaffRecords(deps.InstanceStaffRepo) == nil {
		deps.InstanceStaffRepo = fixtures.IntoInstanceStaffRecords(fakeNoInstanceStaff{})
	}
	if deps.Rooms == nil {
		deps.Rooms = fakeLockableRoom{}
	}
	if deps.ActivityGroupRepo == nil {
		deps.ActivityGroupRepo = &absorbActivityGroupRepo{}
	}
	if deps.StudentRepo == nil {
		deps.StudentRepo = &reopenStudentLockStub{}
	}
	if deps.SupervisorRepo == nil {
		deps.SupervisorRepo = &absorbSupervisorRepo{}
	}
	if fixtures.FromInstanceStudentRecords(deps.InstanceStudents) == nil {
		deps.InstanceStudents = fixtures.IntoInstanceStudentRecords(&absorbInstanceStudentRepo{})
	}
	if fixtures.FromActivityRecoveryRecords(deps.RecoveryRepo) == nil {
		deps.RecoveryRepo = fixtures.IntoActivityRecoveryRecords(fakeRecovery{})
	}
	return newLifecycle(t, deps)
}

type noStartConflicts struct{}

func (noStartConflicts) DetectStartConflicts(context.Context, timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
	return nil, nil
}

type fakeNoInstanceStaff struct {
	fixtures.
		InstanceStaffRecords
}

func (fakeNoInstanceStaff) FindByInstanceID(context.Context, int64) ([]*fixtures.InstanceStaff, error) {
	return nil, nil
}

// fakeLockableRoom is an existing room of unlimited capacity.
type fakeLockableRoom struct {
	timetablePorts.LifecycleRooms
}

func (fakeLockableRoom) LockRoom(context.Context, int64) (*int, bool, error) {
	return nil, true, nil
}

type fakeRecovery struct {
	fixtures.
		ActivityRecoveryRecords
}

func (fakeRecovery) LockAttendance(context.Context, int64) error { return nil }

func (fakeRecovery) Restore(context.Context, int64, fixtures.ActivityCompletionSnapshot, time.Time) error {
	return nil
}

// --- absorption of unsupervised sessions on start (#2161) ------------------

// absorbPlannedInstance is the planned block the absorption tests start;
// the reload under the day lock returns it unchanged.
type absorbInstanceRepo struct {
	fixtures.
		ActivityInstanceRecords
	planned *fixtures.ActivityInstance
	byGroup map[int64]*fixtures.ActivityInstance
}

func (r *absorbInstanceRepo) FindByID(_ context.Context, _ int64) (*fixtures.ActivityInstance, error) {
	planned := *r.planned
	return &planned, nil
}

func (r *absorbInstanceRepo) FindByActiveGroupID(_ context.Context, groupID int64) (*fixtures.ActivityInstance, error) {
	return r.byGroup[groupID], nil
}

func (r *absorbInstanceRepo) UpdateColumns(context.Context, *fixtures.ActivityInstance, ...string) (int64, error) {
	return 1, nil
}

func absorbPlannedInstance(instanceID, roomID int64) *fixtures.ActivityInstance {
	planned := fixtures.NewActivityInstance(func(fixtureRow *fixtures.ActivityInstance) {
		fixtureRow.Date = calendar.NewDate(2026, 4, 20)
		fixtureRow.Title = "Absorb"
		fixtureRow.StartTime = time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
		fixtureRow.EndTime = time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
		fixtureRow.RoomID = roomID
		fixtureRow.Status = fixtures.InstanceStatusPlanned()
	})

	planned.ID = instanceID
	return planned
}

type absorbGroupRepo struct {
	studentpresence.SessionRecords
	newGroupID   int64
	openGroups   []*studentpresence.LiveGroup
	lockedGroups map[int64]*studentpresence.LiveGroup
	lockedIDs    []int64
}

// CreateSession stores the started block's session under newGroupID.
func (r *absorbGroupRepo) CreateSession(_ context.Context, group *studentpresence.LiveGroup) error {
	group.ID = r.newGroupID
	return nil
}

func (r *absorbGroupRepo) FindActiveByRoomID(_ context.Context, _ int64) ([]*studentpresence.LiveGroup, error) {
	return r.openGroups, nil
}

func (r *absorbGroupRepo) FindByIDForUpdate(_ context.Context, id int64) (*studentpresence.LiveGroup, error) {
	r.lockedIDs = append(r.lockedIDs, id)
	if group := r.lockedGroups[id]; group != nil {
		return group, nil
	}
	for _, group := range r.openGroups {
		if group.ID == id {
			return group, nil
		}
	}
	return nil, nil
}

type absorbSupervisorRepo struct {
	studentpresence.SupervisionRecords
	byGroup map[int64][]*studentpresence.StaffedSupervision
}

func (r *absorbSupervisorRepo) FindByActiveGroupID(_ context.Context, groupID int64, _ bool) ([]*studentpresence.StaffedSupervision, error) {
	return r.byGroup[groupID], nil
}

type absorbVisitRepo struct {
	timetablePorts.LifecyclePresence
	visits    []studentpresence.Visit
	transfers [][2]int64
	endedIDs  []int64
}

func (r *absorbVisitRepo) EndGroup(_ context.Context, id int64, _ time.Time) error {
	r.endedIDs = append(r.endedIDs, id)
	return nil
}

type absorbActivityGroupRepo struct {
	timetablePorts.LifecycleActivityGroups
	byID map[int64]*activityContract.Group
}

func (r *absorbActivityGroupRepo) FindByIDs(_ context.Context, ids []int64) ([]*activityContract.Group, error) {
	out := make([]*activityContract.Group, 0, len(ids))
	for _, id := range ids {
		if group := r.byID[id]; group != nil {
			out = append(out, group)
		}
	}
	return out, nil
}

func (r *absorbVisitRepo) TransferOpenVisits(_ context.Context, oldGroupID, newGroupID int64) (int64, error) {
	r.transfers = append(r.transfers, [2]int64{oldGroupID, newGroupID})
	moved := int64(0)
	for i := range r.visits {
		visit := &r.visits[i]
		if visit.ActiveGroupID == oldGroupID && visit.ExitTime == nil {
			visit.ActiveGroupID = newGroupID
			moved++
		}
	}
	return moved, nil
}

func (r *absorbVisitRepo) ListVisits(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	visits := make([]studentpresence.Visit, 0)
	for _, visit := range r.visits {
		if visit.ActiveGroupID == filter.ActiveGroupIDs[0] {
			visits = append(visits, visit)
		}
	}
	return visits, nil
}

type absorbInstanceStudentRepo struct {
	fixtures.
		InstanceStudentRecords
	unplannedStudentID int64
	updates            []absorbedAttendanceUpdate
	lookups            []absorbedAttendanceUpdate
	creates            []absorbedAttendanceUpdate
}

type absorbedAttendanceUpdate struct {
	instanceID int64
	studentID  int64
	checkedIn  time.Time
}

func (r *absorbInstanceStudentRepo) UpdateAttendanceFromCheckin(
	_ context.Context, instanceID, studentID int64, checkedIn time.Time,
) (bool, error) {
	r.updates = append(r.updates, absorbedAttendanceUpdate{
		instanceID: instanceID,
		studentID:  studentID,
		checkedIn:  checkedIn,
	})
	return studentID != r.unplannedStudentID, nil
}

func (r *absorbInstanceStudentRepo) FindByInstanceAndStudent(
	_ context.Context, instanceID, studentID int64,
) (*fixtures.InstanceStudent, error) {
	r.lookups = append(r.lookups, absorbedAttendanceUpdate{
		instanceID: instanceID,
		studentID:  studentID,
	})
	return nil, nil
}

func (r *absorbInstanceStudentRepo) CreateUnplannedPresentIfAbsent(
	_ context.Context, instanceID, studentID int64, checkedIn time.Time,
) (*fixtures.InstanceStudent, error) {
	r.creates = append(r.creates, absorbedAttendanceUpdate{
		instanceID: instanceID,
		studentID:  studentID,
		checkedIn:  checkedIn,
	})
	return fixtures.NewInstanceStudent(func(fixtureRow *fixtures.InstanceStudent) {
	}), nil
}

func (r *absorbInstanceStudentRepo) FindByInstanceIDs(_ context.Context, instanceIDs []int64) ([]*fixtures.InstanceStudent, error) {
	if len(instanceIDs) > 0 && r.unplannedStudentID > 0 {
		r.lookups = append(r.lookups, absorbedAttendanceUpdate{
			instanceID: instanceIDs[0],
			studentID:  r.unplannedStudentID,
		})
	}
	return nil, nil
}

func TestInstanceStart_DoesNotAbsorbGroupMovedAfterCandidateLookup(t *testing.T) {
	t.Parallel()

	now := time.Now()
	candidate := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	candidate.ID = 11
	locked := *candidate
	locked.RoomID = 99
	groupRepo := &absorbGroupRepo{
		newGroupID:   10,
		openGroups:   []*studentpresence.LiveGroup{candidate},
		lockedGroups: map[int64]*studentpresence.LiveGroup{candidate.ID: &locked},
	}
	visitRepo := &absorbVisitRepo{}
	svc := newFakeLifecycle(t, timetablePorts.InstanceLifecycleDependencies{
		InstanceRepo:     fixtures.IntoActivityInstanceRecords(&absorbInstanceRepo{planned: absorbPlannedInstance(9, 42)}),
		InstanceStudents: fixtures.IntoInstanceStudentRecords(&absorbInstanceStudentRepo{}),
		ActiveGroupRepo:  groupRepo,
		SupervisorRepo:   &absorbSupervisorRepo{},
		Presence:         visitRepo,
	})

	_, err := svc.Start(testpkg.Ctx(t), 9, 0)

	require.NoError(t, err)
	assert.Equal(t, []int64{candidate.ID}, groupRepo.lockedIDs)
	assert.Empty(t, visitRepo.transfers)
	assert.Empty(t, visitRepo.endedIDs)
}

// A started instance absorbs open sessions WITHOUT active supervisors in its
// room (their open visits move over, the orphan session ends) and leaves
// supervised parallel sessions alone (#2161, sanctioned pattern per #2139).
func TestInstanceStart_AbsorbsUnsupervisedOpenGroups(t *testing.T) {
	t.Parallel()

	const (
		instanceID         int64 = 9
		newGroupID         int64 = 10
		studentID          int64 = 21
		unplannedStudentID int64 = 22
	)

	now := time.Now()
	newGroup := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	newGroup.ID = newGroupID
	unsupervised := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	unsupervised.ID = 11
	supervised := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	supervised.ID = 12
	bridged := &studentpresence.LiveGroup{StartTime: now, RoomID: 42}
	bridged.ID = 13
	staleFallback := &studentpresence.LiveGroup{StartTime: now.AddDate(0, 0, -1), RoomID: 42}
	staleFallback.ID = 14
	systemActivityID := int64(88)
	independent := &studentpresence.LiveGroup{StartTime: now, RoomID: 42, ActivityGroupID: &systemActivityID}
	independent.ID = 15

	groupRepo := &absorbGroupRepo{
		newGroupID: newGroupID,
		openGroups: []*studentpresence.LiveGroup{newGroup, unsupervised, supervised, bridged, staleFallback, independent},
	}
	supervisorRepo := &absorbSupervisorRepo{byGroup: map[int64][]*studentpresence.StaffedSupervision{
		12: {{GroupSupervision: studentpresence.GroupSupervision{StaffID: 7, GroupID: 12}}},
	}}
	entryTime := now.Add(-15 * time.Minute)
	visitRepo := &absorbVisitRepo{visits: []studentpresence.Visit{
		{
			StudentID:     studentID,
			ActiveGroupID: unsupervised.ID,
			EntryTime:     entryTime,
		},
		{
			StudentID:     unplannedStudentID,
			ActiveGroupID: unsupervised.ID,
			EntryTime:     entryTime,
		},
	}}
	instanceStudents := &absorbInstanceStudentRepo{unplannedStudentID: unplannedStudentID}
	instanceRepo := &absorbInstanceRepo{planned: absorbPlannedInstance(instanceID, 42), byGroup: map[int64]*fixtures.ActivityInstance{
		13: fixtures.NewActivityInstance(func(fixtureRow *fixtures.ActivityInstance) {
			fixtureRow.Date = calendar.Date(calendar.TodayDate())
			fixtureRow.Status = fixtures.InstanceStatusActive()
			fixtureRow.ActiveGroupID = &bridged.ID
		}),
	}}
	systemActivity := &activityContract.Group{IsSystem: true}
	systemActivity.ID = systemActivityID

	svc := newFakeLifecycle(t, timetablePorts.InstanceLifecycleDependencies{
		InstanceRepo:      fixtures.IntoActivityInstanceRecords(instanceRepo),
		InstanceStudents:  fixtures.IntoInstanceStudentRecords(instanceStudents),
		ActiveGroupRepo:   groupRepo,
		ActivityGroupRepo: &absorbActivityGroupRepo{byID: map[int64]*activityContract.Group{systemActivityID: systemActivity}},
		SupervisorRepo:    supervisorRepo,
		Presence:          visitRepo,
	})

	result, err := svc.Start(testpkg.Ctx(t), instanceID, 0)

	require.NoError(t, err)
	assert.Equal(t, newGroupID, result.ActiveGroupID)
	assert.Equal(t, []int64{11, 12, 13}, groupRepo.lockedIDs, "only today's candidate sessions are locked")
	assert.Equal(t, []int64{11}, visitRepo.endedIDs, "only the unbridged unsupervised session is ended")
	assert.Equal(t, [][2]int64{{11, newGroupID}}, visitRepo.transfers, "open visits move through the conditional bulk update")
	assert.Equal(t, []absorbedAttendanceUpdate{{
		instanceID: instanceID,
		studentID:  studentID,
		checkedIn:  entryTime,
	}, {
		instanceID: instanceID,
		studentID:  unplannedStudentID,
		checkedIn:  entryTime,
	}}, instanceStudents.updates, "absorbed visit is mirrored into the planned attendance row")
	assert.Equal(t, []absorbedAttendanceUpdate{{
		instanceID: instanceID,
		studentID:  unplannedStudentID,
	}}, instanceStudents.lookups, "missing planned attendance is confirmed before inserting")
	assert.Equal(t, []absorbedAttendanceUpdate{{
		instanceID: instanceID,
		studentID:  unplannedStudentID,
		checkedIn:  entryTime,
	}}, instanceStudents.creates, "unplanned absorbed student gets a present attendance row")
}

// --- reopen: lock the snapshot's children before the room --------------------

type reopenVisitStub struct {
	timetablePorts.LifecyclePresence
	byGroup map[int64][]studentpresence.Visit
	current map[int64]studentpresence.Visit
	findErr error
}

func (r *reopenVisitStub) ListVisits(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if len(filter.ActiveGroupIDs) > 0 {
		return r.byGroup[filter.ActiveGroupIDs[0]], nil
	}
	var visits []studentpresence.Visit
	for _, studentID := range filter.StudentIDs {
		if visit, ok := r.current[studentID]; ok {
			visits = append(visits, visit)
		}
	}
	return visits, nil
}

type reopenStudentLockStub struct {
	timetablePorts.LifecycleStudents
	locked []int64
}

func (s *reopenStudentLockStub) FindByIDForUpdate(_ context.Context, id int64) (*timetable.LifecycleStudent, error) {
	s.locked = append(s.locked, id)
	return &timetable.LifecycleStudent{ID: id}, nil
}

func (s *reopenStudentLockStub) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	students := make(map[int64]*timetable.LifecycleStudent, len(ids))
	for _, id := range ids {
		student, err := s.FindByIDForUpdate(ctx, id)
		if err != nil {
			return nil, err
		}
		students[id] = student
	}
	return students, nil
}

// FindReadScopeByIDs answers the restored visits' invalidation: no child
// has an education group here.
func (s *reopenStudentLockStub) FindReadScopeByIDs(context.Context, []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return map[int64]*timetable.LifecycleStudent{}, nil
}

// reopenSessionRepo reports the snapshot's room as free.
type reopenSessionRepo struct {
	studentpresence.SessionRecords
}

func (reopenSessionRepo) CheckRoomConflict(context.Context, int64, int64) (bool, *studentpresence.LiveGroup, error) {
	return false, nil, nil
}

// reopenCompletedInstance is a completed block of group 90 inside its reopen
// window whose snapshot names the given closed visits.
type reopenInstanceRepo struct {
	fixtures.
		ActivityInstanceRecords
	completed *fixtures.ActivityInstance
}

func (r *reopenInstanceRepo) FindByID(context.Context, int64) (*fixtures.ActivityInstance, error) {
	completed := *r.completed
	return &completed, nil
}

func (r *reopenInstanceRepo) UpdateColumns(context.Context, *fixtures.ActivityInstance, ...string) (int64, error) {
	return 1, nil
}

func reopenCompletedInstance(t *testing.T, visitIDs []int64) *reopenInstanceRepo {
	t.Helper()
	snapshot, err := json.Marshal(*fixtures.NewActivityCompletionSnapshot(func(fixtureRow *fixtures.ActivityCompletionSnapshot) {
		fixtureRow.ActiveGroupID = 90
		fixtureRow.VisitIDs = visitIDs
	}))
	require.NoError(t, err)
	reopenUntil := time.Now().Add(time.Hour)
	completed := fixtures.NewActivityInstance(func(fixtureRow *fixtures.ActivityInstance) {
		fixtureRow.Date = calendar.NewDate(2026, 4, 20)
		fixtureRow.Title = "Reopen"
		fixtureRow.StartTime = time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
		fixtureRow.EndTime = time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
		fixtureRow.RoomID = 42
		fixtureRow.Status = fixtures.InstanceStatusCompleted()
		fixtureRow.ReopenUntil = &reopenUntil
		fixtureRow.CompletionSnapshot = snapshot
	})

	completed.ID = 5
	return &reopenInstanceRepo{completed: completed}
}

func TestLockReopenSnapshotStudents_LocksSortedUniqueStudents(t *testing.T) {
	t.Parallel()

	visitA := studentpresence.Visit{StudentID: 52}
	visitA.ID = 20
	visitB := studentpresence.Visit{StudentID: 41}
	visitB.ID = 21
	visitDup := studentpresence.Visit{StudentID: 52}
	visitDup.ID = 22
	students := &reopenStudentLockStub{}
	broadcaster := testpkg.NewRecordingBroadcaster()
	deps := timetablePorts.InstanceLifecycleDependencies{
		InstanceRepo: fixtures.IntoActivityInstanceRecords(reopenCompletedInstance(t, []int64{22, 20, 21})),
		Presence: &reopenVisitStub{byGroup: map[int64][]studentpresence.Visit{
			90: {visitA, visitB, visitDup},
		}},
		StudentRepo:     students,
		ActiveGroupRepo: reopenSessionRepo{},
		Broadcaster:     lifecycleRealtime{broadcaster: broadcaster},
	}
	svc := newFakeLifecycle(t, deps)

	_, err := svc.Reopen(testpkg.Ctx(t), 5, 0, true)
	require.NoError(t, err)
	assert.Equal(t, []int64{41, 52}, students.locked)
	// The locked children are the ones the restored visits announce.
	restored := broadcaster.GroupCallsForTopic("90")
	var restoredIDs []string
	for _, call := range restored {
		if call.Event.Type == realtime.EventBulkStudentCheckIn {
			require.NotNil(t, call.Event.Data.StudentIDs)
			restoredIDs = *call.Event.Data.StudentIDs
		}
	}
	assert.Equal(t, []string{"41", "52"}, restoredIDs)
}

func TestLockReopenSnapshotStudents_RejectsActiveVisit(t *testing.T) {
	t.Parallel()

	visit := studentpresence.Visit{StudentID: 52}
	visit.ID = 20
	current := studentpresence.Visit{StudentID: 52}
	current.ID = 99
	svc := newFakeLifecycle(t, timetablePorts.InstanceLifecycleDependencies{
		InstanceRepo: fixtures.IntoActivityInstanceRecords(reopenCompletedInstance(t, []int64{20})),
		Presence: &reopenVisitStub{
			byGroup: map[int64][]studentpresence.Visit{90: {visit}},
			current: map[int64]studentpresence.Visit{52: current},
		},
		StudentRepo:     &reopenStudentLockStub{},
		ActiveGroupRepo: reopenSessionRepo{},
	})

	_, err := svc.Reopen(testpkg.Ctx(t), 5, 0, true)
	require.ErrorIs(t, err, timetable.ErrTimetableOperationConflict)
}
