package integration_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetableprojection"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

func projectionReads(id int64, ids []int64) map[string]func(context.Context, bun.IDB, int64) (any, error) {
	date := timezone.NewDate(2026, 9, 4)
	return map[string]func(context.Context, bun.IDB, int64) (any, error){
		"group names": func(ctx context.Context, db bun.IDB, tenantID int64) (any, error) {
			return timetableprojection.GroupNames(ctx, db, tenantID, ids)
		},
		"activity groups": func(ctx context.Context, db bun.IDB, tenantID int64) (any, error) {
			return timetableprojection.ActivityGroupsByID(ctx, db, tenantID, ids)
		},
		"manual planning": func(ctx context.Context, db bun.IDB, tenantID int64) (any, error) {
			return timetableprojection.ListManualPlanningOccurrences(ctx, db, tenantID, id, date, date)
		},
		"request enrollments": func(ctx context.Context, db bun.IDB, tenantID int64) (any, error) {
			return timetableprojection.CountRequestSourceEnrollments(ctx, db, tenantID, id)
		},
		"child enrollments": func(ctx context.Context, db bun.IDB, tenantID int64) (any, error) {
			return timetableprojection.CountChildSourceEnrollments(ctx, db, tenantID, id)
		},
		"student enrollments": func(ctx context.Context, db bun.IDB, tenantID int64) (any, error) {
			return timetableprojection.CountStudentEnrollments(ctx, db, tenantID, id)
		},
		"running enrollments": func(ctx context.Context, db bun.IDB, tenantID int64) (any, error) {
			return timetableprojection.CountRunningEnrollmentsAfter(ctx, db, tenantID, ids, date, "[]")
		},
	}
}

func TestProjectionReadsRejectInvalidTenantBeforeSQL(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Group")
	ctx := testpkg.Ctx(t)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx = counter.Context(ctx)
	for _, ids := range [][]int64{{group.ID}, nil} {
		for name, read := range projectionReads(group.ID, ids) {
			t.Run(name, func(t *testing.T) {
				// An ambient valid tenant must not rescue an invalid explicit argument.
				for _, invalid := range []int64{0, -1} {
					result, err := read(ctx, tx, invalid)
					assert.ErrorIs(t, err, timetableprojection.ErrInvalidTenantID)
					assert.Empty(t, result)
				}
			})
		}
	}
	assert.Empty(t, counter.Queries(), "invalid tenants must fail before SQL, including empty inputs")
}

func TestProjectionReadsPreserveDatabaseErrors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	group := testpkg.CreateTestActivityGroup(t, db, "Group")
	ctx := testpkg.Ctx(t)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	for name, read := range projectionReads(group.ID, []int64{group.ID}) {
		t.Run(name, func(t *testing.T) {
			_, err := read(ctx, tx, testpkg.Tenant(t))
			assert.ErrorIs(t, err, sql.ErrTxDone)
			assert.ErrorContains(t, err, "timetable projection:")
		})
	}
	// Empty batch reads still succeed without touching the database for valid tenants.
	names, err := timetableprojection.GroupNames(ctx, tx, testpkg.Tenant(t), nil)
	require.NoError(t, err)
	assert.NotNil(t, names)
	assert.Empty(t, names)
	groups, err := timetableprojection.ActivityGroupsByID(ctx, tx, testpkg.Tenant(t), nil)
	require.NoError(t, err)
	assert.NotNil(t, groups)
	assert.Empty(t, groups)
	counts, err := timetableprojection.CountRunningEnrollmentsAfter(ctx, tx, testpkg.Tenant(t), nil, timezone.NewDate(2026, 9, 4), "[]")
	require.NoError(t, err)
	assert.NotNil(t, counts)
	assert.Empty(t, counts)
}

func TestGroupNamesRejectsMissingTenantInElevatedTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := testpkg.CreateTestActivityGroup(t, db, "Own group")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	other := testpkg.CreateTestActivityGroupForTenant(t, db, otherTenant, "Other group")
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	ids := []int64{own.ID, other.ID}
	// Prove this transaction can see both tenants without an explicit predicate.
	var visible int
	require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM activities.groups WHERE id IN (?)`, bun.List(ids)).Scan(ctx, &visible))
	require.Equal(t, 2, visible)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx = counter.Context(ctx)
	names, err := timetableprojection.GroupNames(ctx, tx, tenant.FromContext(ctx), ids)
	assert.ErrorIs(t, err, timetableprojection.ErrInvalidTenantID)
	assert.Nil(t, names)
	assert.Empty(t, counter.Queries(), "missing tenant must fail before SQL")

	names, err = timetableprojection.GroupNames(ctx, tx, testpkg.Tenant(t), ids)
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{own.ID: own.Name}, names)
	groups, err := timetableprojection.ActivityGroupsByID(ctx, tx, testpkg.Tenant(t), ids)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, own.ID, groups[0].ID)
}
