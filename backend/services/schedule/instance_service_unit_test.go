package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lifecycleSettingsStub struct {
	intVal  int
	intErr  error
	boolVal bool
	boolErr error
}

func (s lifecycleSettingsStub) ResolveInt(context.Context, string) (int, error) {
	return s.intVal, s.intErr
}

func (s lifecycleSettingsStub) ResolveBool(context.Context, string) (bool, error) {
	return s.boolVal, s.boolErr
}

func plannedLifecycleInstance() *scheduleModel.ActivityInstance {
	return &scheduleModel.ActivityInstance{
		Date:      timezone.NewDate(2026, 8, 13),
		StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
	}
}

func TestValidateStartTime(t *testing.T) {
	inst := plannedLifecycleInstance()
	tooEarly := time.Date(2026, 8, 13, 13, 0, 0, 0, timezone.Berlin)
	inWindow := time.Date(2026, 8, 13, 13, 50, 0, 0, timezone.Berlin)
	afterEnd := time.Date(2026, 8, 13, 15, 0, 0, 0, timezone.Berlin)

	require.NoError(t, (&instanceService{}).validateStartTime(context.Background(), inst, tooEarly))

	svc := &instanceService{deps: InstanceServiceDependencies{
		EnforceTimePolicy: true,
		Settings:          lifecycleSettingsStub{intVal: 15},
	}}
	require.ErrorIs(t, svc.validateStartTime(context.Background(), inst, tooEarly), ErrInstanceStartTooEarly)
	require.NoError(t, svc.validateStartTime(context.Background(), inst, inWindow))
	require.ErrorIs(t, svc.validateStartTime(context.Background(), inst, afterEnd), ErrInstanceStartExpired)

	spontaneous := *inst
	spontaneous.IsSpontaneous = true
	require.NoError(t, svc.validateStartTime(context.Background(), &spontaneous, tooEarly))

	svc.deps.Settings = lifecycleSettingsStub{intErr: errors.New("settings unavailable")}
	require.ErrorIs(t, svc.validateStartTime(context.Background(), inst, inWindow), ErrLifecycleSettings)
}

func TestValidateCompleteTime(t *testing.T) {
	inst := plannedLifecycleInstance()
	beforeEnd := time.Date(2026, 8, 13, 14, 30, 0, 0, timezone.Berlin)
	atEnd := time.Date(2026, 8, 13, 15, 0, 0, 0, timezone.Berlin)

	require.NoError(t, (&instanceService{}).validateCompleteTime(context.Background(), inst, beforeEnd))

	svc := &instanceService{deps: InstanceServiceDependencies{
		EnforceTimePolicy: true,
		Settings:          lifecycleSettingsStub{boolVal: true},
	}}
	require.ErrorIs(t, svc.validateCompleteTime(context.Background(), inst, beforeEnd), ErrInstanceCompleteEarly)
	require.NoError(t, svc.validateCompleteTime(context.Background(), inst, atEnd))

	svc.deps.Settings = lifecycleSettingsStub{boolVal: false}
	require.NoError(t, svc.validateCompleteTime(context.Background(), inst, beforeEnd))

	spontaneous := *inst
	spontaneous.IsSpontaneous = true
	svc.deps.Settings = lifecycleSettingsStub{boolVal: true}
	require.NoError(t, svc.validateCompleteTime(context.Background(), &spontaneous, beforeEnd))

	svc.deps.Settings = lifecycleSettingsStub{boolErr: errors.New("settings unavailable")}
	require.ErrorIs(t, svc.validateCompleteTime(context.Background(), inst, atEnd), ErrLifecycleSettings)
}

func TestInstanceNowUsesWallClockWhenUnset(t *testing.T) {
	got := (&instanceService{}).now()
	assert.False(t, got.IsZero())
}

