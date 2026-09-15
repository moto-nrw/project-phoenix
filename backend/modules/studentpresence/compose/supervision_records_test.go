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

func TestSupervisionRecordsPreserveTenantIdentityAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	staff := testpkg.CreateTestStaff(t, db, "Native", "Supervisor")
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	input := studentpresence.GroupSupervision{GroupID: group.ID, StaffID: staff.ID, Role: "supervisor", StartDate: "2026-08-24"}
	abort := errors.New("rollback supervision")
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.RecordSupervision(txCtx, input)
		require.NoError(t, err)
		return abort
	}), abort)
	rows, err := module.ListGroupSupervisions(ctx, group.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
	row, err := module.RecordSupervision(ctx, input)
	require.NoError(t, err)
	require.Positive(t, row.ID)
	require.Equal(t, testpkg.Tenant(t), row.TenantID)
	require.False(t, row.CreatedAt.IsZero())
	require.False(t, row.UpdatedAt.IsZero())
	input = row
	input.Role = "lead"
	input.CreatedAt = time.Time{}
	input.TenantID = 0
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		changed, err := module.ReviseSupervision(txCtx, input)
		require.NoError(t, err)
		require.Equal(t, row.CreatedAt, changed.CreatedAt)
		require.Equal(t, row.TenantID, changed.TenantID)
		return abort
	}), abort)
	rows, err = module.ListGroupSupervisions(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, row, rows[0])
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.ErrorIs(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		_, err := module.ReviseSupervision(otherCtx, input)
		return err
	}), studentpresence.ErrSupervisionNotFound)
	_, err = module.RecordSupervision(context.Background(), input)
	require.Error(t, err)
	changed, err := module.ReviseSupervision(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "lead", changed.Role)
	require.Equal(t, row.CreatedAt, changed.CreatedAt)
	bad := input
	bad.Role = ""
	_, err = module.ReviseSupervision(ctx, bad)
	require.Error(t, err)
	invalidEnd := "2026-08-23"
	bad = input
	bad.EndDate = &invalidEnd
	_, err = module.ReviseSupervision(ctx, bad)
	require.Error(t, err)
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.RemoveSupervision(txCtx, row.ID))
		return abort
	}), abort)
	rows, err = module.ListGroupSupervisions(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		return module.RemoveSupervision(otherCtx, row.ID)
	}))
	require.NoError(t, module.RemoveSupervision(ctx, row.ID))
	require.NoError(t, module.RemoveSupervision(ctx, row.ID))
	rows, err = module.ListGroupSupervisions(ctx, group.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestSupervisionEndWritesPreserveFieldsAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	staff := testpkg.CreateTestStaff(t, db, "End", "Projection")
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	row, err := module.RecordSupervision(ctx, studentpresence.GroupSupervision{
		GroupID: group.ID, StaffID: staff.ID, Role: "lead", StartDate: "2026-08-24",
	})
	require.NoError(t, err)
	read := func() studentpresence.GroupSupervision {
		rows, err := module.ListGroupSupervisions(ctx, group.ID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		return rows[0]
	}
	abort := errors.New("rollback end-date write")
	at := time.Now().UTC().Truncate(time.Second)
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		count, err := module.SetSupervisionEnd(txCtx, row.ID, row.StartDate, at)
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
		return abort
	}), abort)
	require.Equal(t, row, read())
	require.ErrorIs(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		count, err := module.EndOpenGroupSupervisions(txCtx, group.ID, staff.ID, row.StartDate)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		count, err = module.EndOpenGroupSupervisions(txCtx, group.ID, staff.ID, row.StartDate)
		require.NoError(t, err)
		require.Zero(t, count)
		return abort
	}), abort)
	require.Equal(t, row, read())
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, other, func(otherCtx context.Context) error {
		count, err := module.SetSupervisionEnd(otherCtx, row.ID, row.StartDate, at)
		require.NoError(t, err)
		require.Zero(t, count)
		ended, err := module.EndOpenGroupSupervisions(otherCtx, group.ID, staff.ID, row.StartDate)
		require.NoError(t, err)
		require.Zero(t, ended)
		return err
	}))
	_, err = module.SetSupervisionEnd(context.Background(), row.ID, row.StartDate, at)
	require.Error(t, err)
	_, err = module.EndOpenGroupSupervisions(context.Background(), group.ID, staff.ID, row.StartDate)
	require.Error(t, err)
	count, err := module.SetSupervisionEnd(ctx, row.ID, row.StartDate, at)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	expected := row
	expected.EndDate = &row.StartDate
	// The table trigger owns updated_at and replaces the supplied timestamp.
	require.True(t, read().UpdatedAt.After(row.UpdatedAt))
	expected.UpdatedAt = read().UpdatedAt
	require.Equal(t, expected, read())
	ended, err := module.EndOpenGroupSupervisions(ctx, group.ID, staff.ID, row.StartDate)
	require.NoError(t, err)
	require.Zero(t, ended, "an already-ended assignment is not overwritten")
	require.Equal(t, expected, read())
}

func TestSupervisionFiltersComposeAndPaginate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	staff := testpkg.CreateTestStaff(t, db, "Filter", "Supervisor")
	var observation compose.Observation
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(o compose.Observation) { observation = o }})
	require.NoError(t, err)
	date := "2026-08-24"
	first, err := module.RecordSupervision(ctx, studentpresence.GroupSupervision{
		GroupID: group.ID, StaffID: staff.ID, Role: "first", StartDate: date,
	})
	require.NoError(t, err)
	second, err := module.RecordSupervision(ctx, studentpresence.GroupSupervision{
		GroupID: group.ID, StaffID: staff.ID, Role: "second", StartDate: date,
	})
	require.NoError(t, err)
	filter := studentpresence.GroupSupervisionFilter{StaffIDs: []int64{staff.ID}, ActiveOn: &date, Limit: 1, Offset: 1}
	rows, err := module.QueryGroupSupervisions(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, []studentpresence.GroupSupervision{second}, rows)
	require.Equal(t, 1, observation.Queries)
	filter.Limit, filter.Offset = 0, 0
	filter.IDs = []int64{first.ID}
	rows, err = module.QueryGroupSupervisions(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, []studentpresence.GroupSupervision{first}, rows)
	filter.StaffIDs = []int64{}
	rows, err = module.QueryGroupSupervisions(ctx, filter)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, observation.Queries)
	_, err = module.QueryGroupSupervisions(context.Background(), filter)
	require.Error(t, err)
	_, err = module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{Limit: -1})
	require.Error(t, err)
	_, err = module.SetSupervisionEnd(ctx, first.ID, date, time.Now())
	require.NoError(t, err)
	rows, err = module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{EndedBy: &date, StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, first.ID, rows[0].ID)
	rows, err = module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{ActiveOn: &date, StaffIDs: []int64{staff.ID}})
	require.NoError(t, err)
	require.Equal(t, []studentpresence.GroupSupervision{second}, rows)
}
