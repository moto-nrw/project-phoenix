package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// endedSessionSteps records the session-end pass over the retained slot rows
// and the Timetable owner's completion. The row effects of both are covered
// in workflows/sessionend and modules/timetable/compose; this suite pins the
// order, the arguments and the error handling of the scheduler's pass.
type endedSessionSteps struct {
	steps          []string
	closedGroups   []int64
	completedGroup []int64
	closedAt       time.Time
	completedAt    time.Time
	closeErr       error
	completeErr    error
}

func (r *endedSessionSteps) CloseOpenCheckoutsByActiveGroupIDs(_ context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error) {
	r.steps = append(r.steps, "close-checkouts")
	r.closedGroups, r.closedAt = activeGroupIDs, checkedOutAt
	return len(activeGroupIDs), r.closeErr
}

func (r *endedSessionSteps) CompleteActiveByActiveGroupIDs(_ context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	r.steps = append(r.steps, "complete-instances")
	r.completedGroup, r.completedAt = activeGroupIDs, completedAt
	return int64(len(activeGroupIDs)), r.completeErr
}

func TestCompleteTimetableInstancesForEndedSessions(t *testing.T) {
	t.Parallel()
	rows := &endedSessionSteps{}
	s := unitScheduler(&Scheduler{
		instanceStudentRepo: rows,
		timetableBridge:     rows,
		logger:              slog.Default()})

	completed, err := s.completeTimetableInstancesForEndedSessions(context.Background(), &studentpresence.DailySessionCleanupResult{
		EndedActiveGroupIDs: []int64{811, 812},
	})

	require.NoError(t, err)
	assert.Equal(t, 2, completed)
	assert.Equal(t, []string{"close-checkouts", "complete-instances"}, rows.steps,
		"open slot checkouts close before the blocks complete")
	assert.Equal(t, []int64{811, 812}, rows.closedGroups)
	assert.Equal(t, []int64{811, 812}, rows.completedGroup)
	assert.Equal(t, rows.closedAt, rows.completedAt, "checkout and completion share one instant")
}

func TestCompleteTimetableInstancesForEndedSessionsPropagatesFailures(t *testing.T) {
	t.Parallel()
	for name, rows := range map[string]*endedSessionSteps{
		"close checkouts":    {closeErr: errors.New("checkout close failed")},
		"complete instances": {completeErr: errors.New("completion failed")},
	} {
		t.Run(name, func(t *testing.T) {
			s := unitScheduler(&Scheduler{instanceStudentRepo: rows, timetableBridge: rows, logger: slog.Default()})

			_, err := s.completeTimetableInstancesForEndedSessions(context.Background(), &studentpresence.DailySessionCleanupResult{
				EndedActiveGroupIDs: []int64{821},
			})

			require.Error(t, err, "a failed sync rolls the session end back with it")
		})
	}
}

func TestCompleteTimetableInstancesForEndedSessionsSkipsWithoutEndedGroups(t *testing.T) {
	t.Parallel()
	rows := &endedSessionSteps{}
	s := unitScheduler(&Scheduler{instanceStudentRepo: rows, timetableBridge: rows, logger: slog.Default()})

	completed, err := s.completeTimetableInstancesForEndedSessions(context.Background(), &studentpresence.DailySessionCleanupResult{})

	require.NoError(t, err)
	assert.Zero(t, completed)
	assert.Empty(t, rows.steps)
}
