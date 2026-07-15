package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceStart_SchulhofRequiresDedicatedSupervision(t *testing.T) {
	inst := deleteUnitInstance(100, nil, timezone.NewDate(2026, 7, 5), scheduleModel.InstanceStatusPlanned, false)
	instanceRepo := &deleteUnitInstanceRepo{instance: inst}
	roomRepo := &startGuardRoomRepo{room: &facilitiesModel.Room{Name: constants.SchulhofRoomName}}
	svc := &instanceService{deps: InstanceServiceDependencies{
		InstanceRepo: instanceRepo,
		RoomRepo:     roomRepo,
		Logger:       slog.New(slog.DiscardHandler),
	}}

	result, err := svc.Start(context.Background(), inst.ID, 0)

	require.ErrorIs(t, err, ErrSchulhofSupervisionRequired)
	assert.Nil(t, result)
	assert.Equal(t, 1, roomRepo.findCalls)
}

func TestInstanceStart_RoomLookupFailuresAreInternal(t *testing.T) {
	inst := deleteUnitInstance(99, nil, timezone.NewDate(2026, 7, 4), scheduleModel.InstanceStatusPlanned, false)
	tests := []struct {
		name     string
		roomRepo *startGuardRoomRepo
	}{
		{name: "repository error", roomRepo: &startGuardRoomRepo{err: errors.New("room lookup failed")}},
		{name: "missing room", roomRepo: &startGuardRoomRepo{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &instanceService{deps: InstanceServiceDependencies{
				InstanceRepo: &deleteUnitInstanceRepo{instance: inst},
				RoomRepo:     tt.roomRepo,
				Logger:       slog.New(slog.DiscardHandler),
			}}

			result, err := svc.Start(context.Background(), inst.ID, 0)

			assert.Nil(t, result)
			var scheduleErr *ScheduleError
			require.ErrorAs(t, err, &scheduleErr)
			assert.Equal(t, "start instance: load room", scheduleErr.Op)
		})
	}
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

type startGuardRoomRepo struct {
	facilitiesModel.RoomRepository
	room      *facilitiesModel.Room
	err       error
	findCalls int
}

func (r *startGuardRoomRepo) FindByID(_ context.Context, _ any) (*facilitiesModel.Room, error) {
	r.findCalls++
	return r.room, r.err
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
