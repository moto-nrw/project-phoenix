package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildActivitiesExposeStableStringIDs(t *testing.T) {
	t.Parallel()

	templateID := int64(41)
	category := "Sport"
	template := domain.ActivityTemplate{ID: templateID, Name: "Schach", CategoryName: &category}
	running := buildRunningActivities(
		[]domain.ActiveSession{{ID: 101, RoomID: 7, TemplateID: &templateID, Running: true}},
		nil,
		[]domain.ActivityTemplate{template},
		nil,
		map[int64]string{7: "Raum 1"},
	)
	require.Len(t, running, 1)
	assert.Equal(t, "101", running[0].ID)
	assert.Equal(t, "Sport", running[0].Category)

	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	start := now.Add(time.Hour)
	upcoming := buildUpcomingActivities(
		[]domain.PlannedActivity{
			{ID: 201, Title: "Schach", RoomID: 7, ActivityGroupID: &templateID, Planned: true, StartWallClock: start},
			{ID: 202, Title: "Schach", RoomID: 7, ActivityGroupID: &templateID, Planned: true, StartWallClock: start},
		},
		[]domain.ActivityTemplate{template},
		map[int64]string{7: "Raum 1"},
		now,
	)
	require.Len(t, upcoming, 2)
	assert.Equal(t, []string{"201", "202"}, []string{upcoming[0].ID, upcoming[1].ID})
	assert.Equal(t, []string{"Sport", "Sport"}, []string{upcoming[0].Category, upcoming[1].Category})
}

// A session whose template vanished must disappear from the panel rather than
// render as a spontaneous activity.
func TestBuildRunningActivitiesDropsSessionsWithoutTemplate(t *testing.T) {
	t.Parallel()

	missing := int64(99)
	running := buildRunningActivities(
		[]domain.ActiveSession{{ID: 101, RoomID: 7, TemplateID: &missing, Running: true}},
		nil,
		nil,
		nil,
		map[int64]string{7: "Raum 1"},
	)
	assert.Empty(t, running)
}

// A spontaneous session borrows today's linked instance title and otherwise
// falls back to the generic German labels the screen renders.
func TestBuildRunningActivitiesUsesInstanceTitleForSpontaneousSessions(t *testing.T) {
	t.Parallel()

	sessionID := int64(101)
	running := buildRunningActivities(
		[]domain.ActiveSession{{ID: sessionID, RoomID: 7, Running: true}},
		[]domain.PresentVisit{{ActiveGroupID: sessionID, StudentID: 1}},
		nil,
		[]domain.PlannedActivity{{ID: 201, Title: "Vorlesen", ActiveGroupID: &sessionID}},
		map[int64]string{7: "Raum 1"},
	)
	require.Len(t, running, 1)
	assert.Equal(t, "Vorlesen", running[0].Name)
	assert.Equal(t, fallbackCategoryName, running[0].Category)
	assert.Equal(t, 1, running[0].Participants)
}

// Instances that already started today are past, not upcoming.
func TestBuildUpcomingActivitiesSkipsStartedInstances(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	upcoming := buildUpcomingActivities(
		[]domain.PlannedActivity{
			{ID: 201, Title: "Vorbei", RoomID: 7, Planned: true, StartWallClock: now.Add(-time.Minute)},
			{ID: 202, Title: "Jetzt", RoomID: 7, Planned: true, StartWallClock: now},
			{ID: 203, Title: "Abgesagt", RoomID: 7, Planned: false, StartWallClock: now.Add(time.Hour)},
		},
		nil,
		map[int64]string{7: "Raum 1"},
		now,
	)
	require.Len(t, upcoming, 1)
	assert.Equal(t, "202", upcoming[0].ID)
}