func TestWithCompletionConfirmationStoresClone(t *testing.T) {
	ids := []int64{42, 41}
	ctx := WithCompletionConfirmation(context.Background(), ids)
	ids[0] = 40
	got, ok := ctx.Value(lifecycleConfirmedStudentsKey).([]int64)
	require.True(t, ok)
	assert.Equal(t, []int64{42, 41}, got)
}

func TestCanReopenInstance(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	until := now.Add(5 * time.Minute)
	completedBy := int64(42)
	snapshot := []byte(`{"active_group_id":1}`)

	base := &scheduleModel.ActivityInstance{
		Status:             scheduleModel.InstanceStatusCompleted,
		CompletedBy:        &completedBy,
		ReopenUntil:        &until,
		CompletionSnapshot: snapshot,
	}

	assert.True(t, CanReopenInstance(base, 42, false, now))
	assert.True(t, CanReopenInstance(base, 41, true, now))
	assert.False(t, CanReopenInstance(base, 40, false, now))
	assert.False(t, CanReopenInstance(base, 42, false, until.Add(time.Second)))
	assert.True(t, CanReopenAsActor(base, 42, false))
	assert.False(t, CanReopenAsActor(base, 40, false))
	completedAt := now
	base.CompletedAt = &completedAt
	changed := &scheduleModel.InstanceStudent{}
	changed.UpdatedAt = now.Add(time.Minute)
	assert.False(t, AttendanceUnchangedSinceCompletion(base, []*scheduleModel.InstanceStudent{changed}))
	unchanged := &scheduleModel.InstanceStudent{}
	unchanged.UpdatedAt = now.Add(-time.Minute)
	assert.True(t, AttendanceUnchangedSinceCompletion(base, []*scheduleModel.InstanceStudent{unchanged}))
	assert.False(t, CanReopenInstance(&scheduleModel.ActivityInstance{
		Status:             scheduleModel.InstanceStatusCompleted,
		CompletedBy:        &completedBy,
		ReopenUntil:        &until,
		CompletionSnapshot: nil,
	}, 42, true, now))
	assert.False(t, CanReopenInstance(&scheduleModel.ActivityInstance{
		Status:      scheduleModel.InstanceStatusActive,
		CompletedBy: &completedBy,
		ReopenUntil: &until,
	}, 42, true, now))
}

type studentReadScopeStub struct {
	usersModel.StudentRepository
	byID map[int64]*usersModel.Student
}

func (s studentReadScopeStub) FindReadScopeByIDs(_ context.Context, ids []int64) (map[int64]*usersModel.Student, error) {
	out := make(map[int64]*usersModel.Student, len(ids))
	for _, id := range ids {
		if student, ok := s.byID[id]; ok {
			out[id] = student
		}
	}
	return out, nil
}

func TestBroadcastRestoredVisits_EmitsBulkCheckInAndDashboard(t *testing.T) {
	broadcaster := testpkg.NewRecordingBroadcaster()
	eduGroupID := int64(70)
	student := &usersModel.Student{GroupID: &eduGroupID}
	student.ID = 42
	svc := &instanceService{deps: InstanceServiceDependencies{
		Broadcaster: broadcaster,
		StudentRepo: studentReadScopeStub{byID: map[int64]*usersModel.Student{42: student}},
	}}

	svc.broadcastRestoredVisits(tenant.WithTenantID(context.Background(), 1), 99, []int64{42})

	groupEvents := broadcaster.EventsOfType(realtime.EventBulkStudentCheckIn)
	require.Len(t, groupEvents, 2)
	assert.Equal(t, "99", groupEvents[0].ActiveGroupID)
	require.NotNil(t, groupEvents[0].Data.StudentIDs)
	assert.Equal(t, []string{"42"}, *groupEvents[0].Data.StudentIDs)
	require.NotNil(t, groupEvents[0].Data.GroupIDs)
	assert.Equal(t, []string{"70"}, *groupEvents[0].Data.GroupIDs)

	eduCalls := broadcaster.GroupCallsForTopic("edu:70")
	require.Len(t, eduCalls, 1)
	assert.Equal(t, realtime.EventBulkStudentCheckIn, eduCalls[0].Event.Type)
	require.NotNil(t, eduCalls[0].Event.Data.StudentIDs)
	assert.Equal(t, []string{"42"}, *eduCalls[0].Event.Data.StudentIDs)

	dash := broadcaster.EventsOfType(realtime.EventDashboardCountsChanged)
	require.Len(t, dash, 1)
	require.NotNil(t, dash[0].Data.GroupIDs)
	assert.Equal(t, []string{"70"}, *dash[0].Data.GroupIDs)
	assert.Empty(t, broadcaster.EventsOfType(realtime.EventStudentCheckIn))
}

