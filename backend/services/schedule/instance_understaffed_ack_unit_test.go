package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ackUnitStaffRepo stubs InstanceStaffRepository.FindByInstanceID for the
// understaffed-acknowledgement clearing tests.
type ackUnitStaffRepo struct {
	scheduleModel.InstanceStaffRepository
	rows []*scheduleModel.InstanceStaff
	err  error
}

func (r *ackUnitStaffRepo) FindByInstanceID(_ context.Context, _ int64) ([]*scheduleModel.InstanceStaff, error) {
	return r.rows, r.err
}

// UpdateColumns records the partial-update call so tests can assert the ack was
// (or was not) persisted. Lives here because clearStaleAckIfStaffed is the only
// user in this test group.
func (r *deleteUnitInstanceRepo) UpdateColumns(_ context.Context, _ *scheduleModel.ActivityInstance, columns ...string) (int64, error) {
	r.updatedColumns = append(r.updatedColumns, columns)
	return 1, r.updateColumnsErr
}

func ackUnitInstance(ack bool) *scheduleModel.ActivityInstance {
	inst := deleteUnitInstance(700, nil, timezone.NewDate(2026, 7, 5), scheduleModel.InstanceStatusPlanned, false)
	inst.UnderstaffedAck = ack
	note := "keine Vertretung"
	inst.UnderstaffedNote = &note
	return inst
}

func TestClearUnderstaffedAckIfStaffed_NoAckIsNoop(t *testing.T) {
	t.Parallel()

	inst := ackUnitInstance(false)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst}
	staffRepo := &ackUnitStaffRepo{}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:       instanceRepo,
		InstanceStaffRepo:  staffRepo,
		DeviationEventRepo: &recordingDeviationEventRepo{},
		Logger:             slog.New(slog.DiscardHandler),
	}}

	require.NoError(t, svc.ClearUnderstaffedAckIfStaffed(context.Background(), inst.ID, nil))
	assert.Empty(t, instanceRepo.updatedColumns, "must not touch the ack when it was never set")
}

func TestClearUnderstaffedAckIfStaffed_StillShortStaffedKeepsAck(t *testing.T) {
	t.Parallel()

	inst := ackUnitInstance(true)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst}
	// Two planned positions, one absent without a substitute → still understaffed.
	staffRepo := &ackUnitStaffRepo{rows: []*scheduleModel.InstanceStaff{
		{StaffID: 11, IsAbsent: false, IsSubstitute: false},
		{StaffID: 12, IsAbsent: true, IsSubstitute: false},
	}}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:       instanceRepo,
		InstanceStaffRepo:  staffRepo,
		DeviationEventRepo: &recordingDeviationEventRepo{},
		Logger:             slog.New(slog.DiscardHandler),
	}}

	require.NoError(t, svc.ClearUnderstaffedAckIfStaffed(context.Background(), inst.ID, nil))
	assert.Empty(t, instanceRepo.updatedColumns, "a partially covered block keeps its acknowledgement")
	assert.True(t, inst.UnderstaffedAck)
}

func TestClearUnderstaffedAckIfStaffed_FullyStaffedClearsAck(t *testing.T) {
	t.Parallel()

	inst := ackUnitInstance(true)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst}
	events := &recordingDeviationEventRepo{}
	// Planned person absent but a substitute covers → fully staffed again.
	staffRepo := &ackUnitStaffRepo{rows: []*scheduleModel.InstanceStaff{
		{StaffID: 11, IsAbsent: true, IsSubstitute: false},
		{StaffID: 12, IsAbsent: false, IsSubstitute: true},
	}}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:       instanceRepo,
		InstanceStaffRepo:  staffRepo,
		DeviationEventRepo: events,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	require.NoError(t, svc.ClearUnderstaffedAckIfStaffed(context.Background(), inst.ID, nil))

	require.Len(t, instanceRepo.updatedColumns, 1)
	assert.ElementsMatch(t, []string{"understaffed_ack", "understaffed_note"}, instanceRepo.updatedColumns[0])
	assert.False(t, inst.UnderstaffedAck)
	assert.Nil(t, inst.UnderstaffedNote)
	require.Len(t, events.events, 1)
	assert.Equal(t, "understaffed_unack", events.events[0].EventType)
	assert.JSONEq(t, `{"understaffed_ack":true,"note":"keine Vertretung"}`, string(events.events[0].OldValue))
	assert.JSONEq(t, `{"understaffed_ack":false}`, string(events.events[0].NewValue))
}

func TestClearUnderstaffedAckIfStaffed_LoadStaffError(t *testing.T) {
	t.Parallel()

	inst := ackUnitInstance(true)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst}
	staffRepo := &ackUnitStaffRepo{err: errors.New("staff load failed")}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:       instanceRepo,
		InstanceStaffRepo:  staffRepo,
		DeviationEventRepo: &recordingDeviationEventRepo{},
		Logger:             slog.New(slog.DiscardHandler),
	}}

	err := svc.ClearUnderstaffedAckIfStaffed(context.Background(), inst.ID, nil)
	var scheduleErr *ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Equal(t, "clear stale ack: load staff", scheduleErr.Op)
}

func TestClearUnderstaffedAckIfStaffed_LoadInstanceError(t *testing.T) {
	t.Parallel()

	instanceRepo := &deleteUnitInstanceRepo{findErr: errors.New("instance load failed")}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:      instanceRepo,
		InstanceStaffRepo: &ackUnitStaffRepo{},
		Logger:            slog.New(slog.DiscardHandler),
	}}

	err := svc.ClearUnderstaffedAckIfStaffed(context.Background(), int64(700), nil)
	var scheduleErr *ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Equal(t, "load instance", scheduleErr.Op)
}

func TestClearUnderstaffedAckIfStaffed_UpdatePersistError(t *testing.T) {
	t.Parallel()

	inst := ackUnitInstance(true)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst, updateColumnsErr: errors.New("update failed")}
	staffRepo := &ackUnitStaffRepo{rows: []*scheduleModel.InstanceStaff{
		{StaffID: 11, IsAbsent: false, IsSubstitute: false},
	}}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:       instanceRepo,
		InstanceStaffRepo:  staffRepo,
		DeviationEventRepo: &recordingDeviationEventRepo{},
		Logger:             slog.New(slog.DiscardHandler),
	}}

	err := svc.ClearUnderstaffedAckIfStaffed(context.Background(), inst.ID, nil)
	var scheduleErr *ScheduleError
	require.ErrorAs(t, err, &scheduleErr)
	assert.Equal(t, "clear stale ack: update", scheduleErr.Op)
}

func TestIsUnderstaffed_SkipsNilRows(t *testing.T) {
	t.Parallel()

	// A nil row must be ignored; one present, non-substitute planned person
	// leaves the block fully staffed.
	rows := []*scheduleModel.InstanceStaff{
		nil,
		{StaffID: 11, IsAbsent: false, IsSubstitute: false},
	}
	assert.False(t, IsUnderstaffed(rows))
	assert.True(t, IsUnderstaffed([]*scheduleModel.InstanceStaff{nil}))
}
