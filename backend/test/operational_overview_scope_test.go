// School-wide operational overview (#2380).
//
// The live-group enumeration a covered caller gets must stop at their own
// school even with the widest overview scope. These tests use real
// phoenix_tenant transactions, because RLS is what guarantees that.
package test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// TestOperationalOverviewNeverCrossesTenants pins what the broad list actually
// returns: even with the widest scope, the enumeration a covered caller gets
// stops at their own school's active groups. RLS is what guarantees that, so
// the query runs through a real phoenix_tenant transaction.
func TestOperationalOverviewNeverCrossesTenants(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	tenantA := UniqueTestTenantID(t)
	tenantB := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)

	// Both schools open their running modules to all staff — the worst case
	// for a leak, since neither side's scope narrows the query.
	SetTestTenantSetting(t, db, tenantA, settings.KeyOperationalOverviewScope, `"`+settings.OverviewScopeAllStaff+`"`)
	SetTestTenantSetting(t, db, tenantB, settings.KeyOperationalOverviewScope, `"`+settings.OverviewScopeAllStaff+`"`)

	groupA := CreateTestActiveGroupForTenant(t, db, tenantA)
	groupB := CreateTestActiveGroupForTenant(t, db, tenantB)

	presence := api.NewStudentPresenceForTests(NewDatabase(t, db))

	assertSeesOnlyOwn := func(tb testing.TB, ownTenant, ownGroup, foreignGroup int64) {
		tb.Helper()
		err := WithTenantTx(t, context.Background(), db, ownTenant, func(txCtx context.Context, _ bun.Tx) error {
			groups, err := presence.QueryLiveGroups(txCtx, studentpresence.LiveGroupFilter{})
			require.NoError(tb, err)

			ids := make(map[int64]bool, len(groups))
			for _, group := range groups {
				ids[group.ID] = true
			}
			assert.True(tb, ids[ownGroup], "a school must see its own running module")
			assert.False(tb, ids[foreignGroup], "the school-wide overview leaked another school's running module")
			return nil
		})
		require.NoError(tb, err)
	}

	assertSeesOnlyOwn(t, tenantA, groupA.ID, groupB.ID)
	assertSeesOnlyOwn(t, tenantB, groupB.ID, groupA.ID)
}
