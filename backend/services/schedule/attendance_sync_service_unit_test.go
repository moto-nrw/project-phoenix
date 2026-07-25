// Unit tests for AttendanceSyncService that don't require a database.
//
// The integration tests in attendance_sync_service_integration_test.go cover
// the happy-path branches (B1, B3, B5, B6, B9) via real repos. The
// graceful-degradation error branches (B2 instance lookup error, B4
// instance_student lookup error, B7 UPDATE error, B8 race, and the panic
// recovery paths) are not hermetically reachable with real DB code — RLS
// hides rows rather than returning errors, and transient Postgres failures
// cannot be reliably injected without extra infrastructure.
//
// This file closes the coverage gap by stubbing both repositories with
// hand-rolled fakes that return the shapes each branch expects.
package schedule_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	modelsBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Fake ActivityInstanceRepository — only FindByActiveGroupID is used by the
// service. Other methods panic if called so a regression that adds a new
// dependency fails loudly rather than silently returning zero-values.
// -----------------------------------------------------------------------------

type fakeInstanceRepo struct {
	findErr   error
	instance  *scheduleModel.ActivityInstance
	findPanic any
}

func (f *fakeInstanceRepo) FindByActiveGroupID(_ context.Context, _ int64) (*scheduleModel.ActivityInstance, error) {
	if f.findPanic != nil {
		panic(f.findPanic)
	}
	return f.instance, f.findErr
}

// Unused interface methods — panic keeps them from accidentally hiding a
// regression that adds a new dependency to the service.
func (f *fakeInstanceRepo) Create(context.Context, *scheduleModel.ActivityInstance) error {
	panic("unused")
}

func (f *fakeInstanceRepo) CreateTemplateBackedIfAbsent(context.Context, *scheduleModel.ActivityInstance) (bool, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) FindByID(context.Context, interface{}) (*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) Update(context.Context, *scheduleModel.ActivityInstance) error {
	panic("unused")
}

func (f *fakeInstanceRepo) Delete(context.Context, interface{}) error {
	panic("unused")
}

func (f *fakeInstanceRepo) List(context.Context, *modelsBase.QueryOptions) ([]*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) FindByTenantAndDate(context.Context, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) FindByTenantAndDateRange(context.Context, timezone.Date, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) FindByActivityGroupAndDate(context.Context, int64, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) FindByActivityGroupAndDateRange(context.Context, int64, timezone.Date, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) MarkCompleted(context.Context, int64, time.Time) error {
	panic("unused")
}

// -----------------------------------------------------------------------------
// Fake InstanceStudentRepository — controls FindByInstanceAndStudent and
// UpdateAttendanceFromCheckin outcomes for every branch.
// -----------------------------------------------------------------------------

type fakeInstanceStudentRepo struct {
	findRow        *scheduleModel.InstanceStudent
	findErr        error
	updateResult   bool
	updateErr      error
	updateCalls    int
	unplannedRow   *scheduleModel.InstanceStudent
	unplannedErr   error
	unplannedCalls int
	candidates     []*scheduleModel.InstanceStudent
	candidateErr   error
	candidatePanic any
	dateRows       []*scheduleModel.InstanceStudent
	dateErr        error
	datePanic      any
	checkoutErr    error
	checkoutCalls  int
	checkoutID     int64
	reconcileErr   error
	reconcileCalls int
	previousIn     time.Time
	previousOut    *time.Time
	updatedIn      time.Time
	updatedOut     *time.Time
}

func (f *fakeInstanceStudentRepo) FindByInstanceAndStudent(_ context.Context, _, _ int64) (*scheduleModel.InstanceStudent, error) {
	return f.findRow, f.findErr
}

func (f *fakeInstanceStudentRepo) UpdateAttendanceFromCheckin(_ context.Context, _, _ int64, _ time.Time) (bool, error) {
	f.updateCalls++
	return f.updateResult, f.updateErr
}

func (f *fakeInstanceStudentRepo) CreateUnplannedPresentIfAbsent(context.Context, int64, int64, time.Time) (*scheduleModel.InstanceStudent, error) {
	f.unplannedCalls++
	return f.unplannedRow, f.unplannedErr
}

