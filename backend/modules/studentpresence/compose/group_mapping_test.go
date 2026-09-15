package compose_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGroupMappingCommandsAreIdempotentAndTransactional(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	var combinedID int64
	require.NoError(t, db.NewRaw("INSERT INTO active.combined_groups (tenant_id,start_time) VALUES (?,NOW()) RETURNING id", testpkg.Tenant(t)).Scan(ctx, &combinedID))
	for _, filter := range []studentpresence.GroupMappingFilter{{CombinedGroupID: &combinedID}, {ActiveGroupID: &group.ID}} {
		rows, listErr := module.ListGroupMappings(ctx, filter)
		require.NoError(t, listErr)
		require.NotNil(t, rows)
		require.Empty(t, rows)
	}
	_, err = module.RecordGroupMapping(ctx, 0, group.ID)
	require.Error(t, err)
	count := func() int {
		n, err := db.NewSelect().Table("active.group_mappings").Where("tenant_id = ? AND active_combined_group_id = ?", testpkg.Tenant(t), combinedID).Count(ctx)
		require.NoError(t, err)
		return n
	}
	abort := errors.New("abort mapping")
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.AddGroupToCombination(txCtx, combinedID, group.ID))
		return abort
	}), abort)
	require.Zero(t, count())
	require.NoError(t, module.AddGroupToCombination(ctx, combinedID, group.ID))
	require.NoError(t, module.AddGroupToCombination(ctx, combinedID, group.ID))
	require.Equal(t, 1, count())
	mappings, err := module.ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedID})
	require.NoError(t, err)
	require.Len(t, mappings, 1)
	require.Equal(t, group.ID, mappings[0].ActiveGroupID)
	require.Equal(t, testpkg.Tenant(t), mappings[0].TenantID)
	require.NotZero(t, mappings[0].ID)
	byGroup, err := module.ListGroupMappings(ctx, studentpresence.GroupMappingFilter{ActiveGroupID: &group.ID})
	require.NoError(t, err)
	require.Equal(t, mappings, byGroup)
	_, err = module.ListGroupMappings(context.Background(), studentpresence.GroupMappingFilter{})
	require.Error(t, err)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, otherTenant, func(otherCtx context.Context) error {
		rows, err := module.ListGroupMappings(otherCtx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedID})
		require.NoError(t, err)
		require.Empty(t, rows)
		return nil
	}))
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.RemoveGroupFromCombination(txCtx, combinedID, group.ID))
		return abort
	}), abort)
	require.Equal(t, 1, count())
	require.Error(t, module.RemoveGroupFromCombination(context.Background(), combinedID, group.ID))
	require.NoError(t, module.RemoveGroupFromCombination(ctx, combinedID, group.ID))
	require.NoError(t, module.RemoveGroupFromCombination(ctx, combinedID, group.ID))
	require.Zero(t, count())
	recorded, err := module.RecordGroupMapping(ctx, combinedID, group.ID)
	require.NoError(t, err)
	require.NotZero(t, recorded.ID)
	require.False(t, recorded.CreatedAt.IsZero())
	require.Equal(t, testpkg.Tenant(t), recorded.TenantID)
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.DeleteGroupMapping(txCtx, recorded.ID))
		return abort
	}), abort)
	require.Equal(t, 1, count())
	err = testpkg.WithinTenantContext(t, context.Background(), db, otherTenant, func(otherCtx context.Context) error {
		return module.DeleteGroupMapping(otherCtx, recorded.ID)
	})
	require.Error(t, err, "another tenant cannot delete this mapping")
	require.Equal(t, 1, count())
	require.NoError(t, module.DeleteGroupMapping(ctx, recorded.ID))
	require.Zero(t, count())
	require.Error(t, module.DeleteGroupMapping(ctx, recorded.ID), "deletion by identity reports a missing mapping")
	secondGroup := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	for _, groupID := range []int64{group.ID, secondGroup.ID} {
		_, err = module.RecordGroupMapping(ctx, combinedID, groupID)
		require.NoError(t, err)
	}
	mappings, err = module.ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedID})
	require.NoError(t, err)
	require.Len(t, mappings, 2)
	require.ElementsMatch(t, []int64{group.ID, secondGroup.ID}, []int64{mappings[0].ActiveGroupID, mappings[1].ActiveGroupID})
}
