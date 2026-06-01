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

func (f *fakeInstanceRepo) FindByTenantAndDate(context.Context, time.Time) ([]*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) FindByTenantAndDateRange(context.Context, time.Time, time.Time) ([]*scheduleModel.ActivityInstance, error) {
	panic("unused")
}

func (f *fakeInstanceRepo) FindByActivityGroupAndDate(context.Context, int64, time.Time) ([]*scheduleModel.ActivityInstance, error) {
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
	findRow      *scheduleModel.InstanceStudent
	findErr      error
	updateResult bool
	updateErr    error
	updateCalls  int
}

func (f *fakeInstanceStudentRepo) FindByInstanceAndStudent(_ context.Context, _, _ int64) (*scheduleModel.InstanceStudent, error) {
	return f.findRow, f.findErr
}

func (f *fakeInstanceStudentRepo) UpdateAttendanceFromCheckin(_ context.Context, _, _ int64, _ time.Time) (bool, error) {
	f.updateCalls++
	return f.updateResult, f.updateErr
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

func (f *fakeInstanceStudentRepo) FindExpectedByInstanceIDs(context.Context, []int64) ([]*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindByStudentAndDateRange(context.Context, int64, time.Time, time.Time) ([]*scheduleModel.InstanceStudent, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindPlannedStudentIDsByDate(context.Context, []int64, time.Time) ([]int64, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) DeleteByInstanceID(context.Context, int64) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) UpdateAttendanceFields(context.Context, int64, scheduleModel.AttendanceFieldPatch) error {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) BulkUpdateStatus(context.Context, int64, string, string) (int, error) {
	panic("unused")
}

func (f *fakeInstanceStudentRepo) FindInstancesWithAttendanceByStudentAndDateRange(context.Context, int64, time.Time, time.Time) ([]*scheduleModel.ScheduledInstanceRow, error) {
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
	isRepo := &fakeInstanceStudentRepo{findRow: nil}
	syncer := newUnitSyncer(instRepo, isRepo)

	assert.Nil(t, syncer.MirrorCheckInForVisit(context.Background(), validVisit()))
	assert.Equal(t, 0, isRepo.updateCalls, "no row → skip UPDATE")
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
// LoadAttendanceForVisit — error branches
// -----------------------------------------------------------------------------

func TestLoadAttendance_NilVisit(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.LoadAttendanceForVisit(context.Background(), nil))
}

func TestLoadAttendance_ZeroActiveGroup(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{}, &fakeInstanceStudentRepo{})
	v := validVisit()
	v.ActiveGroupID = 0
	assert.Nil(t, syncer.LoadAttendanceForVisit(context.Background(), v))
}

func TestLoadAttendance_InstanceLookupError(t *testing.T) {
	instRepo := &fakeInstanceRepo{findErr: errors.New("db down")}
	syncer := newUnitSyncer(instRepo, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.LoadAttendanceForVisit(context.Background(), validVisit()))
}

func TestLoadAttendance_NoInstance(t *testing.T) {
	syncer := newUnitSyncer(&fakeInstanceRepo{instance: nil}, &fakeInstanceStudentRepo{})
	assert.Nil(t, syncer.LoadAttendanceForVisit(context.Background(), validVisit()))
}

func TestLoadAttendance_InstanceStudentLookupError(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findErr: errors.New("scan failed")}
	syncer := newUnitSyncer(instRepo, isRepo)
	assert.Nil(t, syncer.LoadAttendanceForVisit(context.Background(), validVisit()))
}

func TestLoadAttendance_NoRow(t *testing.T) {
	instRepo := &fakeInstanceRepo{instance: instanceWithID(7)}
	isRepo := &fakeInstanceStudentRepo{findRow: nil}
	syncer := newUnitSyncer(instRepo, isRepo)
	assert.Nil(t, syncer.LoadAttendanceForVisit(context.Background(), validVisit()))
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

	snap := syncer.LoadAttendanceForVisit(context.Background(), validVisit())
	require.NotNil(t, snap)
	assert.Equal(t, scheduleModel.AttendanceStatusPresent, snap.Status)
	require.NotNil(t, snap.Substatus)
	assert.Equal(t, scheduleModel.AttendanceSubstatusLate, *snap.Substatus)
	require.NotNil(t, snap.Note)
	assert.Equal(t, "bus delay", *snap.Note)
}

func TestLoadAttendance_PanicRecovery(t *testing.T) {
	instRepo := &fakeInstanceRepo{findPanic: errors.New("kaboom")}
	syncer := newUnitSyncer(instRepo, &fakeInstanceStudentRepo{})
	require.NotPanics(t, func() {
		snap := syncer.LoadAttendanceForVisit(context.Background(), validVisit())
		assert.Nil(t, snap)
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
		assert.Nil(t, svc.LoadAttendanceForVisit(context.Background(), validVisit()))
	})
}
