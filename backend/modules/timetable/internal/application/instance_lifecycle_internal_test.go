package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func plannedLifecycleInstance() *scheduleModel.ActivityInstance {
	return &scheduleModel.ActivityInstance{
		Date:      calendar.NewDate(2026, 8, 13),
		StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
	}
}

func TestValidateStartTime(t *testing.T) {
	t.Parallel()

	inst := plannedLifecycleInstance()
	tooEarly := time.Date(2026, 8, 13, 13, 0, 0, 0, calendar.Berlin)
	inWindow := time.Date(2026, 8, 13, 13, 50, 0, 0, calendar.Berlin)
	afterEnd := time.Date(2026, 8, 13, 15, 0, 0, 0, calendar.Berlin)

	require.NoError(t, (&InstanceLifecycleService{}).validateStartTime(context.Background(), inst, tooEarly))

	svc := &InstanceLifecycleService{deps: ports.InstanceLifecycleDependencies{
		EnforceTimePolicy: true,
		Settings:          lifecycleSettingsStub{intVal: 15},
	}}
	require.ErrorIs(t, svc.validateStartTime(context.Background(), inst, tooEarly), timetable.ErrInstanceStartTooEarly)
	require.NoError(t, svc.validateStartTime(context.Background(), inst, inWindow))
	require.ErrorIs(t, svc.validateStartTime(context.Background(), inst, afterEnd), timetable.ErrInstanceStartExpired)

	spontaneous := *inst
	spontaneous.IsSpontaneous = true
	require.NoError(t, svc.validateStartTime(context.Background(), &spontaneous, tooEarly))

	svc.deps.Settings = lifecycleSettingsStub{intErr: errors.New("settings unavailable")}
	require.ErrorIs(t, svc.validateStartTime(context.Background(), inst, inWindow), timetable.ErrLifecycleSettings)
}

func TestValidateSpontaneousStartWorkday(t *testing.T) {
	t.Parallel()

	spontaneous := plannedLifecycleInstance()
	spontaneous.IsSpontaneous = true
	saturday := time.Date(2026, 8, 15, 0, 0, 0, 0, calendar.Berlin)
	assert.ErrorIs(t, validateSpontaneousStartWorkday(context.Background(), spontaneous, saturday), timetable.ErrInstanceWeekend)

	monday := time.Date(2026, 8, 17, 0, 0, 0, 0, calendar.Berlin)
	assert.NoError(t, validateSpontaneousStartWorkday(context.Background(), spontaneous, monday))

	planned := *spontaneous
	planned.IsSpontaneous = false
	assert.NoError(t, validateSpontaneousStartWorkday(context.Background(), &planned, saturday))
}

func TestValidateCompleteTime(t *testing.T) {
	t.Parallel()

	inst := plannedLifecycleInstance()
	beforeEnd := time.Date(2026, 8, 13, 14, 30, 0, 0, calendar.Berlin)
	atEnd := time.Date(2026, 8, 13, 15, 0, 0, 0, calendar.Berlin)

	require.NoError(t, (&InstanceLifecycleService{}).validateCompleteTime(context.Background(), inst, beforeEnd))

	svc := &InstanceLifecycleService{deps: ports.InstanceLifecycleDependencies{
		EnforceTimePolicy: true,
		Settings:          lifecycleSettingsStub{boolVal: true},
	}}
	require.ErrorIs(t, svc.validateCompleteTime(context.Background(), inst, beforeEnd), timetable.ErrInstanceCompleteEarly)
	require.NoError(t, svc.validateCompleteTime(context.Background(), inst, atEnd))

	svc.deps.Settings = lifecycleSettingsStub{boolVal: false}
	require.NoError(t, svc.validateCompleteTime(context.Background(), inst, beforeEnd))

	spontaneous := *inst
	spontaneous.IsSpontaneous = true
	svc.deps.Settings = lifecycleSettingsStub{boolVal: true}
	require.NoError(t, svc.validateCompleteTime(context.Background(), &spontaneous, beforeEnd))

	svc.deps.Settings = lifecycleSettingsStub{boolErr: errors.New("settings unavailable")}
	require.ErrorIs(t, svc.validateCompleteTime(context.Background(), inst, atEnd), timetable.ErrLifecycleSettings)
}