func (f *fakeInstanceStudentRepo) UpdateAttendanceCheckout(_ context.Context, instanceID int64, _ int64, _ time.Time) error {
	f.checkoutCalls++
	f.checkoutID = instanceID
	return f.checkoutErr
}

func (f *fakeInstanceStudentRepo) ReconcileAttendanceInterval(
	_ context.Context,
	_, _ int64,
	previousIn time.Time,
	previousOut *time.Time,
	updatedIn time.Time,
	updatedOut *time.Time,
) (bool, error) {
	f.reconcileCalls++
	f.previousIn = previousIn
	f.previousOut = previousOut
	f.updatedIn = updatedIn
	f.updatedOut = updatedOut
	return f.reconcileErr == nil, f.reconcileErr
}

func (f *fakeInstanceStudentRepo) FindCurrentCandidates(context.Context, int64, timezone.Date, time.Time) ([]*scheduleModel.InstanceStudent, error) {
	if f.candidatePanic != nil {
		panic(f.candidatePanic)
	}
	return f.candidates, f.candidateErr
}

func (f *fakeInstanceStudentRepo) ApplyStatusDay(context.Context, int64, timezone.Date, int64, string) (int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) ReleaseStatusDay(context.Context, int64) (int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) ApplyActiveStatusDaysForInstance(context.Context, int64, timezone.Date) (int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) Create(context.Context, *scheduleModel.InstanceStudent) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindByID(context.Context, interface{}) (*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) Update(context.Context, *scheduleModel.InstanceStudent) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) Delete(context.Context, interface{}) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) List(context.Context, *modelsBase.QueryOptions) ([]*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindByInstanceID(context.Context, int64) ([]*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindByInstanceIDs(context.Context, []int64) ([]*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindExpectedByInstanceIDs(context.Context, []int64) ([]*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindNotScheduledCandidatesByInstanceIDs(context.Context, []int64) ([]*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindByStudentAndDateRange(context.Context, int64, timezone.Date, timezone.Date) ([]*scheduleModel.InstanceStudent, error) {
	if f.datePanic != nil {
		panic(f.datePanic)
	}
	return f.dateRows, f.dateErr
}

func (f *fakeInstanceStudentRepo) FindPlannedStudentIDsByDate(context.Context, []int64, timezone.Date) ([]int64, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) DeleteByInstanceID(context.Context, int64) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) ArchivePlannedByStudentIDsFrom(context.Context, int64, []int64, timezone.Date) (int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) RestoreArchivedByTransition(context.Context, int64, []int64, timezone.Date) (int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) UpdateAttendanceFields(context.Context, int64, scheduleModel.AttendanceFieldPatch) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) BulkUpdateStatus(context.Context, int64, string, string, []int64) (int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) MarkNotScheduled(context.Context, []scheduleModel.StudentInstanceRef) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindInstancesWithAttendanceByStudentAndDateRange(context.Context, int64, timezone.Date, timezone.Date) ([]*scheduleModel.ScheduledInstanceRow, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) HasPlannedSlotsInRange(context.Context, timezone.Date, timezone.Date) (bool, error) {
	panic("unused")
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func newSilentLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logger, buf
}

func newUnitSyncer(instRepo *fakeInstanceRepo, isRepo *fakeInstanceStudentRepo) *scheduleSvc.AttendanceSyncService {
	logger, _ := newSilentLogger()
	return scheduleSvc.NewAttendanceSyncService(instRepo, isRepo, logger)
}

func validVisit() *activeModel.Visit {
	return &activeModel.Visit{
		StudentID:     100,
		ActiveGroupID: 42,
		EntryTime:     time.Date(2026, 4, 20, 14, 0, 0, 0, time.UTC),
	}
}

func instanceWithID(id int64) *scheduleModel.ActivityInstance {
	inst := &scheduleModel.ActivityInstance{}
	inst.ID = id
	return inst
}

func expectedRow(id int64) *scheduleModel.InstanceStudent {
	row := &scheduleModel.InstanceStudent{
		InstanceID: 42,
		StudentID:  100,
		Status:     scheduleModel.AttendanceStatusExpected,
	}
	row.ID = id
	return row
}

// -----------------------------------------------------------------------------
// MirrorCheckInForVisit — error branches
// -----------------------------------------------------------------------------

func TestMirrorCheckIn_B1_NilVisit(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.MirrorCheckInForVisit(context.Background(), nil))
}

func TestMirrorCheckIn_B1_ZeroActiveGroup(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{})
	v := validVisit()
	v.ActiveGroupID = 0
	assert.Nil(t, syncer.MirrorCheckInForVisit(context.Background(), v))
}

func TestMirrorCheckIn_B1_NegativeActiveGroup(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{})
	v := validVisit()
	v.ActiveGroupID = -1
	assert.Nil(t, syncer.MirrorCheckInForVisit(context.Background(), v))
}

