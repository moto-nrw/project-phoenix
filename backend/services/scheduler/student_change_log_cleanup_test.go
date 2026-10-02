package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A failed change-history sweep fails the school's tenant transaction, so
// the deletion audit the People Directory sweep wrote before its delete rolls
// back with it, and the school stays eligible for the next matching minute.
// The sweep's own audit-then-delete order is covered in services/users.
func TestStudentChangeLogCleanup_DeleteFailureRollsBackDeletionAudit(t *testing.T) {
	t.Parallel()

	scheduleNow := time.Now()
	if scheduleNow.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race on timeMatchesNow")
	}

	deleteErr := errors.New("delete failed")
	var results []string
	s := unitScheduler(&Scheduler{
		studentChangeLogCleanup: func(context.Context) (StudentChangeLogCleanupResult, error) {
			return StudentChangeLogCleanupResult{}, deleteErr
		},
		settings: &fakeSettingsResolver{
			boolValues: map[string]bool{
				settingDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				settingDataCleanupTime: scheduleNow.Format("15:04"),
			},
			intValues: map[string]int{
				settingDataCleanupTimeoutMinutes: 30,
			},
		},
		unitOfWorkObserver: func(_, kind, result string, _ time.Duration, _ int) {
			if kind == unitOfWorkTransaction {
				results = append(results, result)
			}
		},
		logger: slog.Default()})

	s.checkAndRunStudentChangeLogCleanup(context.Background(), &ScheduledTask{Name: "student-change-log-cleanup"})

	require.Equal(t, []string{"rollback"}, results, "the failed sweep rolls its tenant transaction back")
	_, ranToday := s.lastStudentChangeLogCleanup.Load(schedulerUnitTenantID)
	assert.False(t, ranToday, "failed cleanup must remain eligible for retry")
}

func TestStudentChangeLogCleanup_CommitsAndMarksTheDay(t *testing.T) {
	t.Parallel()

	scheduleNow := time.Now()
	if scheduleNow.Second() >= 58 {
		t.Skip("skipping to avoid minute-boundary race on timeMatchesNow")
	}

	calls := 0
	s := unitScheduler(&Scheduler{
		studentChangeLogCleanup: func(context.Context) (StudentChangeLogCleanupResult, error) {
			calls++
			return StudentChangeLogCleanupResult{EditsDeleted: 2, StudentsAffected: 1, RetentionDays: 90}, nil
		},
		settings: &fakeSettingsResolver{
			boolValues:   map[string]bool{settingDataCleanupEnabled: true},
			stringValues: map[string]string{settingDataCleanupTime: scheduleNow.Format("15:04")},
			intValues:    map[string]int{settingDataCleanupTimeoutMinutes: 30},
		},
		logger: slog.Default()})

	s.checkAndRunStudentChangeLogCleanup(context.Background(), &ScheduledTask{Name: "student-change-log-cleanup"})

	assert.Equal(t, 1, calls)
	assert.True(t, wasRunToday(&s.lastStudentChangeLogCleanup, schedulerUnitTenantID))
}