// The tenant's lead lets a planned block complete before its planned end
// (#3809); the write path refuses one second earlier and surfaces a failed
// lookup instead of assuming no lead.
func TestValidateCompleteTimeHonorsCompleteLead(t *testing.T) {
	t.Parallel()

	inst := plannedLifecycleInstance()
	leadBoundary := time.Date(2026, 8, 13, 14, 45, 0, 0, calendar.Berlin)
	svc := &InstanceLifecycleService{deps: ports.InstanceLifecycleDependencies{
		EnforceTimePolicy: true,
		Settings:          lifecycleSettingsStub{boolVal: true, completeLead: 15},
	}}

	err := svc.validateCompleteTime(context.Background(), inst, leadBoundary.Add(-time.Second))
	require.ErrorIs(t, err, timetable.ErrInstanceCompleteEarly)
	assert.Contains(t, err.Error(), leadBoundary.Format(time.RFC3339))
	require.NoError(t, svc.validateCompleteTime(context.Background(), inst, leadBoundary))

	svc.deps.Settings = lifecycleSettingsStub{boolVal: true, completeLeadErr: errors.New("settings unavailable")}
	require.ErrorIs(t, svc.validateCompleteTime(context.Background(), inst, leadBoundary), timetable.ErrLifecycleSettings)
}

func TestInstanceNowUsesWallClockWhenUnset(t *testing.T) {
	t.Parallel()

	got := (&InstanceLifecycleService{}).now()
	assert.False(t, got.IsZero())
}

func TestWithCompletionConfirmationStoresClone(t *testing.T) {
	t.Parallel()

	ids := []int64{42, 41}
	ctx := timetable.WithCompletionConfirmation(context.Background(), ids)
	ids[0] = 40
	got, ok := timetable.CompletionConfirmation(ctx)
	require.True(t, ok)
	assert.Equal(t, []int64{42, 41}, got)
}

type studentReadScopeStub struct {
	ports.LifecycleStudents
	byID map[int64]*timetable.LifecycleStudent
}

func (s studentReadScopeStub) FindReadScopeByIDs(_ context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	out := make(map[int64]*timetable.LifecycleStudent, len(ids))
	for _, id := range ids {
		if student, ok := s.byID[id]; ok {
			out[id] = student
		}
	}
	return out, nil
}

func TestBroadcastRestoredVisits_EmitsBulkCheckInAndDashboard(t *testing.T) {
	t.Parallel()

	broadcaster := &lifecycleEventRecorder{}
	eduGroupID := int64(70)
	student := &timetable.LifecycleStudent{GroupID: &eduGroupID}
	student.ID = 42
	svc := &InstanceLifecycleService{deps: ports.InstanceLifecycleDependencies{
		Broadcaster: broadcaster,
		StudentRepo: studentReadScopeStub{byID: map[int64]*timetable.LifecycleStudent{42: student}},
	}}

	svc.broadcastRestoredVisits(testpkg.Ctx(t), 99, []int64{42})

	groupEvents := broadcaster.EventsOfType(LifecycleEventBulkStudentCheckIn)
	require.Len(t, groupEvents, 2)
	assert.Equal(t, "99", groupEvents[0].ActiveGroupID)
	require.NotNil(t, groupEvents[0].StudentIDs)
	assert.Equal(t, []string{"42"}, *groupEvents[0].StudentIDs)
	require.NotNil(t, groupEvents[0].GroupIDs)
	assert.Equal(t, []string{"70"}, *groupEvents[0].GroupIDs)

	eduCalls := broadcaster.GroupCallsForTopic("edu:70")
	require.Len(t, eduCalls, 1)
	assert.Equal(t, LifecycleEventBulkStudentCheckIn, eduCalls[0].Event.Type)
	require.NotNil(t, eduCalls[0].Event.StudentIDs)
	assert.Equal(t, []string{"42"}, *eduCalls[0].Event.StudentIDs)

	dash := broadcaster.EventsOfType(LifecycleEventDashboardCountsChanged)
	require.Len(t, dash, 1)
	require.NotNil(t, dash[0].GroupIDs)
	assert.Equal(t, []string{"70"}, *dash[0].GroupIDs)
	// The retained vocabulary's student_checkin event type.
	assert.Empty(t, broadcaster.EventsOfType("student_checkin"))
}

func TestLockReopenSnapshotStudents_EmptySnapshot(t *testing.T) {
	t.Parallel()

	svc := &InstanceLifecycleService{}
	got, err := svc.lockReopenSnapshotStudents(context.Background(), scheduleModel.ActivityCompletionSnapshot{})
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestBroadcastRestoredVisits_SkipsEmptyRestore(t *testing.T) {
	t.Parallel()

	broadcaster := &lifecycleEventRecorder{}
	svc := &InstanceLifecycleService{deps: ports.InstanceLifecycleDependencies{Broadcaster: broadcaster}}
	svc.broadcastRestoredVisits(testpkg.Ctx(t), 99, nil)
	assert.Empty(t, broadcaster.Calls())
}

func TestValidateLegacyWeekendInstanceDate(t *testing.T) {
	t.Parallel()

	saturday := calendar.NewDate(2026, time.May, 9)
	monday := saturday.AddDays(2)

	assert.NoError(t, validateLegacyWeekendInstanceDate(context.Background(), monday, monday), "weekday updates stay valid")
	assert.NoError(t, validateLegacyWeekendInstanceDate(context.Background(), saturday, saturday), "legacy weekend rows may retain their original date")
	assert.ErrorIs(t, validateLegacyWeekendInstanceDate(context.Background(), monday, saturday), timetable.ErrInstanceWeekend,
		"new weekend dates must be rejected")
}

func TestInstanceDelete_PlannedTemplateBackedCreatesCancellationException(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7301)
	groupID := int64(410)
	date := calendar.NewDate(2026, 7, 6)
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
	assert.Equal(t, calendar.Date(date), created.ExceptionDate)
	assert.Equal(t, scheduleModel.ActivityExceptionCancelled, created.ExceptionType)
	require.NotNil(t, created.Reason)
	assert.Equal(t, deletedSlotReason, *created.Reason)
	assert.Equal(t, int64(7301), created.TenantID)
}

