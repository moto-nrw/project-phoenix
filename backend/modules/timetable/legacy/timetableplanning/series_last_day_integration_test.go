package timetableplanning_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// A series may end before its planning period (#3594), e.g. a holiday-care
// week. makeScenario's factory clock stands on Monday 2026-08-24.

func setSeriesLastDay(t *testing.T, s *scenarioSetup, lastDay *timezone.Date) {
	t.Helper()
	var value any
	if lastDay != nil {
		value = lastDay.String()
	}
	_, err := s.db.NewUpdate().Table("activities.groups").
		Set("series_last_day = ?", value).
		Where("id = ?", s.template.ID).Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
}

func TestMaterializeForTenant_StopsAfterTheSeriesLastDay(t *testing.T) {
	t.Parallel()

	first := timezone.NewDate(2026, time.August, 31)
	lastDay := first.AddDays(7)
	after := first.AddDays(14)
	s := makeScenario(t, activitiesModels.WeekdayMonday, first)
	setSeriesLastDay(t, s, &lastDay)

	result, err := s.factory.Materialization.MaterializeForTenant(s.ctx, first, after, timetable.MaterializationSourceManual)
	require.NoError(t, err)

	assert.Equal(t, 2, result.InstancesCreated, "the first Monday and the last day itself")
	assert.Equal(t, 1, result.CandidatesSkippedEnded)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, lastDay), 1, "the last day is inclusive")
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, after))
}

// Shortening a series in the template update removes the later planned
// occurrences itself, whatever window a later re-plan covers. The module's
// "today" runs on the real clock, so the series lies far in the future.
func TestUpdateTemplateFields_DropsOccurrencesAfterTheNewLastDay(t *testing.T) {
	t.Parallel()

	first := timezone.NewDate(2099, time.December, 7) // a Monday
	lastDay := first.AddDays(7)
	after := first.AddDays(14)
	s := makeScenario(t, activitiesModels.WeekdayMonday, first)
	_, err := s.factory.Materialization.MaterializeForTenant(s.ctx, first, after, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	require.Len(t, listInstancesForDate(t, s.db, s.template.ID, after), 1)

	groups := repositories.NewFactory(s.db, repositories.NewUnobservedTimetableDependencies(s.db)).ActivityGroup
	newEnd := lastDay.String()
	updated, err := groups.UpdateTemplateFields(s.ctx, s.template.ID, activitiesModels.TemplateFieldsUpdate{
		Name: s.template.Name, Type: s.template.Type, CategoryID: s.template.CategoryID,
		RoomID: *s.template.PlannedRoomID, MaxParticipants: s.template.MaxParticipants,
		CalendarPeriodID: s.template.CalendarPeriodID, TargetGroupType: s.template.TargetGroupType,
		SeriesLastDay: &newEnd, SeriesLastDayProvided: true,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, updated)

	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, first), 1)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, lastDay), 1, "the new last day stays")
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, after), "the occurrence after the new last day is gone")
}

// Shortening a series removes its later planned occurrences through the
// existing re-plan; lifting the end brings them back.
func TestReplanWeek_FollowsTheSeriesLastDay(t *testing.T) {
	t.Parallel()

	first := timezone.NewDate(2026, time.August, 31)
	lastDay := first.AddDays(7)
	after := first.AddDays(14)
	s := makeScenario(t, activitiesModels.WeekdayMonday, first)
	_, err := s.factory.Materialization.MaterializeForTenant(s.ctx, first, after, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	require.Len(t, listInstancesForDate(t, s.db, s.template.ID, after), 1)

	templateID := s.template.ID
	setSeriesLastDay(t, s, &lastDay)
	_, err = s.factory.Instance.ReplanWeek(s.ctx, first, after, &templateID, nil)
	require.NoError(t, err)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, lastDay), 1)
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, after), "shortened series drops the later Monday")

	setSeriesLastDay(t, s, nil)
	_, err = s.factory.Instance.ReplanWeek(s.ctx, first, after, &templateID, nil)
	require.NoError(t, err)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, after), 1, "without an end the series runs on")
}
