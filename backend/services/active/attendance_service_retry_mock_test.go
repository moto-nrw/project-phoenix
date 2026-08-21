package active

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retryAttendanceRepository struct {
	activeModels.AttendanceRepository
	existing    *activeModels.Attendance
	fetchedDate timezone.Date
}

func (r *retryAttendanceRepository) CreateIfNoOpenForToday(context.Context, *activeModels.Attendance) (bool, error) {
	return false, nil
}

func (r *retryAttendanceRepository) FindForDateByStudentIDs(_ context.Context, date timezone.Date, _ []int64) ([]*activeModels.Attendance, error) {
	r.fetchedDate = date
	return []*activeModels.Attendance{r.existing}, nil
}

func TestPerformCheckIn_BinaryRetryMirrorsExistingCheckInTime(t *testing.T) {
	t.Parallel()

	existingCheckIn := time.Date(2026, 7, 15, 7, 15, 0, 0, time.UTC)
	retryTime := existingCheckIn.Add(30 * time.Minute)
	existing := &activeModels.Attendance{
		Model:       base.Model{ID: 701},
		StudentID:   702,
		CheckInTime: existingCheckIn,
	}
	syncer := &recordingAttendanceSyncer{}
	repo := &retryAttendanceRepository{existing: existing}
	svc := &service{
		ServiceDependencies: ServiceDependencies{
			AttendanceRepo:   repo,
			AttendanceSyncer: syncer,
		},
		settings: &fakeSettingsResolver{hasOverride: true, resolved: "binary"},
	}

	result, err := svc.performCheckIn(
		context.Background(),
		existing.StudentID,
		703,
		704,
		retryTime,
		timezone.DateFromTime(existingCheckIn),
		checkinTypeToggle,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, syncer.mirrorAt, 1)
	assert.Equal(t, existing.StudentID, syncer.mirrorAt[0].studentID)
	assert.Equal(t, existingCheckIn, syncer.mirrorAt[0].at)
	assert.Equal(t, existingCheckIn, result.Timestamp)
	// The absorbed-conflict re-fetch must use the caller-supplied snapshot
	// date, not a re-derived "today" (review #2372).
	assert.Equal(t, timezone.DateFromTime(existingCheckIn), repo.fetchedDate)
}
