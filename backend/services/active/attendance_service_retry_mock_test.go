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
	existing *activeModels.Attendance
}

func (r *retryAttendanceRepository) CreateIfNoOpenForToday(context.Context, *activeModels.Attendance) (bool, error) {
	return false, nil
}

func (r *retryAttendanceRepository) GetStudentCurrentStatus(context.Context, int64) (*activeModels.Attendance, error) {
	return r.existing, nil
}

func TestPerformCheckIn_BinaryRetryMirrorsExistingCheckInTime(t *testing.T) {
	existingCheckIn := time.Date(2026, 7, 15, 7, 15, 0, 0, time.UTC)
	retryTime := existingCheckIn.Add(30 * time.Minute)
	existing := &activeModels.Attendance{
		Model:       base.Model{ID: 701},
		StudentID:   702,
		CheckInTime: existingCheckIn,
	}
	syncer := &recordingAttendanceSyncer{}
	svc := &service{
		ServiceDependencies: ServiceDependencies{
			AttendanceRepo:   &retryAttendanceRepository{existing: existing},
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
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, syncer.mirrorAt, 1)
	assert.Equal(t, existing.StudentID, syncer.mirrorAt[0].studentID)
	assert.Equal(t, existingCheckIn, syncer.mirrorAt[0].at)
	assert.Equal(t, existingCheckIn, result.Timestamp)
}