func TestInstanceDelete_AmbiguousTemplateBackedDateDoesNotDelete(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7302)
	groupID := int64(411)
	date := calendar.NewDate(2026, 7, 7)
	inst := deleteUnitInstance(102, &groupID, date, scheduleModel.InstanceStatusPlanned, false)
	otherSlot := deleteUnitInstance(103, &groupID, date, scheduleModel.InstanceStatusPlanned, false)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst, sameDayRows: []*scheduleModel.ActivityInstance{inst, nil, otherSlot}}
	exceptionRepo := &deleteUnitExceptionRepo{}
	svc := deleteUnitService(instanceRepo, exceptionRepo)

	err := svc.DeleteCancelled(ctx, inst.ID)

	require.ErrorIs(t, err, timetable.ErrAmbiguousTemplateInstanceDelete)
	assert.Empty(t, instanceRepo.deleted)
	assert.Empty(t, exceptionRepo.created)
	assert.Empty(t, exceptionRepo.updated)
}

func TestInstanceDelete_SpontaneousTemplateLinkedSkipsException(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7303)
	groupID := int64(412)
	inst := deleteUnitInstance(104, &groupID, calendar.NewDate(2026, 7, 8), scheduleModel.InstanceStatusPlanned, true)
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
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7304)
	groupID := int64(413)
	date := calendar.NewDate(2026, 7, 9)
	inst := deleteUnitInstance(105, &groupID, date, scheduleModel.InstanceStatusCancelled, false)
	start := time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC)
	end := time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC)
	roomID := int64(900)
	reason := "Raumwechsel"
	existing := &scheduleModel.ActivityException{
		ActivityGroupID: groupID,
		ExceptionDate:   calendar.Date(date),
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
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7305)
	groupID := int64(414)
	date := calendar.NewDate(2026, 7, 10)
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
			var scheduleErr *ports.ScheduleError
			require.ErrorAs(t, err, &scheduleErr)
			assert.Equal(t, tt.wantOp, scheduleErr.Op)
		})
	}
}

func TestInstanceDelete_RejectsProtectedStatuses(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTenantID(context.Background(), 7306)
	for _, status := range []string{scheduleModel.InstanceStatusActive, scheduleModel.InstanceStatusCompleted} {
		t.Run(status, func(t *testing.T) {
			inst := deleteUnitInstance(107, nil, calendar.NewDate(2026, 7, 11), status, false)
			instanceRepo := &deleteUnitInstanceRepo{instance: inst}
			svc := deleteUnitService(instanceRepo, &deleteUnitExceptionRepo{})

			err := svc.DeleteCancelled(ctx, inst.ID)

			require.ErrorIs(t, err, timetable.ErrInvalidInstanceTransition)
			assert.Empty(t, instanceRepo.deleted)
		})
	}
}

func deleteUnitService(instanceRepo *deleteUnitInstanceRepo, exceptionRepo *deleteUnitExceptionRepo) *InstanceLifecycleService {
	return &InstanceLifecycleService{deps: ports.InstanceLifecycleDependencies{
		InstanceRepo:  fakeInstanceRows(instanceRepo),
		ExceptionRepo: exceptionRepo,
		Logger:        slog.New(slog.DiscardHandler),
	}}
}

