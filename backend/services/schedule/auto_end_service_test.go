package schedule

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutoEnd_RunForTenant_CompletesOnlyDueActivePlannedInstances(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 14, 15, 0, 0, timezone.Berlin)
	duePastGrace := autoStartInstance(101, schedule.InstanceStatusActive, 13, 0, 13, 45)
	dueAfterGrace := autoStartInstance(102, schedule.InstanceStatusActive, 13, 0, 14, 0)
	notDue := autoStartInstance(103, schedule.InstanceStatusActive, 13, 0, 14, 1)
	neverStarted := autoStartInstance(104, schedule.InstanceStatusPlanned, 13, 0, 14, 0)
	spontaneous := autoStartInstance(105, schedule.InstanceStatusActive, 13, 0, 14, 0)
	spontaneous.IsSpontaneous = true
	alreadyCompleted := autoStartInstance(106, schedule.InstanceStatusCompleted, 13, 0, 14, 0)
	overdueFromYesterday := autoStartInstance(107, schedule.InstanceStatusActive, 13, 0, 14, 0)
	overdueFromYesterday.Date = overdueFromYesterday.Date.AddDays(-1)

	completer := &autoEndCompleter{}
	svc := NewAutoEndService(&autoStartInstanceRepo{instances: []*schedule.ActivityInstance{
		duePastGrace, dueAfterGrace, notDue, neverStarted, spontaneous, alreadyCompleted, overdueFromYesterday,
	}}, completer)

	result, err := svc.RunForTenant(context.Background(), now, 15*time.Minute)

	require.NoError(t, err)
	assert.Equal(t, 7, result.Checked)
	assert.Equal(t, 3, result.Completed)
	assert.Equal(t, 1, result.SkippedBeforeDeadline)
	assert.Equal(t, 1, result.SkippedSpontaneous)
	assert.Equal(t, 2, result.SkippedNonActive)
	assert.Equal(t, []int64{101, 102, 107}, completer.completedIDs)
}

func TestAutoEnd_RunForTenant_UsesZeroGrace(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 14, 0, 0, 0, timezone.Berlin)
	completer := &autoEndCompleter{}
	svc := NewAutoEndService(&autoStartInstanceRepo{instances: []*schedule.ActivityInstance{
		autoStartInstance(201, schedule.InstanceStatusActive, 13, 0, 14, 0),
	}}, completer)

	result, err := svc.RunForTenant(context.Background(), now, 0)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Completed)
	assert.Equal(t, []int64{201}, completer.completedIDs)
}

func TestAutoEnd_RunForTenant_UsesBerlinBoundaryForUTCInstant(t *testing.T) {
	t.Parallel()

	// 12:00 UTC is 14:00 in Berlin on this date (CEST).
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	completer := &autoEndCompleter{}
	svc := NewAutoEndService(&autoStartInstanceRepo{instances: []*schedule.ActivityInstance{
		autoStartInstance(251, schedule.InstanceStatusActive, 13, 0, 14, 0),
	}}, completer)

	result, err := svc.RunForTenant(context.Background(), now, 0)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Completed)
	assert.Equal(t, []int64{251}, completer.completedIDs)
}

func TestAutoEnd_RunForTenant_TreatsConcurrentCompletionAsIdempotent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 14, 15, 0, 0, timezone.Berlin)
	completer := &autoEndCompleter{errorsByID: map[int64]error{
		301: fmt.Errorf("manual completion won: %w", ErrInvalidInstanceTransition),
	}}
	svc := NewAutoEndService(&autoStartInstanceRepo{
		instances: []*schedule.ActivityInstance{
			autoStartInstance(301, schedule.InstanceStatusActive, 13, 0, 14, 0),
			autoStartInstance(302, schedule.InstanceStatusActive, 13, 0, 14, 0),
		},
		findStatusByID: map[int64]string{301: schedule.InstanceStatusCompleted},
	}, completer)

	result, err := svc.RunForTenant(context.Background(), now, 15*time.Minute)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Completed)
	assert.Equal(t, 1, result.SkippedConcurrent)
	assert.Equal(t, []int64{301, 302}, completer.completedIDs)
}

func TestAutoEnd_RunForTenant_DoesNotHideCorruptActiveInstance(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 14, 15, 0, 0, timezone.Berlin)
	completer := &autoEndCompleter{errorsByID: map[int64]error{
		401: fmt.Errorf("active group is already closed: %w", ErrInvalidInstanceTransition),
	}}
	svc := NewAutoEndService(&autoStartInstanceRepo{instances: []*schedule.ActivityInstance{
		autoStartInstance(401, schedule.InstanceStatusActive, 13, 0, 14, 0),
	}}, completer)

	result, err := svc.RunForTenant(context.Background(), now, 15*time.Minute)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInstanceTransition)
	assert.Equal(t, 1, result.Failed)
	assert.Zero(t, result.SkippedConcurrent)
}

type autoEndCompleter struct {
	InstanceService
	completedIDs []int64
	errorsByID   map[int64]error
}

func (c *autoEndCompleter) Complete(_ context.Context, instanceID int64) (*schedule.ActivityInstance, error) {
	c.completedIDs = append(c.completedIDs, instanceID)
	if err := c.errorsByID[instanceID]; err != nil {
		return nil, err
	}
	return &schedule.ActivityInstance{Status: schedule.InstanceStatusCompleted}, nil
}