func TestBroadcastRestoredVisits_SkipsEmptyRestore(t *testing.T) {
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := &instanceService{deps: InstanceServiceDependencies{Broadcaster: broadcaster}}
	svc.broadcastRestoredVisits(tenant.WithTenantID(context.Background(), 1), 99, nil)
	assert.Empty(t, broadcaster.Calls())
}

func TestValidateLegacyWeekendInstanceDate(t *testing.T) {
	saturday := timezone.NewDate(2026, time.May, 9)
	monday := saturday.AddDays(2)

	assert.NoError(t, validateLegacyWeekendInstanceDate(monday, monday), "weekday updates stay valid")
	assert.NoError(t, validateLegacyWeekendInstanceDate(saturday, saturday), "legacy weekend rows may retain their original date")
	assert.ErrorIs(t, validateLegacyWeekendInstanceDate(monday, saturday), ErrInstanceWeekend,
		"new weekend dates must be rejected")
}

func TestInstanceDelete_PlannedTemplateBackedCreatesCancellationException(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 7301)
	groupID := int64(410)
	date := timezone.NewDate(2026, 7, 6)
	inst := deleteUnitInstance(101, &groupID, date, scheduleModel.InstanceStatusPlanned, false)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst, sameDayRows: []*scheduleModel.ActivityInstance{inst}}
	exceptionRepo := &deleteUnitExceptionRepo{}
	svc := deleteUnitService(instanceRepo, exceptionRepo)

	err := svc.DeleteCancelled(ctx, inst.ID)

	require.NoError(t, err)
	assert.Equal(t, []int64{inst.ID}, instanceRepo.deleted)
	require.Len(t, exceptionRepo.created, 1)
	created := exceptionRepo.created[0]
	assert.Equal(t, groupID, created.ActivityGroupID)
	assert.Equal(t, date, created.ExceptionDate)
	assert.Equal(t, scheduleModel.ActivityExceptionCancelled, created.ExceptionType)
	require.NotNil(t, created.Reason)
	assert.Equal(t, deletedSlotReason, *created.Reason)
	assert.Equal(t, int64(7301), created.TenantID)
}

func TestInstanceDelete_AmbiguousTemplateBackedDateDoesNotDelete(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 7302)
	groupID := int64(411)
	date := timezone.NewDate(2026, 7, 7)
	inst := deleteUnitInstance(102, &groupID, date, scheduleModel.InstanceStatusPlanned, false)
	otherSlot := deleteUnitInstance(103, &groupID, date, scheduleModel.InstanceStatusPlanned, false)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst, sameDayRows: []*scheduleModel.ActivityInstance{inst, nil, otherSlot}}
	exceptionRepo := &deleteUnitExceptionRepo{}
	svc := deleteUnitService(instanceRepo, exceptionRepo)

	err := svc.DeleteCancelled(ctx, inst.ID)

	require.ErrorIs(t, err, ErrAmbiguousTemplateInstanceDelete)
	assert.Empty(t, instanceRepo.deleted)
	assert.Empty(t, exceptionRepo.created)
	assert.Empty(t, exceptionRepo.updated)
}

