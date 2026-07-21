package display

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

func TestBuildActivitiesExposeStableStringIDs(t *testing.T) {
	templateID := int64(41)
	template := &activitiesModels.Group{Name: "Schach"}
	template.ID = templateID
	runningGroup := &activeModels.Group{
		StartTime: time.Now().Add(-time.Hour),
		GroupID:   &templateID,
		RoomID:    7,
	}
	runningGroup.ID = 101
	running := buildRunningActivities(
		[]*activeModels.Group{runningGroup},
		nil,
		[]*activitiesModels.Group{template},
		nil,
		map[int64]string{7: "Raum 1"},
	)
	require.Len(t, running, 1)
	assert.Equal(t, "101", running[0].ID)

	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, timezone.Berlin)
	first := &scheduleModels.ActivityInstance{
		Title:     "Schach",
		StartTime: now.Add(time.Hour),
		RoomID:    7,
		Status:    scheduleModels.InstanceStatusPlanned,
	}
	first.ID = 201
	second := &scheduleModels.ActivityInstance{
		Title:     first.Title,
		StartTime: first.StartTime,
		RoomID:    first.RoomID,
		Status:    first.Status,
	}
	second.ID = 202

	upcoming := buildUpcomingActivities(
		[]*scheduleModels.ActivityInstance{first, second},
		nil,
		map[int64]string{7: "Raum 1"},
		now,
	)
	require.Len(t, upcoming, 2)
	assert.Equal(t, []string{"201", "202"}, []string{upcoming[0].ID, upcoming[1].ID})
}
