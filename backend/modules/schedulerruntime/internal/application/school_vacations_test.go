// School vacation sync tick tests (#5345). Pure in-memory: the sync is a
// fake; what it stores is tested with the School Calendar composition.
package application

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchoolVacationTickSyncsOncePerRun(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	s := unitScheduler(&unitSchedulerDependencies{logger: slog.Default()})
	s.schoolVacations = func(context.Context) (int, int, error) {
		calls.Add(1)
		return 16, 230, nil
	}

	s.checkAndRunSchoolVacations(context.Background(), &ScheduledTask{Name: "school-vacation-sync"})

	assert.Equal(t, int32(1), calls.Load())
}

func TestSchoolVacationTickSurvivesAFailedSync(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	s := unitScheduler(&unitSchedulerDependencies{logger: slog.Default()})
	s.schoolVacations = func(context.Context) (int, int, error) {
		calls.Add(1)
		return 0, 0, errors.New("openholidays unavailable")
	}
	task := &ScheduledTask{Name: "school-vacation-sync"}

	s.checkAndRunSchoolVacations(context.Background(), task)
	s.checkAndRunSchoolVacations(context.Background(), task)

	assert.Equal(t, int32(2), calls.Load(), "a failed sync is retried on the next tick")
	assert.False(t, task.Running)
}

func TestSchoolVacationTickSkipsWhileARunIsInProgress(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	s := unitScheduler(&unitSchedulerDependencies{logger: slog.Default()})
	s.schoolVacations = func(context.Context) (int, int, error) {
		calls.Add(1)
		return 0, 0, nil
	}

	s.checkAndRunSchoolVacations(context.Background(), &ScheduledTask{Name: "school-vacation-sync", Running: true})

	assert.Zero(t, calls.Load())
}
