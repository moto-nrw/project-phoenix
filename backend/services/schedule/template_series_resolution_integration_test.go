// Hermetic integration tests for the #2187 split-series segment resolution.
//
// The Betreuungsplan grid renders every occurrence with the template id that
// materialized it, so a click on a first-school-week block can carry the id of
// a segment that has meanwhile been capped twice. ResolveLivingTemplateSegment
// maps such an id back to the one segment of the lineage that is still
// editable; a lineage without an open segment is a genuine miss.
//
// Fixtures: makeSeriesChain builds the T → S1 → S2 shape (one fully bounded
// NON-root middle segment) on top of makeScenario. Unique tenant per test, all
// rows cleaned up by scenarioSetup.runCleanup.
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
	"github.com/uptrace/bun"
)

// seriesChain is a two-split lineage T → S1 → S2:
//
//	rootID   schedules [-∞, firstBoundary)
//	middleID schedules [firstBoundary, secondBoundary)   ← bounded NON-root
//	livingID schedules [secondBoundary, ∞)
type seriesChain struct {
	*scenarioSetup
	rootID         int64
	middleID       int64
	livingID       int64
	weekdays       []int
	firstBoundary  timezone.Date
	secondBoundary timezone.Date
}

// makeSeriesChain builds the two-split chain on the given weekdays. The
// scenario template starts with weekdays[0]; the remaining weekdays are added
// as extra schedule rows sharing the scenario's timeframe, so every segment of
// the resulting chain runs on all of them.
func makeSeriesChain(t *testing.T, weekdays []int, firstBoundary, secondBoundary timezone.Date) *seriesChain {
	t.Helper()
	require.NotEmpty(t, weekdays)
	require.True(t, firstBoundary.Before(secondBoundary), "chain boundaries must be ordered")

	s := makeScenario(t, weekdays[0], firstBoundary)
	for _, weekday := range weekdays[1:] {
		row := &activitiesModels.Schedule{
			Weekday:         weekday,
			TimeframeID:     &s.timeframe.ID,
			ActivityGroupID: s.template.ID,
			WeekPattern:     0,
		}
		row.SetTenantID(s.tenantID)
		_, err := s.db.NewInsert().Model(row).ModelTableExpr(`activities.schedules`).Exec(s.ctx)
		require.NoError(t, err)
		s.registerCleanup("activities.schedules", row.ID)
	}

	middleInput := baseSplitInput(s, firstBoundary, fmt.Sprintf("Chain-Middle-%d", time.Now().UnixNano()))
	middleInput.Weekdays = weekdays
	middle, err := s.factory.TemplateSplit.Split(s.ctx, middleInput)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, middle.NewTemplateID)

	livingInput := baseSplitInput(s, secondBoundary, fmt.Sprintf("Chain-Living-%d", time.Now().UnixNano()))
	livingInput.TemplateID = middle.NewTemplateID
	livingInput.Weekdays = weekdays
	living, err := s.factory.TemplateSplit.Split(s.ctx, livingInput)
	require.NoError(t, err)
	registerSuccessorCleanup(t, s, living.NewTemplateID)

	chain := &seriesChain{
		scenarioSetup:  s,
		rootID:         s.template.ID,
		middleID:       middle.NewTemplateID,
		livingID:       living.NewTemplateID,
		weekdays:       weekdays,
		firstBoundary:  firstBoundary,
		secondBoundary: secondBoundary,
	}
	chain.registerSweep(t)
	chain.assertShape(t)
	return chain
}

// registerSweep deletes everything still attached to the chain's groups at
// teardown. The tests below create roster and instance rows the per-id
// registrations cannot know about, and the group rows (registered by
// registerSuccessorCleanup) can only be deleted once their children are gone.
func (c *seriesChain) registerSweep(t *testing.T) {
	t.Helper()
	ids := []int64{c.rootID, c.middleID, c.livingID}
	c.extraCleanups = append([]func(){func() {
		exec := func(table, where string, args ...any) {
			_, err := c.db.NewDelete().TableExpr(table).Where(where, args...).Exec(c.ctx)
			require.NoError(t, err)
		}
		exec("schedule.instance_students",
			`instance_id IN (SELECT id FROM schedule.activity_instances WHERE activity_group_id IN (?))`, bun.List(ids))
		exec("schedule.instance_staff",
			`instance_id IN (SELECT id FROM schedule.activity_instances WHERE activity_group_id IN (?))`, bun.List(ids))
		exec("schedule.activity_instances", `activity_group_id IN (?)`, bun.List(ids))
		exec("activities.student_enrollments", `activity_group_id IN (?)`, bun.List(ids))
		exec("activities.supervisors", `group_id IN (?)`, bun.List(ids))
		exec("activities.schedules", `activity_group_id IN (?)`, bun.List(ids))
	}}, c.extraCleanups...)
}