func TestMirrorCheckIn_B2_InstanceLookupError(t *testing.T) {
	instRepo := &fakeInstanceRepo{findErr: errors.New("connection lost")}
	isRepo := &fakeInstanceStudentRepo{}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	assert.Nil(t, snap, "instance lookup error must return nil (graceful degradation)")
	assert.Equal(t, 0, isRepo.updateCalls, "must not attempt UPDATE after lookup failure")
}

func TestMirrorCheckIn_B3_NoInstance(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: nil}, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.MirrorCheckInForVisit(context.Background(), validVisit()))
}

func TestMirrorCheckIn_B4_InstanceStudentLookupError(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findErr: errors.New("scan error")}
	syncer := newUnitSyncer(instRepo, isRepo)

	assert.Nil(t, syncer.MirrorCheckInForVisit(context.Background(), validVisit()))
	assert.Equal(t, 0, isRepo.updateCalls)
}

func TestMirrorCheckIn_B5_NoInstanceStudentRow(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findRow: nil, unplannedRow: &scheduleModel.InstanceStudent{
		InstanceID: 7, StudentID: validVisit().StudentID,
		Status: scheduleModel.AttendanceStatusPresent, IsUnplanned: true,
	}}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	require.NotNil(t, snap)
	assert.True(t, snap.IsUnplanned)
	assert.Equal(t, 1, isRepo.unplannedCalls)
	assert.Equal(t, 0, isRepo.updateCalls, "no row → skip UPDATE")
}

func TestMirrorCheckIn_UnplannedPersistenceError(t *testing.T) {
	isRepo := &fakeInstanceStudentRepo{unplannedErr: errors.New("insert failed")}
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: instanceWithID(7)}, isRepo)

	assert.Nil(t, syncer.MirrorCheckInForVisit(context.Background(), validVisit()))
	assert.Equal(t, 1, isRepo.unplannedCalls)
}

func TestMirrorCheckIn_B6_AlreadyPresent_NoClobber(t *testing.T) {
	late := scheduleModel.AttendanceSubstatusLate
	note := "bus late"
	row := expectedRow(3)
	row.Status = scheduleModel.AttendanceStatusPresent
	row.Substatus = &late
	row.Note = &note

	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findRow: row}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	require.NotNil(t, snap)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, snap.Status)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusLate, *snap.Substatus)
	require.NotNil(t, snap.Note)
	assert.Equal(t, "bus late", *snap.Note)
	assert.Equal(t, 0, isRepo.updateCalls, "must NOT UPDATE when row is already past expected")
}

