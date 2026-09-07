package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retryAttendanceRepository struct {
	StudentPresence
	existing    studentpresence.Attendance
	fetchedDate studentpresence.AttendanceFilter
}

func (r *retryAttendanceRepository) EnsureAttendance(context.Context, studentpresence.Attendance) (studentpresence.Attendance, bool, error) {
	return studentpresence.Attendance{}, false, nil
}

func (r *retryAttendanceRepository) ListAttendance(_ context.Context, filter studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	r.fetchedDate = filter
	return []studentpresence.Attendance{r.existing}, nil
}

func TestPerformCheckIn_BinaryRetryMirrorsExistingCheckInTime(t *testing.T) {
	t.Parallel()

	existingCheckIn := time.Date(2026, 7, 15, 7, 15, 0, 0, time.UTC)
	retryTime := existingCheckIn.Add(30 * time.Minute)
	existing := studentpresence.Attendance{
		ID:          701,
		StudentID:   702,
		CheckInTime: existingCheckIn,
	}
	syncer := &recordingAttendanceSyncer{}
	repo := &retryAttendanceRepository{existing: existing}
	svc := &service{
		ServiceDependencies: ServiceDependencies{
			SchoolPresence:   repo,
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
	assert.Equal(t, studentpresence.AttendanceFilter{StudentIDs: []int64{existing.StudentID}, FromDate: timezone.DateFromTime(existingCheckIn).String(), UntilDate: timezone.DateFromTime(existingCheckIn).String()}, repo.fetchedDate)
}

func TestPerformCheckIn_BinaryRetryReturnsSyncFailure(t *testing.T) {
	t.Parallel()
	existingTime := time.Date(2026, 7, 15, 7, 15, 0, 0, time.UTC)
	injected := errors.New("retry sync unavailable")
	syncer := &recordingAttendanceSyncer{mirrorAtErr: injected}
	repo := &retryAttendanceRepository{existing: studentpresence.Attendance{ID: 701, StudentID: 702, CheckInTime: existingTime}}
	svc := &service{
		ServiceDependencies: ServiceDependencies{SchoolPresence: repo, AttendanceSyncer: syncer},
		settings:            &fakeSettingsResolver{hasOverride: true, resolved: "binary"},
	}

	result, err := svc.performCheckIn(context.Background(), 702, 703, 704, existingTime.Add(time.Minute), timezone.DateFromTime(existingTime), checkinTypeToggle)

	require.ErrorIs(t, err, injected)
	assert.Nil(t, result)
	require.Len(t, syncer.mirrorAt, 1)
	assert.Equal(t, existingTime, syncer.mirrorAt[0].at)
}
