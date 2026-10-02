package compose

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateLifecycleAvailability_PlannedBoundaries(t *testing.T) {
	t.Parallel()

	instance := &scheduleModel.ActivityInstance{
		Date:      scheduleModel.NewDate(2026, 8, 13),
		StartTime: time.Date(1, 1, 1, 13, 45, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
	}

	before := evaluateLifecycleAvailability(instance, time.Date(2026, 8, 13, 13, 29, 59, 0, timezone.Berlin), 15, 0, true)
	assert.False(t, before.CanStart)
	assert.False(t, before.CanComplete)

	atStartBoundary := evaluateLifecycleAvailability(instance, time.Date(2026, 8, 13, 13, 30, 0, 0, timezone.Berlin), 15, 0, true)
	assert.True(t, atStartBoundary.CanStart)

	atEndBoundary := evaluateLifecycleAvailability(instance, time.Date(2026, 8, 13, 14, 30, 0, 0, timezone.Berlin), 15, 0, true)
	assert.False(t, atEndBoundary.CanStart)
	assert.True(t, atEndBoundary.CanComplete)
}

func TestEvaluateLifecycleAvailability_SettingsAndSpontaneous(t *testing.T) {
	t.Parallel()

	instance := &scheduleModel.ActivityInstance{
		Date:      scheduleModel.NewDate(2026, 8, 13),
		StartTime: time.Date(1, 1, 1, 13, 45, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
	}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, timezone.Berlin)
	assert.True(t, evaluateLifecycleAvailability(instance, now, 120, 0, false).CanStart)
	assert.True(t, evaluateLifecycleAvailability(instance, now, 0, 0, false).CanComplete)

	instance.IsSpontaneous = true
	got := evaluateLifecycleAvailability(instance, now, 0, 0, true)
	assert.True(t, got.CanStart)
	assert.True(t, got.CanComplete)
}

// A school may let its team complete a planned block a few minutes before its
// planned end (#3809): with a 15-minute lead a block ending 16:00 completes
// from 15:45, not before.
func TestEvaluateLifecycleAvailability_CompleteLead(t *testing.T) {
	t.Parallel()

	instance := &scheduleModel.ActivityInstance{
		Date:      scheduleModel.NewDate(2026, 10, 2),
		StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
	}
	completeAt := time.Date(2026, 10, 2, 15, 45, 0, 0, timezone.Berlin)

	before := evaluateLifecycleAvailability(instance, completeAt.Add(-time.Second), 15, 15, true)
	assert.False(t, before.CanComplete)
	assert.True(t, before.CompleteAvailableAt.Equal(completeAt))

	atBoundary := evaluateLifecycleAvailability(instance, completeAt, 15, 15, true)
	assert.True(t, atBoundary.CanComplete)
	assert.True(t, atBoundary.CanStart, "the lead to complete does not end the start window")

	withoutLead := evaluateLifecycleAvailability(instance, completeAt, 15, 0, true)
	assert.False(t, withoutLead.CanComplete)
	assert.True(t, withoutLead.CompleteAvailableAt.Equal(completeAt.Add(15*time.Minute)))

	unenforced := evaluateLifecycleAvailability(instance, completeAt.Add(-time.Hour), 15, 15, false)
	assert.True(t, unenforced.CanComplete, "without the planned-end rule the lead does not lock anything")
}