func TestMirrorCheckIn_ReopensCheckedOutPresentRow(t *testing.T) {
	checkedIn := time.Now().Add(-2 * time.Hour)
	checkedOut := time.Now().Add(-time.Hour)
	row := expectedRow(4)
	row.Status = scheduleModel.AttendanceStatusPresent
	row.CheckedInAt = &checkedIn
	row.CheckedOutAt = &checkedOut
	isRepo := &fakeInstanceStudentRepo{findRow: row, updateResult: true}
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: instanceWithID(7)}, isRepo)

	visit := validVisit()
	snap := syncer.MirrorCheckInForVisit(context.Background(), visit)

	require.NotNil(t, snap)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, snap.Status)
	assert.Equal(t, 1, isRepo.updateCalls)
	assert.Nil(t, row.CheckedOutAt, "re-entry must reopen the slot observation")
	// The reopen re-stamps checked_in_at (session boundary): a delayed
	// checkout from the superseded interval must not close the reopened slot.
	require.NotNil(t, row.CheckedInAt)
	assert.Equal(t, visit.EntryTime, *row.CheckedInAt, "reopen must re-stamp check-in with the re-entry time")
}

func TestMirrorCheckInAt_AssignsOnlyOneCurrentBookedSlot(t *testing.T) {
	row := expectedRow(11)
	row.InstanceID = 77
	isRepo := &fakeInstanceStudentRepo{candidates: []*scheduleModel.InstanceStudent{row}, updateResult: true}
	syncer := newUnitSyncer(&fakeInstanceRepo{}, isRepo)

	snapshot := syncer.MirrorCheckInAt(context.Background(), row.StudentID, time.Now())

	require.NotNil(t, snapshot)
	assert.Equal(t, int64(77), snapshot.InstanceID)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, snapshot.Status)
	assert.Equal(t, 1, isRepo.updateCalls)
}

func TestMirrorCheckInAt_ReopensCheckedOutPresentRow(t *testing.T) {
	checkedIn := time.Now().Add(-2 * time.Hour)
	checkedOut := time.Now().Add(-time.Hour)
	row := expectedRow(17)
	row.InstanceID = 78
	row.Status = scheduleModel.AttendanceStatusPresent
	row.CheckedInAt = &checkedIn
	row.CheckedOutAt = &checkedOut
	isRepo := &fakeInstanceStudentRepo{candidates: []*scheduleModel.InstanceStudent{row}, updateResult: true}
	syncer := newUnitSyncer(&fakeInstanceRepo{}, isRepo)

	reentry := time.Now()
	snapshot := syncer.MirrorCheckInAt(context.Background(), row.StudentID, reentry)

	require.NotNil(t, snapshot)
	assert.Equal(t, 1, isRepo.updateCalls)
	assert.Nil(t, row.CheckedOutAt, "binary-mode re-entry must reopen the slot")
	require.NotNil(t, row.CheckedInAt)
	assert.Equal(t, reentry, *row.CheckedInAt, "reopen must re-stamp check-in with the re-entry time")
}

func TestMirrorCheckInAt_AmbiguousSlotsStayUnassigned(t *testing.T) {
	first := expectedRow(12)
	second := expectedRow(13)
	isRepo := &fakeInstanceStudentRepo{candidates: []*scheduleModel.InstanceStudent{first, second}}
	syncer := newUnitSyncer(&fakeInstanceRepo{}, isRepo)

	assert.Nil(t, syncer.MirrorCheckInAt(context.Background(), first.StudentID, time.Now()))
	assert.Equal(t, 0, isRepo.updateCalls)
}

func TestMirrorCheckInAt_HandlesLookupAndUpdateFailures(t *testing.T) {
	t.Run("lookup error", func(t *testing.T) {
		syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{candidateErr: errors.New("lookup failed")})
		assert.Nil(t, syncer.MirrorCheckInAt(context.Background(), 100, time.Now()))
	})

	t.Run("update error", func(t *testing.T) {
		row := expectedRow(14)
		syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{
			candidates: []*scheduleModel.InstanceStudent{row},
			updateErr:  errors.New("update failed"),
		})
		assert.Nil(t, syncer.MirrorCheckInAt(context.Background(), row.StudentID, time.Now()))
	})

	t.Run("panic", func(t *testing.T) {
		syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{candidatePanic: "boom"})
		require.NotPanics(t, func() {
			assert.Nil(t, syncer.MirrorCheckInAt(context.Background(), 100, time.Now()))
		})
	})
}

