// Hermetic tests for the Änderungsprotokoll (#1886): deviation writes append
// audit.deviation_events rows; re-plan losses are logged, successful
// reapplies are not.
package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// loadDeviationEvents returns every protocol row of one tenant, oldest first.
func loadDeviationEvents(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64) []*auditModels.DeviationEvent {
	t.Helper()
	var rows []*auditModels.DeviationEvent
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.deviation_events AS "deviation_event"`).
		Where(`"deviation_event".tenant_id = ?`, tenantID).
		Order("id ASC").
		Scan(ctx)
	require.NoError(t, err)
	return rows
}

// TestReplanWeek_LogsDroppedSnapshotWhenSlotVanishes: the deviated slot's
// schedule row is deleted before the re-plan, so the occurrence never
// regenerates — the snapshot is dropped and MUST leave a
// deviation_dropped_by_replan protocol row anchored at the slot key.
func TestReplanWeek_LogsDroppedSnapshotWhenSlotVanishes(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	seedDeviatedOccurrence(t, s, date)

	// Remove the template's only schedule slot: re-plan deletes the deviated
	// occurrence and regenerates nothing. This delete is the test's ACT step,
	// not a teardown.
	_, err := s.db.NewDelete().
		TableExpr("activities.schedules").
		Where("id = ?", s.schedule.ID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	events := loadDeviationEvents(t, s.db, s.ctx, s.tenantID)
	require.Len(t, events, 1, "exactly one drop event for the vanished slot")
	ev := events[0]
	assert.Equal(t, auditModels.DeviationEventDroppedByReplan, ev.EventType)
	require.NotNil(t, ev.ActivityGroupID)
	assert.Equal(t, s.template.ID, *ev.ActivityGroupID)
	assert.Equal(t, date, ev.OccurrenceDate)
	assert.Nil(t, ev.InstanceID, "vanished slot has no instance to point at")
	assert.Nil(t, ev.ActorAccountID)
	require.NotNil(t, ev.OldValue)
	assert.Contains(t, string(ev.OldValue), "substitutes")
}

// TestReplanWeek_SuccessfulReapplyLogsNothing: the template is unchanged, the
// deviation reattaches — per the owner's decision no protocol row is written.
func TestReplanWeek_SuccessfulReapplyLogsNothing(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.April, 20) // Mon
	s := makeScenario(t, activitiesModels.WeekdayMonday, date)
	defer s.runCleanup(t)

	seedDeviatedOccurrence(t, s, date)

	_, err := s.factory.Instance.ReplanWeek(s.ctx, date, date, &s.template.ID, nil)
	require.NoError(t, err)

	regen := listInstancesForDate(t, s.db, s.template.ID, date)
	require.Len(t, regen, 1)

	events := loadDeviationEvents(t, s.db, s.ctx, s.tenantID)
	assert.Empty(t, events, "a successful reapply is not a state change and logs nothing")
}
