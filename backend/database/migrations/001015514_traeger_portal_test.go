package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraegerMembershipKeepsTheAccountInUse: a person who works only in the
// Träger-Büro has no school mapping, and the nightly erasure must not
// anonymize them; an ended membership goes with the erased account.
func TestTraegerMembershipKeepsTheAccountInUse(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Erasure", Slug: "traeger-erasure"})
	account := testpkg.CreateTestAccount(t, db, "traeger-buero")
	testpkg.UnclaimTestAccount(t, db, account.ID)
	testpkg.CreateTestTraegerMember(t, db, organization.ID, account.ID, "administration")

	unused := func() bool {
		var result bool
		require.NoError(t, db.NewRaw(`SELECT auth.account_is_unused(id, email) FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &result))
		return result
	}
	assert.False(t, unused(), "an active Träger membership keeps the account")
	assert.NotContains(t, eraseUnusedAccountsAsAdmin(t, db), account.ID)

	_, err := db.ExecContext(ctx, `UPDATE platform.organization_accounts SET status = 'inactive', deactivated_at = NOW() WHERE account_id = ?`, account.ID)
	require.NoError(t, err)
	assert.True(t, unused(), "an ended membership keeps nothing")
	assert.Contains(t, eraseUnusedAccountsAsAdmin(t, db), account.ID)
	for _, table := range []string{"platform.organization_accounts", "auth.organization_account_roles"} {
		var left int
		require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE account_id = ?`, account.ID).Scan(ctx, &left))
		assert.Zero(t, left, "%s of the erased account is deleted", table)
	}
}

// TestTraegerSessionNamesItsTraegerAndNoSchool pins chk_tokens_session_owner:
// a Träger session (portal scope org) names its Träger and no school, every
// other session its school; school-portal and parent sessions no Träger.
func TestTraegerSessionNamesItsTraegerAndNoSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Sitzung", Slug: fmt.Sprintf("traeger-sitzung-%d", testpkg.UniqueTestTenantID(t))})
	accountID := testpkg.CreateTestAccount(t, db, "traeger-sitzung").ID
	insert := func(scope string, tenantID, organizationID any) error {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.tokens (account_id, token, expiry, portal_scope, tenant_id, organization_id)
			VALUES (?, encode(sha256(gen_random_uuid()::text::bytea), 'hex'), NOW() + INTERVAL '1 hour', ?, ?, ?)`, accountID, scope, tenantID, organizationID)
		return err
	}
	assert.NoError(t, insert("org", nil, organization.ID))
	assert.ErrorContains(t, insert("org", school, organization.ID), "chk_tokens_session_owner", "a Träger session names no school")
	assert.ErrorContains(t, insert("org", nil, nil), "chk_tokens_session_owner", "a Träger session names its Träger")
	assert.ErrorContains(t, insert("school", school, organization.ID), "chk_tokens_session_owner", "a school-portal session names no Träger")
	assert.ErrorContains(t, insert("parent", school, organization.ID), "chk_tokens_session_owner", "a parent session names no Träger")
	// Since 1.15.556 an OGS session may name the Träger it came through
	// (Absprung, #4654); TestTraegerSchoolSessionNamesSchoolAndTraeger.
	assert.ErrorContains(t, insert("tenant", nil, nil), "chk_tokens_session_owner", "a school session names its school")
	assert.NoError(t, insert("tenant", school, nil))
	_, err := db.ExecContext(ctx, `DELETE FROM auth.tokens WHERE account_id = ?`, accountID)
	require.NoError(t, err)
}
