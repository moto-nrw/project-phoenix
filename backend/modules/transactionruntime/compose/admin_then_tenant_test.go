package compose_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The one-way handoff of ADR 0161 point 2 on a real database: one
// transaction, discovery as phoenix_admin, then phoenix_tenant under the
// discovered school's RLS, and no way back.

type handoffRig struct {
	db               *bun.DB
	ctx              context.Context
	schoolA, schoolB int64
	personA, personB int64
}

func newHandoffRig(t *testing.T) handoffRig {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	runtime, err := compose.NewTenantRuntime(db)
	require.NoError(t, err)
	rig := handoffRig{db: db, ctx: tenant.WithUnitOfWork(context.Background(), runtime)}
	rig.schoolA = testpkg.Tenant(t)
	rig.personA = testpkg.CreateTestStudent(t, db, "Anna", "SchuleA", "1a").PersonID
	t.Run("school B", func(t *testing.T) {
		rig.schoolB = testpkg.OwnTenant(t)
		testpkg.EnsureTestTenant(t, db, rig.schoolB)
		rig.personB = testpkg.CreateTestStudent(t, db, "Ben", "SchuleB", "1b").PersonID
	})
	require.NotEqual(t, rig.schoolA, rig.schoolB)
	return rig
}

func school(t *testing.T, id int64) tenant.TenantID {
	t.Helper()
	value, err := tenant.NewTenantID(id)
	require.NoError(t, err)
	return value
}

// probe reads role, school and how many of the two schools' persons the
// transaction of ctx sees.
func (r handoffRig) probe(t *testing.T, ctx context.Context) (string, string, int) {
	t.Helper()
	raw, ok := tenant.TransactionFromContext(ctx)
	require.True(t, ok, "the callback runs in the unit's transaction")
	tx, ok := raw.(bun.Tx)
	require.True(t, ok)
	var role, current string
	var visible int
	require.NoError(t, tx.NewRaw(
		"SELECT current_user::text, coalesce(current_setting('app.current_tenant_id', true), ''), (SELECT count(*) FROM users.persons WHERE id IN (?, ?))::int",
		r.personA, r.personB,
	).Scan(ctx, &role, &current, &visible))
	return role, current, visible
}

func TestAdminThenTenantSwitchesOnceToTheDiscoveredSchool(t *testing.T) {
	t.Parallel()
	rig := newHandoffRig(t)
	counter := testpkg.CaptureQueries(t, rig.db)

	var adminCtx context.Context
	err := tenant.WithinAdminThenTenant(rig.ctx, func(ctx context.Context) (int64, error) {
		adminCtx = ctx
		require.True(t, tenant.IsAdminTx(ctx))
		role, current, visible := rig.probe(t, ctx)
		require.Equal(t, "phoenix_admin", role)
		require.Empty(t, current)
		require.Equal(t, 2, visible, "discovery sees both schools")
		return rig.schoolA, nil
	}, func(ctx context.Context) error {
		require.False(t, tenant.IsAdminTx(ctx))
		require.Equal(t, rig.schoolA, tenant.FromContext(ctx))
		role, current, visible := rig.probe(t, ctx)
		require.Equal(t, "phoenix_tenant", role)
		require.Equal(t, fmt.Sprint(rig.schoolA), current)
		require.Equal(t, 1, visible, "the school part sees only the discovered school")

		// No way back: an administrative join fails like in any school unit,
		// and even the retained discovery context now runs as the school.
		require.Error(t, tenant.WithinAdmin(ctx, func(context.Context) error { return nil }))
		role, _, visible = rig.probe(t, adminCtx)
		require.Equal(t, "phoenix_tenant", role)
		require.Equal(t, 1, visible)
		// A school unit of the same school joins, another school is refused.
		require.NoError(t, tenant.WithinTenant(ctx, school(t, rig.schoolA), func(context.Context) error { return nil }))
		require.Error(t, tenant.WithinTenant(ctx, school(t, rig.schoolB), func(context.Context) error { return nil }))
		return nil
	})
	queries := counter.Queries()
	counter.Stop()
	require.NoError(t, err)
	require.Equal(t, "BEGIN", queries[0])
	require.Equal(t, "SET LOCAL ROLE phoenix_admin", queries[1])
	require.Equal(t, "COMMIT", queries[len(queries)-1])
	switchTo := fmt.Sprintf("SELECT set_config('role', 'phoenix_tenant', true), set_config('app.current_tenant_id', '%d', true)", rig.schoolA)
	begins, roles, switches := 0, 0, 0
	for _, q := range queries {
		switch q {
		case "BEGIN":
			begins++
		case "SET LOCAL ROLE phoenix_admin":
			roles++
		case switchTo:
			switches++
		}
	}
	require.Equal(t, []int{1, 1, 1}, []int{begins, roles, switches}, "one transaction, one admin part, one switch: %q", queries)
}