func TestMirrorCheckInAt_PreservesManualStatusAndClearsDayStatus(t *testing.T) {
	t.Run("manual status", func(t *testing.T) {
		row := expectedRow(15)
		row.Status = scheduleModel.AttendanceStatusAbsent
		isRepo := &fakeInstanceStudentRepo{candidates: []*scheduleModel.InstanceStudent{row}}
		snapshot := newUnitSyncer(&fakeInstanceRepo{}, isRepo).MirrorCheckInAt(context.Background(), row.StudentID, time.Now())

		require.NotNil(t, snapshot)
		assert.Equal(t, scheduleModel.AttendanceStatusAbsent, snapshot.Status)
		assert.Zero(t, isRepo.updateCalls)
	})

	t.Run("day status", func(t *testing.T) {
		statusDayID := int64(90)
		sick := scheduleModel.AttendanceSubstatusSick
		row := expectedRow(16)
		row.Status = scheduleModel.AttendanceStatusAbsent
		row.Substatus = &sick
		row.StudentStatusDayID = &statusDayID
		isRepo := &fakeInstanceStudentRepo{candidates: []*scheduleModel.InstanceStudent{row}, updateResult: true}
		snapshot := newUnitSyncer(&fakeInstanceRepo{}, isRepo).MirrorCheckInAt(context.Background(), row.StudentID, time.Now())

		require.NotNil(t, snapshot)
		assert.Equal(t, scheduleModel.AttendanceStatusPresent, snapshot.Status)
		assert.Nil(t, snapshot.Substatus)
	})
}

func TestMirrorCheckIn_B6_Absent_NoClobber(t *testing.T) {
	row := expectedRow(3)
	row.Status = scheduleModel.AttendanceStatusAbsent

	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findRow: row}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	require.NotNil(t, snap)
	assert.Equal(t, scheduleModel.AttendanceStatusAbsent, snap.Status)
	assert.Equal(t, 0, isRepo.updateCalls)
}

func TestMirrorCheckIn_B7_UpdateError(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{
		findRow:   expectedRow(3),
		updateErr: errors.New("FK violation"),
	}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	assert.Nil(t, snap, "UPDATE error → nil (graceful, logged at Error for ops)")
	assert.Equal(t, 1, isRepo.updateCalls)
}

func TestMirrorCheckIn_B8_RaceNoRowsAffected(t *testing.T) {
	row := expectedRow(3)
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{
		findRow:      row,
		updateResult: false, // no rows matched (race — row moved out of 'expected')
	}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	require.NotNil(t, snap, "race → snapshot of read row so SSE still fires")
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, snap.Status)
}

func TestMirrorCheckIn_B9_HappyPath(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{
		findRow:      expectedRow(3),
		updateResult: true,
	}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	require.NotNil(t, snap)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, snap.Status)
	assert.Equal(t, 1, isRepo.updateCalls)
}

func TestMirrorCheckIn_B9_HappyPathWithSubstatusAndNote(t *testing.T) {
	// B9 must propagate existing substatus/note from the row it just flipped.
	excused := scheduleModel.AttendanceSubstatusExcused
	note := "Arzttermin"
	row := expectedRow(3)
	row.Substatus = &excused
	row.Note = &note

	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findRow: row, updateResult: true}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
	require.NotNil(t, snap)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusExcused, *snap.Substatus)
	require.NotNil(t, snap.Note)
	assert.Equal(t, "Arzttermin", *snap.Note)
}

func TestMirrorCheckIn_PanicRecovery(t *testing.T) {
	// Exercise the defer/recover belt-and-braces. The instance lookup panics;
	// the service must swallow it and return nil without propagating.
	instRepo := &fakeInstanceRepo{findPanic: "boom"}
	isRepo := &fakeInstanceStudentRepo{}
	syncer := newUnitSyncer(instRepo, isRepo)

	var snap *activeSvcSnapshot
	require.NotPanics(t, func() {
		result := syncer.MirrorCheckInForVisit(context.Background(), validVisit())
		if result != nil {
			snap = &activeSvcSnapshot{Status: result.Status}
		}
	})
	assert.Nil(t, snap, "panic in instance lookup must be recovered → nil snapshot")
}