func deleteUnitInstance(id int64, groupID *int64, date calendar.Date, status string, spontaneous bool) *scheduleModel.ActivityInstance {
	inst := &scheduleModel.ActivityInstance{
		Date:            calendar.Date(date),
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

func (r *deleteUnitInstanceRepo) FindByID(_ context.Context, _ int64) (*scheduleModel.ActivityInstance, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.instance, nil
}

func (r *deleteUnitInstanceRepo) FindByActivityGroupAndDate(_ context.Context, _ int64, _ calendar.Date) ([]*scheduleModel.ActivityInstance, error) {
	r.sameDayCalls++
	if r.sameDayErr != nil {
		return nil, r.sameDayErr
	}
	return r.sameDayRows, nil
}

func (r *deleteUnitInstanceRepo) Delete(_ context.Context, nativeRecordID int64) error {
	var id any = nativeRecordID
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if v, ok := id.(int64); ok {
		r.deleted = append(r.deleted, v)
	}
	return nil
}

// deleteUnitExceptionRepo serves the slot exception of the owner's
// exception capability and records the stored rows, stamped with the tenant
// of the request context the way the store stamps them.
type deleteUnitExceptionRepo struct {
	timetable.ActivityExceptionCapability
	existing  *scheduleModel.ActivityException
	findErr   error
	createErr error
	updateErr error
	findCalls int
	created   []*scheduleModel.ActivityException
	updated   []*scheduleModel.ActivityException
}

func (r *deleteUnitExceptionRepo) ListActivityExceptions(ctx context.Context, _ timetable.ActivityExceptionFilter) ([]timetable.ActivityException, error) {
	r.findCalls++
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.existing == nil {
		return nil, nil
	}
	return []timetable.ActivityException{deleteUnitStoredException(ctx, publicActivityExceptionInput(r.existing))}, nil
}

func (r *deleteUnitExceptionRepo) CreateActivityException(ctx context.Context, input timetable.ActivityExceptionInput) (timetable.ActivityException, error) {
	if r.createErr != nil {
		return timetable.ActivityException{}, r.createErr
	}
	stored := deleteUnitStoredException(ctx, input)
	row, err := legacyActivityException(stored)
	if err != nil {
		return timetable.ActivityException{}, err
	}
	r.created = append(r.created, row)
	return stored, nil
}

func (r *deleteUnitExceptionRepo) UpdateActivityException(ctx context.Context, _ int64, input timetable.ActivityExceptionInput) (timetable.ActivityException, error) {
	if r.updateErr != nil {
		return timetable.ActivityException{}, r.updateErr
	}
	stored := deleteUnitStoredException(ctx, input)
	row, err := legacyActivityException(stored)
	if err != nil {
		return timetable.ActivityException{}, err
	}
	r.updated = append(r.updated, row)
	return stored, nil
}

func deleteUnitStoredException(ctx context.Context, input timetable.ActivityExceptionInput) timetable.ActivityException {
	return timetable.ActivityException{
		TenantID: tenant.FromContext(ctx), ActivityGroupID: input.ActivityGroupID, ExceptionDate: input.ExceptionDate,
		ExceptionType: input.ExceptionType, StartTime: input.StartTime, EndTime: input.EndTime, RoomID: input.RoomID,
		Reason: input.Reason, CreatedBy: input.CreatedBy,
	}
}

func weekendPlanContext(follows bool, err error) context.Context {
	return calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) { return follows, err })
}

// A weekend that follows Friday's plan (#3921) is a care day: spontaneous
// starts and moves onto it are allowed, and its occurrences are re-planned.
func TestWeekendFollowsFridayOpensTheWeekend(t *testing.T) {
	t.Parallel()

	spontaneous := plannedLifecycleInstance()
	spontaneous.IsSpontaneous = true
	saturdayNoon := time.Date(2026, 8, 15, 12, 0, 0, 0, calendar.Berlin)
	assert.NoError(t, validateSpontaneousStartWorkday(weekendPlanContext(true, nil), spontaneous, saturdayNoon))
	assert.ErrorIs(t, validateSpontaneousStartWorkday(weekendPlanContext(false, nil), spontaneous, saturdayNoon), timetable.ErrInstanceWeekend)
	boom := errors.New("settings unavailable")
	err := validateSpontaneousStartWorkday(weekendPlanContext(true, boom), spontaneous, saturdayNoon)
	require.ErrorIs(t, err, boom, "a failed read is no refusal")
	assert.NotErrorIs(t, err, timetable.ErrInstanceWeekend)

	saturday := calendar.NewDate(2026, time.May, 9)
	monday := saturday.AddDays(2)
	assert.NoError(t, validateLegacyWeekendInstanceDate(weekendPlanContext(true, nil), monday, saturday))
	assert.ErrorIs(t, validateLegacyWeekendInstanceDate(weekendPlanContext(false, nil), monday, saturday), timetable.ErrInstanceWeekend)

	groupID := int64(7)
	weekendOccurrence := plannedLifecycleInstance()
	weekendOccurrence.Date = calendar.Date(saturday)
	weekendOccurrence.ActivityGroupID = &groupID
	weekendOccurrence.Status = scheduleModel.InstanceStatusPlanned
	assert.True(t, replannableOccurrence(weekendOccurrence, nil, true))
	assert.False(t, replannableOccurrence(weekendOccurrence, nil, false))
}