func TestInstanceDelete_SpontaneousTemplateLinkedSkipsException(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 7303)
	groupID := int64(412)
	inst := deleteUnitInstance(104, &groupID, timezone.NewDate(2026, 7, 8), scheduleModel.InstanceStatusPlanned, true)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst}
	exceptionRepo := &deleteUnitExceptionRepo{}
	svc := deleteUnitService(instanceRepo, exceptionRepo)

	err := svc.DeleteCancelled(ctx, inst.ID)

	require.NoError(t, err)
	assert.Equal(t, []int64{inst.ID}, instanceRepo.deleted)
	assert.Zero(t, instanceRepo.sameDayCalls)
	assert.Zero(t, exceptionRepo.findCalls)
	assert.Empty(t, exceptionRepo.created)
}

func TestInstanceDelete_ExistingModifiedExceptionIsConvertedToCancellation(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 7304)
	groupID := int64(413)
	date := timezone.NewDate(2026, 7, 9)
	inst := deleteUnitInstance(105, &groupID, date, scheduleModel.InstanceStatusCancelled, false)
	start := time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC)
	end := time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC)
	roomID := int64(900)
	reason := "Raumwechsel"
	existing := &scheduleModel.ActivityException{
		ActivityGroupID: groupID,
		ExceptionDate:   date,
		ExceptionType:   scheduleModel.ActivityExceptionModified,
		StartTime:       &start,
		EndTime:         &end,
		RoomID:          &roomID,
		Reason:          &reason,
	}
	instanceRepo := &deleteUnitInstanceRepo{instance: inst, sameDayRows: []*scheduleModel.ActivityInstance{inst}}
	exceptionRepo := &deleteUnitExceptionRepo{existing: existing}
	svc := deleteUnitService(instanceRepo, exceptionRepo)

	err := svc.DeleteCancelled(ctx, inst.ID)

	require.NoError(t, err)
	assert.Equal(t, []int64{inst.ID}, instanceRepo.deleted)
	assert.Empty(t, exceptionRepo.created)
	require.Len(t, exceptionRepo.updated, 1)
	updated := exceptionRepo.updated[0]
	assert.Equal(t, scheduleModel.ActivityExceptionCancelled, updated.ExceptionType)
	assert.Nil(t, updated.StartTime)
	assert.Nil(t, updated.EndTime)
	assert.Nil(t, updated.RoomID)
	require.NotNil(t, updated.Reason)
	assert.Equal(t, deletedSlotReason, *updated.Reason)
}

func TestInstanceDelete_ErrorBranches(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 7305)
	groupID := int64(414)
	date := timezone.NewDate(2026, 7, 10)
	inst := deleteUnitInstance(106, &groupID, date, scheduleModel.InstanceStatusPlanned, false)

	tests := []struct {
		name          string
		instanceRepo  *deleteUnitInstanceRepo
		exceptionRepo *deleteUnitExceptionRepo
		wantOp        string
	}{
		{
			name:          "same-day lookup error",
			instanceRepo:  &deleteUnitInstanceRepo{instance: inst, sameDayErr: errors.New("same-day failed")},
			exceptionRepo: &deleteUnitExceptionRepo{},
			wantOp:        "delete instance: check same-day template slots",
		},
		{
			name:          "exception lookup error",
			instanceRepo:  &deleteUnitInstanceRepo{instance: inst, sameDayRows: []*scheduleModel.ActivityInstance{inst}},
			exceptionRepo: &deleteUnitExceptionRepo{findErr: errors.New("exception lookup failed")},
			wantOp:        "delete instance: check slot exception",
		},
		{
			name:          "exception create error",
			instanceRepo:  &deleteUnitInstanceRepo{instance: inst, sameDayRows: []*scheduleModel.ActivityInstance{inst}},
			exceptionRepo: &deleteUnitExceptionRepo{createErr: errors.New("insert failed")},
			wantOp:        "delete instance: create cancellation exception",
		},
		{
			name:          "delete error",
			instanceRepo:  &deleteUnitInstanceRepo{instance: inst, sameDayRows: []*scheduleModel.ActivityInstance{inst}, deleteErr: errors.New("delete failed")},
			exceptionRepo: &deleteUnitExceptionRepo{},
			wantOp:        "delete instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := deleteUnitService(tt.instanceRepo, tt.exceptionRepo)

			err := svc.DeleteCancelled(ctx, inst.ID)

			require.Error(t, err)
			var scheduleErr *ScheduleError
			require.ErrorAs(t, err, &scheduleErr)
			assert.Equal(t, tt.wantOp, scheduleErr.Op)
		})
	}
}

