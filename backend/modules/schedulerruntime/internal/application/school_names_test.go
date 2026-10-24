// Analytics school-name tick tests. Pure in-memory: the directory is a fake;
// which schools are named and how is tested with the tracker and the root
// binding.
package application

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeSchoolNamesDirectory struct {
	dbTenantDirectory
	mu    sync.Mutex
	calls int
	err   error
}

func (d *fakeSchoolNamesDirectory) PublishSchoolNames(context.Context) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	return 3, d.err
}

func TestSchoolNamesTickPublishesOncePerRun(t *testing.T) {
	t.Parallel()
	directory := &fakeSchoolNamesDirectory{}
	s := unitScheduler(&unitSchedulerDependencies{logger: slog.Default()})
	s.schoolRepo = directory

	s.checkAndRunSchoolNames(context.Background(), &ScheduledTask{Name: "analytics-school-names"})

	assert.Equal(t, 1, directory.calls)
}

func TestSchoolNamesTickSurvivesAFailedPublish(t *testing.T) {
	t.Parallel()
	directory := &fakeSchoolNamesDirectory{err: errors.New("database unavailable")}
	s := unitScheduler(&unitSchedulerDependencies{logger: slog.Default()})
	s.schoolRepo = directory
	task := &ScheduledTask{Name: "analytics-school-names"}

	s.checkAndRunSchoolNames(context.Background(), task)
	s.checkAndRunSchoolNames(context.Background(), task)

	assert.Equal(t, 2, directory.calls, "a failed publish is retried on the next tick")
	assert.False(t, task.Running)
}

func TestSchoolNamesTickSkipsWhileARunIsInProgress(t *testing.T) {
	t.Parallel()
	directory := &fakeSchoolNamesDirectory{}
	s := unitScheduler(&unitSchedulerDependencies{logger: slog.Default()})
	s.schoolRepo = directory

	s.checkAndRunSchoolNames(context.Background(), &ScheduledTask{Name: "analytics-school-names", Running: true})

	assert.Zero(t, directory.calls)
}