func TestAdminThenTenantEndsWithoutSwitchOnRefusal(t *testing.T) {
	t.Parallel()
	rig := newHandoffRig(t)
	refused := errors.New("not linked")
	for name, discover := range map[string]func(context.Context) (int64, error){
		"discovery refuses": func(context.Context) (int64, error) { return 0, refused },
		"no school":         func(context.Context) (int64, error) { return 0, nil },
	} {
		counter := testpkg.CaptureQueries(t, rig.db)
		called := false
		err := tenant.WithinAdminThenTenant(rig.ctx, discover, func(context.Context) error { called = true; return nil })
		queries := counter.Queries()
		counter.Stop()
		require.Error(t, err, name)
		require.False(t, called, name)
		require.Equal(t, []string{"BEGIN", "SET LOCAL ROLE phoenix_admin", "ROLLBACK"}, queries, name)
	}
	err := tenant.WithinAdminThenTenant(rig.ctx, func(context.Context) (int64, error) { return 0, refused }, func(context.Context) error { return nil })
	require.ErrorIs(t, err, refused, "the discovery's error is returned unchanged")
	failed := errors.New("read failed")
	err = tenant.WithinAdminThenTenant(rig.ctx, func(context.Context) (int64, error) { return rig.schoolA, nil }, func(context.Context) error { return failed })
	require.ErrorIs(t, err, failed)
}

func TestAdminThenTenantRefusesAmbientUnitsAndForeignSchools(t *testing.T) {
	t.Parallel()
	rig := newHandoffRig(t)
	open := func(ctx context.Context) error {
		return tenant.WithinAdminThenTenant(ctx, func(context.Context) (int64, error) { return rig.schoolA, nil }, func(context.Context) error { return nil })
	}
	require.NoError(t, tenant.WithinAdmin(rig.ctx, func(ctx context.Context) error {
		require.Error(t, open(ctx), "inside an administrative unit")
		return nil
	}))
	for _, id := range []int64{rig.schoolA, rig.schoolB} {
		require.NoError(t, tenant.WithinTenant(rig.ctx, school(t, id), func(ctx context.Context) error {
			require.Error(t, open(ctx), "inside a school unit of school %d", id)
			return nil
		}))
	}
	require.Error(t, open(tenant.WithTenantID(rig.ctx, rig.schoolB)), "a context bound to another school")
	require.NoError(t, open(tenant.WithTenantID(rig.ctx, rig.schoolA)), "a context bound to the discovered school")

	demo, err := compose.NewDemoTenantRuntime(rig.db)
	require.NoError(t, err)
	require.Error(t, open(tenant.WithUnitOfWork(context.Background(), demo)), "the demo runtime opens no handoff")
}

func TestAdminThenTenantLeavesThePooledConnectionClean(t *testing.T) {
	t.Parallel()
	rig := newHandoffRig(t)
	rig.db.SetMaxIdleConns(1)
	rig.db.SetMaxOpenConns(1)
	var login string
	require.NoError(t, rig.db.NewRaw("SELECT current_user").Scan(context.Background(), &login))
	for _, fail := range []bool{false, true} {
		_ = tenant.WithinAdminThenTenant(rig.ctx, func(context.Context) (int64, error) { return rig.schoolA, nil }, func(context.Context) error {
			if fail {
				return errors.New("roll back")
			}
			return nil
		})
		var role, current string
		require.NoError(t, rig.db.NewRaw("SELECT current_user, coalesce(current_setting('app.current_tenant_id', true), '')").Scan(context.Background(), &role, &current))
		require.Equal(t, login, role)
		require.Empty(t, current)
	}
}
