package postgres

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchoolPortalRoleReachesEveryReader pins that a school role made from
// the Lehrkraft template (#4574) carries its mark into every read that
// decides the school portal: the login roles, the staff preview listing and
// the push view definition. An edit of the role keeps the mark.
func TestSchoolPortalRoleReachesEveryReader(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	store := lockingStore(db, tenantID)

	unique := time.Now().UnixNano()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("school-portal-%d", unique))
	role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("klassenleitung-%d", unique), tenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	_, err := db.ExecContext(ctx, `UPDATE auth.roles SET school_portal = TRUE WHERE id = ?`, role.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())`,
		account.ID, role.ID, tenantID)
	require.NoError(t, err)

	roles, _, err := store.ListAccountRolesAtTenant(ctx, account.ID, tenantID, false)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.True(t, roles[0].SchoolPortal)
	assert.True(t, domain.IsSchoolPortalOnly(roles), "the copy alone opens moto schule only")

	accounts, _, err := store.ListTenantAccounts(ctx, tenantID)
	require.NoError(t, err)
	var listed *domain.TenantAccount
	for i := range accounts {
		if accounts[i].AccountID == account.ID {
			listed = &accounts[i]
		}
	}
	require.NotNil(t, listed)
	assert.True(t, listed.HoldsSchoolPortalCopy, "the staff preview listing sees the copy")

	var pushView string
	require.NoError(t, db.NewRaw(`SELECT pg_get_viewdef('platform.delivery_push_subscriptions'::regclass)`).Scan(ctx, &pushView))
	assert.Contains(t, pushView, "school_portal", "the push view counts the copy as a school-portal role")

	roleStore := NewRoleStore(store)
	managed, found, err := roleStore.FindRole(ctx, role.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, managed.SchoolPortal)
	managed.Description = "Leitet eine Klasse"
	managed.SchoolPortal = false
	require.NoError(t, roleStore.UpdateRole(ctx, managed))
	reread, _, err := roleStore.FindRole(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, "Leitet eine Klasse", reread.Description)
	assert.True(t, reread.SchoolPortal, "an edit keeps the template's mark")
}
