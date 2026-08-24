package schedule_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateLifecycleAvailability_PlannedBoundaries(t *testing.T) {
	t.Parallel()

	instance := &scheduleModel.ActivityInstance{
		Date:      timezone.NewDate(2026, 8, 13),
		StartTime: time.Date(1, 1, 1, 13, 45, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
	}

	before := scheduleSvc.EvaluateLifecycleAvailability(instance, time.Date(2026, 8, 13, 13, 29, 59, 0, timezone.Berlin), 15, true)
	assert.False(t, before.CanStart)
	assert.False(t, before.CanComplete)

	atStartBoundary := scheduleSvc.EvaluateLifecycleAvailability(instance, time.Date(2026, 8, 13, 13, 30, 0, 0, timezone.Berlin), 15, true)
	assert.True(t, atStartBoundary.CanStart)

	atEndBoundary := scheduleSvc.EvaluateLifecycleAvailability(instance, time.Date(2026, 8, 13, 14, 30, 0, 0, timezone.Berlin), 15, true)
	assert.False(t, atEndBoundary.CanStart)
	assert.True(t, atEndBoundary.CanComplete)
}

func TestEvaluateLifecycleAvailability_SettingsAndSpontaneous(t *testing.T) {
	t.Parallel()

	instance := &scheduleModel.ActivityInstance{
		Date:      timezone.NewDate(2026, 8, 13),
		StartTime: time.Date(1, 1, 1, 13, 45, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
	}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, timezone.Berlin)
	assert.True(t, scheduleSvc.EvaluateLifecycleAvailability(instance, now, 120, false).CanStart)
	assert.True(t, scheduleSvc.EvaluateLifecycleAvailability(instance, now, 0, false).CanComplete)

	instance.IsSpontaneous = true
	got := scheduleSvc.EvaluateLifecycleAvailability(instance, now, 0, true)
	assert.True(t, got.CanStart)
	assert.True(t, got.CanComplete)
}
