package test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionFixturesGiveEachWriteItsOwnDeadline(t *testing.T) {
	t.Parallel()
	db := SetupIsolatedTestDB(t)
	tenantID := Tenant(t)

	// Each dependency finishes within its five-second budget, but together
	// they exceed it. A parent's insert must not spend its budget waiting for
	// those independently bounded fixture calls. The three fixtures write
	// separate rows, so they wait out their delays side by side.
	_, err := db.ExecContext(t.Context(), `
		CREATE FUNCTION public.delay_fixture_dependency() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(3); RETURN NEW; END $$;
		CREATE TRIGGER delay_category BEFORE INSERT ON activities.categories
		FOR EACH ROW EXECUTE FUNCTION public.delay_fixture_dependency();
		CREATE TRIGGER delay_staff BEFORE INSERT ON users.staff_school_memberships
		FOR EACH ROW EXECUTE FUNCTION public.delay_fixture_dependency();
	`)
	require.NoError(t, err)

	// Keep the runner slot while holding the clone slot: otherwise clone
	// waiters can occupy every runner slot and prevent parallel subtests
	// from starting. The fixture writes still run concurrently.
	var fixtures sync.WaitGroup
	for _, name := range []string{"activity", "tenant activity", "tenant session"} {
		fixtures.Go(func() {
			t.Run(name, func(t *testing.T) {
				var activityID int64
				switch name {
				case "activity":
					activityID = CreateTestActivityGroup(t, db, name).ID
				case "tenant activity":
					activityID = CreateTestActivityGroupForTenant(t, db, tenantID, name).ID
				case "tenant session":
					session := CreateTestActiveGroupForTenant(t, db, tenantID)
					activityID = *session.GroupID
					var storedTenant int64
					require.NoError(t, db.NewRaw("SELECT tenant_id FROM active.groups WHERE id = ?", session.ID).
						Scan(t.Context(), &storedTenant))
					require.Equal(t, tenantID, storedTenant)
				}
				var complete bool
				require.NoError(t, db.NewRaw(`SELECT EXISTS (
				SELECT FROM activities.groups g
				JOIN activities.categories c ON c.id = g.category_id AND c.tenant_id = g.tenant_id
				JOIN users.staff_school_memberships s ON s.id = g.created_by AND s.tenant_id = g.tenant_id
				WHERE g.id = ? AND g.tenant_id = ?
			)`, activityID, tenantID).Scan(t.Context(), &complete))
				require.True(t, complete)
			})
		})
	}
	fixtures.Wait()
}
