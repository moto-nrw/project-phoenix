// Hermetic integration tests for the #2187 end cascade: "diesen und alle
// folgenden löschen" chosen on an occurrence that still belongs to an
// already-capped predecessor segment must also end THAT segment from the
// clicked date, instead of leaving live occurrences behind on a template the
// grid keeps rendering.
package schedule_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateEnd_CascadesToCappedPredecessors(t *testing.T) {
	t.Parallel()

	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	require.Len(t, listInstancesForDate(t, c.db, c.middleID, c.earlier), 1)
	require.Len(t, listInstancesForDate(t, c.db, c.middleID, c.anchor), 1)

	result, err := c.factory.TemplateSplit.EndFromDate(c.ctx, scheduleSvc.TemplateEndInput{
		TemplateID:    c.livingID,
		EffectiveDate: c.anchor,
	})
	require.NoError(t, err)
	assert.Equal(t, c.secondBoundary, result.EffectiveDate,
		"the living segment itself is clamped to its own start")

	for _, row := range loadSplitSchedules(t, c.scenarioSetup, c.middleID) {
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, c.anchor, *row.ValidUntil,
			"the predecessor segment must end at the clicked date")
	}
	for _, row := range loadSplitSchedules(t, c.scenarioSetup, c.livingID) {
		require.NotNil(t, row.ValidFrom)
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, c.secondBoundary, *row.ValidFrom)
		assert.Equal(t, c.secondBoundary, *row.ValidUntil, "the living segment is fully capped")
	}
	for _, row := range loadSplitSchedules(t, c.scenarioSetup, c.rootID) {
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, c.firstBoundary, *row.ValidUntil,
			"a segment that already ended before the clicked date is left alone")
	}

	assert.Empty(t, listInstancesForDate(t, c.db, c.middleID, c.anchor),
		"planned predecessor occurrences from the clicked date are deleted")
	assert.Len(t, listInstancesForDate(t, c.db, c.middleID, c.earlier), 1,
		"occurrences before the clicked date stay as history")
}

// TestTemplateEnd_FromCappedPredecessorAlsoEndsLivingSuccessor covers the path
// the client actually takes: the grid renders an occurrence with the id of the
// segment that materialized it, so "diesen und alle folgenden löschen" on a
// first-week occurrence arrives on the CAPPED predecessor. The living
// successor carries every later occurrence and must be ended along with it.
func TestTemplateEnd_FromCappedPredecessorAlsoEndsLivingSuccessor(t *testing.T) {
	t.Parallel()

	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	livingDay := c.secondBoundary
	_, err := c.svc.MaterializeForTenant(
		c.ctx, c.secondBoundary, c.secondBoundary.AddDays(6), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	registerSplitInstancesForCleanup(t, c.scenarioSetup,
		[]int64{c.livingID}, []timezone.Date{livingDay})
	require.Len(t, listInstancesForDate(t, c.db, c.livingID, livingDay), 1)

	result, err := c.factory.TemplateSplit.EndFromDate(c.ctx, scheduleSvc.TemplateEndInput{
		TemplateID:    c.middleID,
		EffectiveDate: c.anchor,
	})
	require.NoError(t, err)
	assert.Equal(t, c.middleID, result.TemplateID)

	for _, row := range loadSplitSchedules(t, c.scenarioSetup, c.middleID) {
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, c.anchor, *row.ValidUntil,
			"the clicked segment ends at the clicked date")
	}
	for _, row := range loadSplitSchedules(t, c.scenarioSetup, c.livingID) {
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, c.secondBoundary, *row.ValidUntil,
			"and the living successor is ended too")
	}

	assert.Empty(t, listInstancesForDate(t, c.db, c.middleID, c.anchor))
	assert.Empty(t, listInstancesForDate(t, c.db, c.livingID, livingDay),
		"no live occurrence may survive on the successor")
	assert.Len(t, listInstancesForDate(t, c.db, c.middleID, c.earlier), 1,
		"occurrences before the clicked date stay as history")
}

func TestTemplateEnd_CascadeLeavesUnrelatedSeriesAlone(t *testing.T) {
	t.Parallel()

	c := makeRosterChain(t, []int{activitiesModels.WeekdayMonday})
	defer c.runCleanup(t)

	// A second, independent template of the same tenant, running on the same
	// weekday and materialized over the same window.
	other, err := c.factory.TimetableData.CreateTemplate(c.ctx, scheduleSvc.CreateTemplateInput{
		Name:            fmt.Sprintf("Unrelated-%d", time.Now().UnixNano()),
		Type:            activitiesModels.GroupTypeActivity,
		Weekdays:        []int{activitiesModels.WeekdayMonday},
		StartTime:       time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC),
		RoomID:          c.roomID,
		CategoryID:      c.categoryID,
		MaxParticipants: 20,
		RosterValidFrom: c.firstBoundary,
		GradeLevelMax:   4,
		StudentIDs:      []int64{c.students[0]},
		StaffIDs:        []int64{c.staffID},
		PrimaryStaffID:  &c.staffID,
	})
	require.NoError(t, err)
	registerSuccessorCleanup(t, c.scenarioSetup, other.TemplateID)

	_, err = c.svc.MaterializeForTenant(
		c.ctx, c.firstBoundary, c.secondBoundary.AddDays(-1), scheduleSvc.MaterializationSourceManual)
	require.NoError(t, err)
	registerSplitInstancesForCleanup(t, c.scenarioSetup,
		[]int64{other.TemplateID}, []timezone.Date{c.earlier, c.anchor})
	require.Len(t, listInstancesForDate(t, c.db, other.TemplateID, c.anchor), 1)

	_, err = c.factory.TemplateSplit.EndFromDate(c.ctx, scheduleSvc.TemplateEndInput{
		TemplateID:    c.livingID,
		EffectiveDate: c.anchor,
	})
	require.NoError(t, err)

	for _, row := range loadSplitSchedules(t, c.scenarioSetup, other.TemplateID) {
		assert.Nil(t, row.ValidUntil, "an unrelated template must keep its open recurrence")
	}
	assert.Len(t, listInstancesForDate(t, c.db, other.TemplateID, c.anchor), 1,
		"an unrelated template keeps its planned occurrences")
}
