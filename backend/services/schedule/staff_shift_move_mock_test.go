package schedule

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shiftMoveExceptionRepo struct {
	createFunc func(context.Context, *scheduleModels.StaffShiftSeriesException) error
}

func (m *shiftMoveExceptionRepo) Create(ctx context.Context, exception *scheduleModels.StaffShiftSeriesException) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, exception)
	}
	return nil
}

func (*shiftMoveExceptionRepo) FindDatesBySeriesID(context.Context, int64) ([]timezone.Date, error) {
	return nil, nil
}

func (*shiftMoveExceptionRepo) RepointToSeriesFrom(context.Context, int64, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func TestShiftService_MoveAcrossStaffPreservesIdentityAndConsumesSeriesOccurrence(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	reason := "Zeiten angepasst"
	seriesID := int64(17)
	existing := validShift(7)
	existing.ID = 5
	existing.SeriesID = &seriesID
	existing.SeriesOccurrenceDate = &existing.Date
	existing.Notes = "Nur Hintereingang"
	existing.ChangeReason = &reason
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}

	var persisted *scheduleModels.StaffShift
	repo.updateFunc = func(_ context.Context, shift *scheduleModels.StaffShift) error {
		persisted = shift
		return nil
	}
	var exception *scheduleModels.StaffShiftSeriesException
	svc.SetSeriesExceptionRepo(&shiftMoveExceptionRepo{
		createFunc: func(_ context.Context, row *scheduleModels.StaffShiftSeriesException) error {
			exception = row
			return nil
		},
	})

	moved, err := svc.MoveShift(context.Background(), MoveShiftInput{
		ShiftID:       existing.ID,
		SourceStaffID: existing.StaffID,
		TargetStaffID: 8,
		Date:          existing.Date.AddDays(1),
		StartTime:     existing.StartTime,
		EndTime:       existing.EndTime,
		BreakMinutes:  existing.BreakMinutes,
		ShiftTypeID:   existing.ShiftTypeID,
		ActorStaffID:  9,
	})

	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Equal(t, existing.ID, moved.ID, "move must retain the concrete row ID")
	assert.Equal(t, int64(8), moved.StaffID)
	assert.Equal(t, "Nur Hintereingang", moved.Notes)
	assert.Equal(t, &reason, moved.ChangeReason)
	assert.Nil(t, moved.SeriesID, "a target person cannot inherit another person's series")
	assert.Nil(t, moved.SeriesOccurrenceDate)
	assert.False(t, moved.Detached)
	require.NotNil(t, exception)
	assert.Equal(t, seriesID, exception.SeriesID)
	assert.Equal(t, existing.Date, exception.Date)
	assert.Equal(t, existing.CreatedBy, exception.CreatedBy)
}

func TestShiftService_MoveRetryReturnsAlreadyMovedRowWithoutWriting(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	alreadyMoved := validShift(8)
	alreadyMoved.ID = 5
	alreadyMoved.Date = alreadyMoved.Date.AddDays(1)
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return alreadyMoved, nil
	}
	updates := 0
	repo.updateFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		updates++
		return nil
	}

	moved, err := svc.MoveShift(context.Background(), MoveShiftInput{
		ShiftID:       alreadyMoved.ID,
		SourceStaffID: 7,
		TargetStaffID: alreadyMoved.StaffID,
		Date:          alreadyMoved.Date,
		StartTime:     alreadyMoved.StartTime,
		EndTime:       alreadyMoved.EndTime,
		BreakMinutes:  alreadyMoved.BreakMinutes,
		ShiftTypeID:   alreadyMoved.ShiftTypeID,
	})

	require.NoError(t, err)
	assert.Same(t, alreadyMoved, moved)
	assert.Zero(t, updates, "a retry must not perform a second write")
}

func TestShiftService_MoveSeriesDateRetainsDeviationAndConsumesOriginalDate(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	seriesID := int64(17)
	existing := validShift(7)
	existing.ID = 5
	existing.SeriesID = &seriesID
	existing.SeriesOccurrenceDate = &existing.Date
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	var exceptionDate timezone.Date
	svc.SetSeriesExceptionRepo(&shiftMoveExceptionRepo{
		createFunc: func(_ context.Context, row *scheduleModels.StaffShiftSeriesException) error {
			exceptionDate = row.Date
			return nil
		},
	})

	moved, err := svc.MoveShift(context.Background(), MoveShiftInput{
		ShiftID:       existing.ID,
		SourceStaffID: existing.StaffID,
		TargetStaffID: existing.StaffID,
		Date:          existing.Date.AddDays(1),
		StartTime:     existing.StartTime,
		EndTime:       existing.EndTime,
		BreakMinutes:  existing.BreakMinutes,
		ShiftTypeID:   existing.ShiftTypeID,
	})

	require.NoError(t, err)
	require.NotNil(t, moved.SeriesID)
	assert.Equal(t, seriesID, *moved.SeriesID)
	assert.True(t, moved.Detached)
	assert.Equal(t, existing.Date, exceptionDate)
}

func TestShiftService_MoveSeriesNoOpDoesNotDetachOrWrite(t *testing.T) {
	svc, repo, _ := shiftServiceFixture()
	seriesID := int64(17)
	existing := validShift(7)
	existing.ID = 5
	existing.SeriesID = &seriesID
	existing.SeriesOccurrenceDate = &existing.Date
	repo.findByIDFunc = func(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
		return existing, nil
	}
	updates := 0
	repo.updateFunc = func(_ context.Context, _ *scheduleModels.StaffShift) error {
		updates++
		return nil
	}
	exceptions := 0
	svc.SetSeriesExceptionRepo(&shiftMoveExceptionRepo{
		createFunc: func(_ context.Context, _ *scheduleModels.StaffShiftSeriesException) error {
			exceptions++
			return nil
		},
	})

	moved, err := svc.MoveShift(context.Background(), MoveShiftInput{
		ShiftID:       existing.ID,
		SourceStaffID: existing.StaffID,
		TargetStaffID: existing.StaffID,
		Date:          existing.Date,
		StartTime:     existing.StartTime,
		EndTime:       existing.EndTime,
		BreakMinutes:  existing.BreakMinutes,
		ShiftTypeID:   existing.ShiftTypeID,
	})

	require.NoError(t, err)
	assert.Same(t, existing, moved)
	assert.False(t, moved.Detached)
	assert.Zero(t, updates)
	assert.Zero(t, exceptions)
}
