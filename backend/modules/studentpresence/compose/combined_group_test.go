package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestCombinedGroupsAreTenantScopedAndEndTransactionally(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	start := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	var id int64
	require.NoError(t, db.NewRaw("INSERT INTO active.combined_groups (tenant_id,start_time) VALUES (?,?) RETURNING id", testpkg.Tenant(t), start).Scan(ctx, &id))
	rows, err := module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, id, rows[0].ID)
	require.Equal(t, testpkg.Tenant(t), rows[0].TenantID)
	require.True(t, start.Equal(rows[0].StartTime))
	abort := errors.New("abort combination end")
	end := start.Add(time.Hour)
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.EndCombination(txCtx, id, end))
		return abort
	}), abort)
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, otherTenant, func(otherCtx context.Context) error {
		rows, err := module.ListCombinedGroups(otherCtx, studentpresence.CombinedGroupFilter{})
		require.NoError(t, err)
		require.Empty(t, rows)
		require.Error(t, module.EndCombination(otherCtx, id, end))
		return nil
	}))
	require.Error(t, module.EndCombination(context.Background(), id, end))
	require.NoError(t, module.EndCombination(ctx, id, end))
	require.Error(t, module.EndCombination(ctx, id, end))
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{OpenOnly: true})
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{From: &end, Until: &end})
	require.NoError(t, err)
	require.Len(t, rows, 1, "range boundaries are inclusive")
	after := end.Add(time.Second)
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{From: &after})
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = module.ListCombinedGroups(context.Background(), studentpresence.CombinedGroupFilter{})
	require.Error(t, err)
}

func TestCombinedGroupWritesPreserveIdentityAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	start := time.Now().UTC().Truncate(time.Second)
	value, err := module.RecordCombination(ctx, start, nil)
	require.NoError(t, err)
	require.NotZero(t, value.ID)
	require.False(t, value.CreatedAt.IsZero())
	require.Equal(t, testpkg.Tenant(t), value.TenantID)
	end := start.Add(time.Hour)
	abort := errors.New("abort combined group write")
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.ReviseCombination(txCtx, value.ID, start, &end)
		require.NoError(t, err)
		return abort
	}), abort)
	found, err := module.GetCombinedGroup(ctx, value.ID)
	require.NoError(t, err)
	require.Nil(t, found.EndTime)
	revised, err := module.ReviseCombination(ctx, value.ID, start, &end)
	require.NoError(t, err)
	require.True(t, revised.CreatedAt.Equal(value.CreatedAt))
	require.True(t, revised.EndTime.Equal(end))
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, otherTenant, func(otherCtx context.Context) error {
		_, err := module.GetCombinedGroup(otherCtx, value.ID)
		require.ErrorIs(t, err, studentpresence.ErrCombinedGroupNotFound)
		_, err = module.ReviseCombination(otherCtx, value.ID, start, nil)
		require.Error(t, err)
		require.NoError(t, module.DeleteCombination(otherCtx, value.ID))
		return nil
	}))
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.DeleteCombination(txCtx, value.ID))
		return abort
	}), abort)
	found, err = module.GetCombinedGroup(ctx, value.ID)
	require.NoError(t, err)
	require.NotNil(t, found.EndTime)
	require.Error(t, module.DeleteCombination(context.Background(), value.ID))
	require.NoError(t, module.DeleteCombination(ctx, value.ID))
	require.NoError(t, module.DeleteCombination(ctx, value.ID))
	_, err = module.GetCombinedGroup(ctx, value.ID)
	require.ErrorIs(t, err, studentpresence.ErrCombinedGroupNotFound)
	_, err = module.RecordCombination(ctx, time.Time{}, nil)
	require.Error(t, err)
	_, err = module.RecordCombination(ctx, end, &start)
	require.Error(t, err)
}

func TestCombinedGroupListFiltersActivityAndPagination(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	past := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	future := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
	open, err := module.RecordCombination(ctx, past, nil)
	require.NoError(t, err)
	scheduled, err := module.RecordCombination(ctx, past, &future)
	require.NoError(t, err)
	ended, err := module.RecordCombination(ctx, past, &past)
	require.NoError(t, err)
	all, err := module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{})
	require.NoError(t, err)
	require.Len(t, all, 3)
	openRows, err := module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, openRows, 1)
	require.Equal(t, open.ID, openRows[0].ID)
	require.Nil(t, openRows[0].EndTime)
	active := true
	rows, err := module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{Active: &active})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, open.ID, rows[0].ID)
	require.Equal(t, scheduled.ID, rows[1].ID)
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{Active: &active, ID: &scheduled.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, scheduled.ID, rows[0].ID)
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{Active: &active, Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, scheduled.ID, rows[0].ID)
	active = false
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{Active: &active, ID: &ended.ID})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, ended.ID, rows[0].ID)
	rows, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{Active: &active, ID: &scheduled.ID})
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = module.ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{Limit: -1})
	require.Error(t, err)
}