// activeSvcSnapshot is a local copy used only to assert shape without
// importing the active services package under a different alias.
type activeSvcSnapshot struct {
	Status string
}

// -----------------------------------------------------------------------------
// MirrorCheckOutForVisit — error branches
// -----------------------------------------------------------------------------

func TestLoadAttendance_NilVisit(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.MirrorCheckOutForVisit(context.Background(), nil))
}

func TestLoadAttendance_ZeroActiveGroup(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{})
	v := validVisit()
	v.ActiveGroupID = 0
	assert.Nil(t, syncer.MirrorCheckOutForVisit(context.Background(), v))
}

func TestLoadAttendance_InstanceLookupError(t *testing.T) {
	instRepo := &fakeInstanceRepo{findErr: errors.New("db down")}
	syncer := newUnitSyncer(instRepo, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.MirrorCheckOutForVisit(context.Background(), validVisit()))
}

func TestLoadAttendance_NoInstance(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: nil}, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.MirrorCheckOutForVisit(context.Background(), validVisit()))
}

func TestLoadAttendance_InstanceStudentLookupError(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findErr: errors.New("scan failed")}
	syncer := newUnitSyncer(instRepo, isRepo)
	assert.Nil(t, syncer.MirrorCheckOutForVisit(context.Background(), validVisit()))
}

func TestLoadAttendance_NoRow(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findRow: nil}
	syncer := newUnitSyncer(instRepo, isRepo)
	assert.Nil(t, syncer.MirrorCheckOutForVisit(context.Background(), validVisit()))
}

func TestLoadAttendance_HappyPath(t *testing.T) {
	late := scheduleModel.AttendanceSubstatusLate
	note := "bus delay"
	row := expectedRow(5)
	row.Status = scheduleModel.AttendanceStatusPresent
	row.Substatus = &late
	row.Note = &note

	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findRow: row}
	syncer := newUnitSyncer(instRepo, isRepo)

	snap := syncer.MirrorCheckOutForVisit(context.Background(), validVisit())
	require.NotNil(t, snap)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, snap.Status)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusLate, *snap.Substatus)
	require.NotNil(t, snap.Note)
	assert.Equal(t, "bus delay", *snap.Note)
}

func TestLoadAttendance_RecordsCheckout(t *testing.T) {
	exit := time.Date(2026, 4, 20, 15, 0, 0, 0, time.UTC)
	visit := validVisit()
	visit.ExitTime = &exit
	row := expectedRow(6)
	checkedIn := exit.Add(-time.Hour)
	row.Status = scheduleModel.AttendanceStatusPresent
	row.CheckedInAt = &checkedIn
	isRepo := &fakeInstanceStudentRepo{findRow: row}
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: instanceWithID(7)}, isRepo)

	snapshot := syncer.MirrorCheckOutForVisit(context.Background(), visit)
	require.NotNil(t, snapshot)
	assert.Equal(t, 1, isRepo.checkoutCalls)
	assert.Equal(t, int64(7), isRepo.checkoutID)
}

func TestLoadAttendance_DoesNotCheckoutUnmirroredRow(t *testing.T) {
	exit := time.Date(2026, 4, 20, 15, 0, 0, 0, time.UTC)
	visit := validVisit()
	visit.ExitTime = &exit

	tests := map[string]*scheduleModel.InstanceStudent{
		"expected": expectedRow(7),
		"present without checkin": func() *scheduleModel.InstanceStudent {
			row := expectedRow(8)
			row.Status = scheduleModel.AttendanceStatusPresent
			return row
		}(),
		"checkout before checkin": func() *scheduleModel.InstanceStudent {
			row := expectedRow(9)
			checkedIn := exit.Add(time.Minute)
			row.Status = scheduleModel.AttendanceStatusPresent
			row.CheckedInAt = &checkedIn
			return row
		}(),
	}

	for name, row := range tests {
		t.Run(name, func(t *testing.T) {
			isRepo := &fakeInstanceStudentRepo{findRow: row}
			snapshot := newUnitSyncer(&fakeInstanceRepo{instance: instanceWithID(7)}, isRepo).
				MirrorCheckOutForVisit(context.Background(), visit)

			require.NotNil(t, snapshot)
			assert.Zero(t, isRepo.checkoutCalls)
		})
	}
}