// assertShape pins the envelope the roster and cascade tests rely on.
func (c *seriesChain) assertShape(t *testing.T) {
	t.Helper()
	for _, row := range loadSplitSchedules(t, c.scenarioSetup, c.rootID) {
		assert.Nil(t, row.ValidFrom, "root segment keeps its open start")
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, c.firstBoundary, *row.ValidUntil)
	}
	middle := loadSplitSchedules(t, c.scenarioSetup, c.middleID)
	require.Len(t, middle, len(c.weekdays))
	for _, row := range middle {
		require.NotNil(t, row.ValidFrom)
		require.NotNil(t, row.ValidUntil)
		assert.Equal(t, c.firstBoundary, *row.ValidFrom)
		assert.Equal(t, c.secondBoundary, *row.ValidUntil, "middle segment must be fully bounded")
	}
	for _, row := range loadSplitSchedules(t, c.scenarioSetup, c.livingID) {
		require.NotNil(t, row.ValidFrom)
		assert.Equal(t, c.secondBoundary, *row.ValidFrom)
		assert.Nil(t, row.ValidUntil, "living segment stays open")
	}
}

func TestResolveLivingTemplateSegment_ResolvesCappedPredecessor(t *testing.T) {
	chain := makeSeriesChain(t, []int{activitiesModels.WeekdayMonday}, futureMonday(1), futureMonday(3))
	defer chain.runCleanup(t)

	resolved, changed, err := chain.factory.TimetableData.ResolveLivingTemplateSegment(chain.ctx, chain.middleID)
	require.NoError(t, err)
	assert.Equal(t, chain.livingID, resolved, "a bounded middle segment resolves to the living successor")
	assert.True(t, changed)

	resolved, changed, err = chain.factory.TimetableData.ResolveLivingTemplateSegment(chain.ctx, chain.rootID)
	require.NoError(t, err)
	assert.Equal(t, chain.livingID, resolved, "the series root resolves to the living successor too")
	assert.True(t, changed)
}

func TestResolveLivingTemplateSegment_LeavesOpenTemplateAlone(t *testing.T) {
	chain := makeSeriesChain(t, []int{activitiesModels.WeekdayMonday}, futureMonday(1), futureMonday(3))
	defer chain.runCleanup(t)

	resolved, changed, err := chain.factory.TimetableData.ResolveLivingTemplateSegment(chain.ctx, chain.livingID)
	require.NoError(t, err)
	assert.Equal(t, chain.livingID, resolved)
	assert.False(t, changed, "the living segment must not report a resolution")
}

func TestResolveLivingTemplateSegment_ErrorsOnFullyEndedSeries(t *testing.T) {
	chain := makeSeriesChain(t, []int{activitiesModels.WeekdayMonday}, futureMonday(1), futureMonday(3))
	defer chain.runCleanup(t)

	t.Run("unknown template id", func(t *testing.T) {
		// Far above every id this tenant's fixtures produced — an id that
		// resolves to no lineage at all.
		unknownID := chain.livingID + 1_000_000
		_, _, err := chain.factory.TimetableData.ResolveLivingTemplateSegment(chain.ctx, unknownID)
		assert.ErrorIs(t, err, scheduleSvc.ErrTemplateSeriesFullyEnded)
	})

	t.Run("every segment ended", func(t *testing.T) {
		_, err := chain.factory.TemplateSplit.EndFromDate(chain.ctx, scheduleSvc.TemplateEndInput{
			TemplateID:    chain.livingID,
			EffectiveDate: chain.secondBoundary,
		})
		require.NoError(t, err)

		_, _, err = chain.factory.TimetableData.ResolveLivingTemplateSegment(chain.ctx, chain.middleID)
		assert.ErrorIs(t, err, scheduleSvc.ErrTemplateSeriesFullyEnded,
			"a lineage without an open segment has nothing left to edit")
	})
}
