package timetable_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
)

func TestCanReopenInstance(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	until := now.Add(5 * time.Minute)
	completedBy := int64(42)

	base := timetable.ScheduledInstance{
		Status:                timetable.InstanceStatusCompleted,
		CompletedBy:           &completedBy,
		ReopenUntil:           &until,
		HasCompletionSnapshot: true,
	}

	assert.True(t, timetable.CanReopenInstance(base, 42, false, now))
	assert.True(t, timetable.CanReopenInstance(base, 41, true, now))
	assert.False(t, timetable.CanReopenInstance(base, 40, false, now))
	assert.False(t, timetable.CanReopenInstance(base, 42, false, until.Add(time.Second)))
	assert.True(t, timetable.CanReopenAsActor(true, base.CompletedBy, 42, false))
	assert.False(t, timetable.CanReopenAsActor(true, base.CompletedBy, 40, false))
	assert.False(t, timetable.CanReopenAsActor(false, base.CompletedBy, 42, true))
	completedAt := now
	base.CompletedAt = &completedAt
	changed := timetable.ScheduledParticipant{UpdatedAt: now.Add(time.Minute)}
	assert.False(t, timetable.AttendanceUnchangedSinceCompletion(base, []timetable.ScheduledParticipant{changed}))
	unchanged := timetable.ScheduledParticipant{UpdatedAt: now.Add(-time.Minute)}
	assert.True(t, timetable.AttendanceUnchangedSinceCompletion(base, []timetable.ScheduledParticipant{unchanged}))
	assert.False(t, timetable.CanReopenInstance(timetable.ScheduledInstance{
		Status:      timetable.InstanceStatusCompleted,
		CompletedBy: &completedBy,
		ReopenUntil: &until,
	}, 42, true, now))
	assert.False(t, timetable.CanReopenInstance(timetable.ScheduledInstance{
		Status:                timetable.InstanceStatusActive,
		CompletedBy:           &completedBy,
		ReopenUntil:           &until,
		HasCompletionSnapshot: true,
	}, 42, true, now))
}