func TestLoadAttendance_CheckoutError(t *testing.T) {
	exit := time.Date(2026, 4, 20, 15, 0, 0, 0, time.UTC)
	visit := validVisit()
	visit.ExitTime = &exit
	row := expectedRow(10)
	checkedIn := exit.Add(-time.Hour)
	row.Status = scheduleModel.AttendanceStatusPresent
	row.CheckedInAt = &checkedIn
	isRepo := &fakeInstanceStudentRepo{findRow: row, checkoutErr: errors.New("update failed")}
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: instanceWithID(7)}, isRepo)

	assert.Nil(t, syncer.MirrorCheckOutForVisit(context.Background(), visit))
	assert.Equal(t, 1, isRepo.checkoutCalls)
}

func TestLoadAttendance_PanicRecovery(t *testing.T) {
	instRepo := &fakeInstanceRepo{findPanic: errors.New("kaboom")}
	syncer := newUnitSyncer(instRepo, &fakeInstanceStudentRepo{})
	require.NotPanics(t, func() {
		snap := syncer.MirrorCheckOutForVisit(context.Background(), validVisit())
		assert.Nil(t, snap)
	})
}

func TestMirrorCheckOutAt_ClosesLatestOpenSlot(t *testing.T) {
	checkedIn := time.Date(2026, 4, 20, 14, 0, 0, 0, time.UTC)
	closed := expectedRow(20)
	closed.Status = scheduleModel.AttendanceStatusPresent
	closed.CheckedInAt = &checkedIn
	closed.CheckedOutAt = &checkedIn
	open := expectedRow(21)
	open.InstanceID = 88
	open.Status = scheduleModel.AttendanceStatusPresent
	open.CheckedInAt = &checkedIn
	isRepo := &fakeInstanceStudentRepo{dateRows: []*scheduleModel.InstanceStudent{closed, open}}

	newUnitSyncer(&fakeInstanceRepo{}, isRepo).MirrorCheckOutAt(context.Background(), open.StudentID, time.Now())

	assert.Equal(t, 1, isRepo.checkoutCalls)
	assert.Equal(t, int64(88), isRepo.checkoutID)
}

func TestMirrorVisitRevision_ReconcilesExactPreviousInterval(t *testing.T) {
	previous := validVisit()
	previousExit := previous.EntryTime.Add(time.Hour)
	previous.ExitTime = &previousExit
	updated := *previous
	updated.EntryTime = previous.EntryTime.Add(10 * time.Minute)
	updated.ExitTime = nil
	isRepo := &fakeInstanceStudentRepo{}
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: instanceWithID(7)}, isRepo)

	syncer.MirrorVisitRevision(context.Background(), previous, &updated)

	require.Equal(t, 1, isRepo.reconcileCalls)
	assert.Equal(t, previous.EntryTime, isRepo.previousIn)
	require.NotNil(t, isRepo.previousOut)
	assert.Equal(t, previousExit, *isRepo.previousOut)
	assert.Equal(t, updated.EntryTime, isRepo.updatedIn)
	assert.Nil(t, isRepo.updatedOut)
}

func TestMirrorVisitRevision_IgnoresIdentityAndGroupChanges(t *testing.T) {
	previous := validVisit()
	tests := []struct {
		name   string
		mutate func(*activeModel.Visit)
	}{
		{name: "student", mutate: func(v *activeModel.Visit) { v.StudentID++ }},
		{name: "group", mutate: func(v *activeModel.Visit) { v.ActiveGroupID++ }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := *previous
			tt.mutate(&updated)
			isRepo := &fakeInstanceStudentRepo{}
			newUnitSyncer(&fakeInstanceRepo{instance: instanceWithID(7)}, isRepo).
				MirrorVisitRevision(context.Background(), previous, &updated)
			assert.Zero(t, isRepo.reconcileCalls)
		})
	}
}

