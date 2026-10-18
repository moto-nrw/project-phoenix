package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A failed change-history sweep fails the school's tenant transaction, so
// the deletion audit the People Directory sweep wrote before its delete rolls
// back with it, and the school stays eligible for the next matching minute.
// The sweep's own audit-then-delete order is covered in services/users.
func TestStudentChangeLogCleanup_DeleteFailureRollsBackDeletionAudit(t *testing.T) {
	t.Parallel()

	scheduleNow := berlin(14, 2, 0)

	deleteErr := errors.New("delete failed")
	var results []string
	s := unitScheduler(func(unit *Scheduler) {
		unit.studentChangeLogCleanup = func(context.Context) (ports.StudentChangeLogCleanupResult, error) {
			return ports.StudentChangeLogCleanupResult{}, deleteErr
		}
		unit.cleanupClock = func() time.Time { return scheduleNow }
		unit.settings = &fakeSettingsResolver{
			boolValues: map[string]bool{
				settingDataCleanupEnabled: true,
			},
			stringValues: map[string]string{
				settingDataCleanupTime: scheduleNow.Format("15:04"),
			},
			intValues: map[string]int{
				settingDataCleanupTimeoutMinutes: 30,
			},
		}
		unit.unitOfWorkObserver = func(_, kind, result string, _ time.Duration, _ int) {
			if kind == unitOfWorkTransaction {
				results = append(results, result)
			}
		}
		unit.logger = slog.Default()
	})

	s.checkAndRunStudentChangeLogCleanup(context.Background(), &ScheduledTask{Name: "student-change-log-cleanup"})

	require.Equal(t, []string{"rollback"}, results, "the failed sweep rolls its tenant transaction back")
	_, ranToday := s.lastStudentChangeLogCleanup.Load(schedulerUnitTenantID)
	assert.False(t, ranToday, "failed cleanup must remain eligible for retry")
}

func TestStudentChangeLogCleanup_CommitsAndMarksTheDay(t *testing.T) {
	t.Parallel()

	scheduleNow := berlin(14, 2, 0)

	calls := 0
	s := unitScheduler(func(unit *Scheduler) {
		unit.studentChangeLogCleanup = func(context.Context) (ports.StudentChangeLogCleanupResult, error) {
			calls++
			return ports.StudentChangeLogCleanupResult{EditsDeleted: 2, StudentsAffected: 1, RetentionDays: 90}, nil
		}
		unit.cleanupClock = func() time.Time { return scheduleNow }
		unit.settings = &fakeSettingsResolver{
			boolValues:   map[string]bool{settingDataCleanupEnabled: true},
			stringValues: map[string]string{settingDataCleanupTime: scheduleNow.Format("15:04")},
			intValues:    map[string]int{settingDataCleanupTimeoutMinutes: 30},
		}
		unit.logger = slog.Default()
	})

	s.checkAndRunStudentChangeLogCleanup(context.Background(), &ScheduledTask{Name: "student-change-log-cleanup"})

	assert.Equal(t, 1, calls)
	assert.True(t, wasRunAt(&s.lastStudentChangeLogCleanup, schedulerUnitTenantID, scheduleNow))
}
