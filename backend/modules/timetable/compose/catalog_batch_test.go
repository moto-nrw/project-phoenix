package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogBatchQueryBudgetPreservesTenantScopeAndArchivedActivities(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	_, service := api.NewActiveTestTimetable(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	catalog := timetableCompose.NewCatalog(service)
	ctx := testpkg.Ctx(t)
	active := testpkg.CreateTestActivityGroup(t, db, "Batch active")
	archived := testpkg.CreateTestActivityGroup(t, db, "Batch archived")
	at := time.Now().Add(-time.Hour)
	archived.ArchivedAt = &at
	_, err := db.NewUpdate().Model(archived).Column("archived_at").WherePK().Exec(ctx)
	require.NoError(t, err)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestActivityGroupForTenant(t, db, otherTenant, "Batch foreign")
	counter := testpkg.CaptureQueries(t, db)
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		singles := make(map[int64]*timetable.Group)
		for _, id := range []int64{active.ID, archived.ID} {
			group, findErr := catalog.FindCatalogGroup(txCtx, id)
			require.NoError(t, findErr)
			singles[id] = group
		}
		counter.Reset()
		groups, findErr := catalog.FindCatalogGroups(txCtx, []int64{archived.ID, foreign.ID, active.ID, archived.ID})
		require.NoError(t, findErr)
		querybudgets.AssertQueryBudget(t, "modules.timetable.catalog_groups.by_ids", counter.Queries())
		require.Len(t, groups, 2)
		for _, group := range groups {
			single := singles[group.ID]
			require.NotNil(t, single)
			assert.Equal(t, single.Name, group.Name)
			assert.Equal(t, single.MaxParticipants, group.MaxParticipants)
			assert.Equal(t, single.PlannedRoomID, group.PlannedRoomID)
			assert.Equal(t, single.IsSystem, group.IsSystem)
			assert.Equal(t, single.IsOpen, group.IsOpen)
			assert.Equal(t, single.ArchivedAt, group.ArchivedAt)
		}
		counter.Reset()
		empty, findErr := catalog.FindCatalogGroups(txCtx, nil)
		require.NoError(t, findErr)
		assert.Empty(t, empty)
		assert.Empty(t, counter.Queries(), "an empty ID set must not list the entire catalog")
		missing, findErr := catalog.FindCatalogGroups(txCtx, []int64{foreign.ID})
		require.NoError(t, findErr)
		assert.Empty(t, missing)
		canceled, cancel := context.WithCancel(txCtx)
		cancel()
		_, findErr = catalog.FindCatalogGroups(canceled, []int64{active.ID})
		require.Error(t, findErr)
		return nil
	}))
}