func TestMirrorCheckOutAt_HandlesFailures(t *testing.T) {
	t.Run("lookup error", func(t *testing.T) {
		isRepo := &fakeInstanceStudentRepo{dateErr: errors.New("lookup failed")}
		newUnitSyncer(&fakeInstanceRepo{}, isRepo).MirrorCheckOutAt(context.Background(), 100, time.Now())
		assert.Zero(t, isRepo.checkoutCalls)
	})

	t.Run("update error", func(t *testing.T) {
		checkedIn := time.Now().Add(-time.Hour)
		row := expectedRow(22)
		row.Status = scheduleModel.AttendanceStatusPresent
		row.CheckedInAt = &checkedIn
		isRepo := &fakeInstanceStudentRepo{dateRows: []*scheduleModel.InstanceStudent{row}, checkoutErr: errors.New("update failed")}
		newUnitSyncer(&fakeInstanceRepo{}, isRepo).MirrorCheckOutAt(context.Background(), row.StudentID, time.Now())
		assert.Equal(t, 1, isRepo.checkoutCalls)
	})

	t.Run("panic", func(t *testing.T) {
		isRepo := &fakeInstanceStudentRepo{datePanic: "boom"}
		require.NotPanics(t, func() {
			newUnitSyncer(&fakeInstanceRepo{}, isRepo).MirrorCheckOutAt(context.Background(), 100, time.Now())
		})
	})
}

// -----------------------------------------------------------------------------
// Logger fallback
// -----------------------------------------------------------------------------

func TestAttendanceSync_NilLoggerUsesDefault(t *testing.T) {
	// When a nil logger is passed, the service must fall back to slog.Default()
	// and still execute end-to-end without panicking.
	svc := scheduleSvc.NewAttendanceSyncService(
		&fakeInstanceRepo{instance: nil},
		&fakeInstanceStudentRepo{},
		nil,
	)
	require.NotPanics(t, func() {
		assert.Nil(t, svc.MirrorCheckInForVisit(context.Background(), validVisit()))
		assert.Nil(t, svc.MirrorCheckOutForVisit(context.Background(), validVisit()))
	})
}

// Stubs for the issue #585 cleanup refactor interface additions — unused by
// the attendance-sync tests.
func (f *fakeInstanceRepo) CompleteActiveByActiveGroupIDs(context.Context, []int64, time.Time) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) CountWithOptions(context.Context, *modelsBase.QueryOptions) (int, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) OldestBefore(context.Context, string, *timezone.Date) (*timezone.Date, error) {
	return nil, nil
}

func (f *fakeInstanceRepo) DeleteOlderThan(context.Context, string, timezone.Date) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceStudentRepo) MarkExpectedAbsentByActiveGroupIDs(context.Context, []int64, time.Time, []scheduleModel.StudentInstanceRef) error {
	return nil
}

func (f *fakeInstanceStudentRepo) CloseOpenCheckoutsByActiveGroupIDs(context.Context, []int64, time.Time) (int, error) {
	return 0, nil
}

func (f *fakeInstanceStudentRepo) ListStudentInstanceRefsBefore(context.Context, timezone.Date) ([]scheduleModel.StudentInstanceRef, error) {
	return nil, nil
}

func (f *fakeInstanceRepo) DeletePlannedNonSpontaneousInWindow(context.Context, timezone.Date, *timezone.Date, *int64, bool) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) PropagateListKindToFutureInstances(context.Context, int64, *string, *string, timezone.Date) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) UpdateColumns(context.Context, *scheduleModel.ActivityInstance, ...string) (int64, error) {
	return 0, nil
}

func (f *fakeInstanceRepo) FindByIDs(context.Context, []int64) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}

func (f *fakeInstanceRepo) FindPlannedTemplateBackedFrom(context.Context, timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, nil
}

func (f *fakeInstanceRepo) MaxID(context.Context) (int64, error) {
	return 0, nil
}
