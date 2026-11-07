package httpintegration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestMaterializationRosterReadFailurePersistsNothing injects a real SQL
// failure into a roster read of the second template of a run (#5814): the
// first template has no enrollments, the second needs the children's names.
// Whether the run reads the rosters per template or once up front, the run
// fails with the permission error and no occurrence survives, neither in its
// own transaction nor in an ambient one whose caller swallows the error like
// the best-effort materialization after a template create.
func TestMaterializationRosterReadFailurePersistsNothing(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	from := calendar.NewDate(2026, time.April, 20)
	s := makeScenario(t, activityContract.WeekdayMonday, from)
	defer s.runCleanup(t)
	s.ctx = testpkg.WithTestTenantRuntime(t, s.ctx)

	// "Aa" sorts before the scenario's template, so it materializes first.
	first := &activityContract.Group{
		Name: fmt.Sprintf("Aa roster fault %d", time.Now().UnixNano()), CategoryID: s.categoryID,
		MaxParticipants: 20, IsOpen: true, PlannedRoomID: &s.roomID, IsTemplate: true,
	}
	first.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(activityGroupFixture(first)).ModelTableExpr(`activities.groups AS "group"`).Exec(s.ctx)
	require.NoError(t, err)
	schedule := &activityContract.Schedule{ActivityGroupID: first.ID, TimeframeID: &s.timeframe.ID, Weekday: activityContract.WeekdayMonday}
	schedule.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(activityScheduleFixture(schedule)).ModelTableExpr(`activities.schedules`).Exec(s.ctx)
	require.NoError(t, err)

	// Relation ACL belongs only to this disposable clone, never a global role.
	_, err = s.db.NewRaw(`REVOKE SELECT ON users.persons FROM phoenix_tenant`).Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, grantErr := s.db.NewRaw(`GRANT SELECT ON users.persons TO phoenix_tenant`).Exec(context.Background())
		require.NoError(t, grantErr)
	})

	result, err := s.svc.MaterializeForTenant(s.ctx, from, from, timetable.MaterializationSourceManual)
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied for table persons")
	t.Logf("FAULT own transaction: err=%v created=%d", err, createdOf(result))
	require.Empty(t, listInstancesForDate(t, s.db, first.ID, from))
	require.Empty(t, listInstancesForDate(t, s.db, s.template.ID, from))

	var swallowed error
	commitErr := tenant.WithinCurrentTenant(s.ctx, func(txCtx context.Context) error {
		result, swallowed = s.svc.MaterializeForTenant(txCtx, from, from, timetable.MaterializationSourceManual)
		return nil
	})
	require.Error(t, swallowed)
	require.Contains(t, swallowed.Error(), "permission denied for table persons")
	t.Logf("FAULT ambient transaction: err=%v created=%d commit=%v", swallowed, createdOf(result), commitErr)
	require.Empty(t, listInstancesForDate(t, s.db, first.ID, from), "the aborted transaction keeps no occurrence")
	require.Empty(t, listInstancesForDate(t, s.db, s.template.ID, from))
}

func createdOf(result *timetable.MaterializationResult) int {
	if result == nil {
		return -1
	}
	return result.InstancesCreated
}
