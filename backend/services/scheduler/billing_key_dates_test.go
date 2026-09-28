// Billing key-date tick tests (#2791). Pure in-memory: the directory is a
// fake; when the counts are due and what they are is Organisation &
// Tenancy's business and is tested there.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBillingDirectory struct {
	dbTenantDirectory
	mu      sync.Mutex
	calls   []time.Time
	written int
	err     error
}

func (d *fakeBillingDirectory) RecordDueBillingKeyDates(ctx context.Context, now time.Time) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, now)
	return d.written, d.err
}

func TestBillingKeyDatesTickHandsTheCurrentInstantToTheCapture(t *testing.T) {
	t.Parallel()
	directory := &fakeBillingDirectory{written: 4}
	s := unitScheduler(&Scheduler{logger: slog.Default()})
	s.schoolRepo = directory

	before := time.Now()
	s.checkAndRunBillingKeyDates(context.Background(), &ScheduledTask{Name: "billing-key-dates"})

	require.Len(t, directory.calls, 1)
	assert.False(t, directory.calls[0].Before(before), "the capture decides on the tick's own clock")
}

func TestBillingKeyDatesTickSurvivesAFailedCapture(t *testing.T) {
	t.Parallel()
	directory := &fakeBillingDirectory{err: errors.New("database unavailable")}
	s := unitScheduler(&Scheduler{logger: slog.Default()})
	s.schoolRepo = directory
	task := &ScheduledTask{Name: "billing-key-dates"}

	s.checkAndRunBillingKeyDates(context.Background(), task)
	s.checkAndRunBillingKeyDates(context.Background(), task)

	assert.Len(t, directory.calls, 2, "a failed capture is retried on the next tick")
	assert.False(t, task.Running)
}

func TestBillingKeyDatesTickSkipsWhileARunIsInProgress(t *testing.T) {
	t.Parallel()
	directory := &fakeBillingDirectory{}
	s := unitScheduler(&Scheduler{logger: slog.Default()})
	s.schoolRepo = directory
	task := &ScheduledTask{Name: "billing-key-dates", Running: true}

	s.checkAndRunBillingKeyDates(context.Background(), task)

	assert.Empty(t, directory.calls)
}