func TestInstanceDelete_RejectsProtectedStatuses(t *testing.T) {
	ctx := tenant.WithTenantID(context.Background(), 7306)
	for _, status := range []string{scheduleModel.InstanceStatusActive, scheduleModel.InstanceStatusCompleted} {
		t.Run(status, func(t *testing.T) {
			inst := deleteUnitInstance(107, nil, timezone.NewDate(2026, 7, 11), status, false)
			instanceRepo := &deleteUnitInstanceRepo{instance: inst}
			svc := deleteUnitService(instanceRepo, &deleteUnitExceptionRepo{})

			err := svc.DeleteCancelled(ctx, inst.ID)

			require.ErrorIs(t, err, ErrInvalidInstanceTransition)
			assert.Empty(t, instanceRepo.deleted)
		})
	}
}

func deleteUnitService(instanceRepo *deleteUnitInstanceRepo, exceptionRepo *deleteUnitExceptionRepo) *instanceService {
	return &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo:  instanceRepo,
		ExceptionRepo: exceptionRepo,
		Logger:        slog.New(slog.DiscardHandler),
	}}
}

func deleteUnitInstance(id int64, groupID *int64, date timezone.Date, status string, spontaneous bool) *scheduleModel.ActivityInstance {
	inst := &scheduleModel.ActivityInstance{
		Date:            date,
		ActivityGroupID: groupID,
		Title:           "Delete unit test",
		StartTime:       time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:          501,
		Status:          status,
		IsSpontaneous:   spontaneous,
	}
	inst.ID = id
	return inst
}

type deleteUnitInstanceRepo struct {
	scheduleModel.ActivityInstanceRepository
	instance         *scheduleModel.ActivityInstance
	findErr          error
	sameDayRows      []*scheduleModel.ActivityInstance
	sameDayErr       error
	sameDayCalls     int
	deleteErr        error
	deleted          []int64
	updatedColumns   [][]string
	updateColumnsErr error
}

func (r *deleteUnitInstanceRepo) FindByID(_ context.Context, _ any) (*scheduleModel.ActivityInstance, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.instance, nil
}

func (r *deleteUnitInstanceRepo) FindByActivityGroupAndDate(_ context.Context, _ int64, _ timezone.Date) ([]*scheduleModel.ActivityInstance, error) {
	r.sameDayCalls++
	if r.sameDayErr != nil {
		return nil, r.sameDayErr
	}
	return r.sameDayRows, nil
}

func (r *deleteUnitInstanceRepo) Delete(_ context.Context, id any) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if v, ok := id.(int64); ok {
		r.deleted = append(r.deleted, v)
	}
	return nil
}

type deleteUnitExceptionRepo struct {
	scheduleModel.ActivityExceptionRepository
	existing  *scheduleModel.ActivityException
	findErr   error
	createErr error
	updateErr error
	findCalls int
	created   []*scheduleModel.ActivityException
	updated   []*scheduleModel.ActivityException
}

func (r *deleteUnitExceptionRepo) FindByActivityGroupAndDate(_ context.Context, _ int64, _ timezone.Date) (*scheduleModel.ActivityException, error) {
	r.findCalls++
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.existing, nil
}

func (r *deleteUnitExceptionRepo) Create(_ context.Context, exc *scheduleModel.ActivityException) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, exc)
	return nil
}

func (r *deleteUnitExceptionRepo) Update(_ context.Context, exc *scheduleModel.ActivityException) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updated = append(r.updated, exc)
	return nil
}
