package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func insertParentSession(ctx context.Context, db bun.IDB, accountID int64, tenantID, organizationID any) error {
	_, err := db.ExecContext(ctx, `INSERT INTO auth.tokens (account_id, token, expiry, portal_scope, tenant_id, organization_id)
		VALUES (?, encode(sha256(gen_random_uuid()::text::bytea), 'hex'), NOW() + INTERVAL '1 hour', 'parent', ?, ?)`, accountID, tenantID, organizationID)
	return err
}

// TestTraegerParentSessionNamesTraegerWithoutSchool pins the parent session
// at a Träger of 1.15.944 (ADR 0216 point 7): it names its Träger and no
// school, and every other parent session still names a school and no Träger.
func TestTraegerParentSessionNamesTraegerWithoutSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Elternkonto", Slug: fmt.Sprintf("traeger-elternkonto-%d", testpkg.UniqueTestTenantID(t))})
	accountID := testpkg.CreateTestAccount(t, db, "traeger-parent-session").ID

	assert.NoError(t, insertParentSession(ctx, db, accountID, nil, organization.ID), "a parent session at a Träger names the Träger and no school")
	assert.NoError(t, insertParentSession(ctx, db, accountID, school, nil), "a parent session at a school stays")
	assert.ErrorContains(t, insertParentSession(ctx, db, accountID, school, organization.ID), "chk_tokens_session_owner", "a parent session names a school or a Träger, not both")
	assert.ErrorContains(t, insertParentSession(ctx, db, accountID, nil, nil), "chk_tokens_session_owner", "a parent session names a school or a Träger")
	_, err := db.ExecContext(ctx, `DELETE FROM auth.tokens WHERE account_id = ?`, accountID)
	require.NoError(t, err)
}

// TestTraegerParentAccessKeepsTheAccount pins the retention of 1.15.944 (ADR
// 0216 point 10): an account set up for a holiday registration has no school,
// no profile and no child; its access keeps it from the hourly erasure only
// at an active, undeleted Träger and only for the 30 days of the setup link,
// and the access goes with the account or the Träger.
func TestTraegerParentAccessKeepsTheAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Aufbewahrung", Slug: fmt.Sprintf("traeger-aufbewahrung-%d", testpkg.UniqueTestTenantID(t))})
	account := testpkg.CreateTestAccount(t, db, "traeger-parent-retention")
	// A fresh account belongs to no school; the test owns it again at its end.
	testpkg.UnclaimTestAccount(t, db, account.ID)
	testpkg.OwnTestAccount(t, db, account.ID)
	unused := func() bool {
		var result bool
		require.NoError(t, db.NewRaw(`SELECT auth.account_is_unused(?, ?)`, account.ID, account.Email).Scan(ctx, &result))
		return result
	}
	require.True(t, unused(), "an account without anything is unused")

	_, err := db.ExecContext(ctx, `INSERT INTO auth.traeger_parent_access (account_id, organization_id) VALUES (?, ?)`, account.ID, organization.ID)
	require.NoError(t, err)
	assert.False(t, unused(), "a fresh access at an active Träger keeps the account until its first sign-in")

	exec := func(query string) {
		t.Helper()
		_, err := db.ExecContext(ctx, query, organization.ID)
		require.NoError(t, err)
	}
	exec(`UPDATE platform.organizations SET active = FALSE WHERE id = ?`)
	assert.True(t, unused(), "an access at an inactive Träger keeps nothing")
	exec(`UPDATE platform.organizations SET active = TRUE, deleted_at = NOW() WHERE id = ?`)
	assert.True(t, unused(), "an access at a deleted Träger keeps nothing")
	exec(`UPDATE platform.organizations SET deleted_at = NULL WHERE id = ?`)
	assert.False(t, unused())
	exec(`UPDATE auth.traeger_parent_access SET created_at = NOW() - INTERVAL '31 days' WHERE organization_id = ?`)
	assert.True(t, unused(), "past the 30 days of its setup link the access alone keeps nothing")

	_, err = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, organization.ID)
	require.NoError(t, err)
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.traeger_parent_access WHERE account_id = ?`, account.ID).Scan(ctx, &left))
	assert.Zero(t, left, "the access goes with the Träger")
	assert.True(t, unused(), "without the access the account is unused again")
}

// TestTraegerParentAccessMigrationRollsBack runs 1.15.944 down and up again on
// its own database: the down removes the parent sessions at a Träger before
// it restores the old constraint, and the table and the retention clause go.
func TestTraegerParentAccessMigrationRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Rückbau", Slug: fmt.Sprintf("traeger-rueckbau-%d", testpkg.UniqueTestTenantID(t))})
	account := testpkg.CreateTestAccount(t, db, "traeger-parent-rollback")
	// A fresh account belongs to no school; the test owns it again at its end.
	testpkg.UnclaimTestAccount(t, db, account.ID)
	testpkg.OwnTestAccount(t, db, account.ID)
	_, err := db.ExecContext(ctx, `INSERT INTO auth.traeger_parent_access (account_id, organization_id) VALUES (?, ?)`, account.ID, organization.ID)
	require.NoError(t, err)
	require.NoError(t, insertParentSession(ctx, db, account.ID, nil, organization.ID))

	require.NoError(t, traegerParentAccessDown(ctx, db))
	var sessions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ?`, account.ID).Scan(ctx, &sessions))
	assert.Zero(t, sessions, "the down removes the parent sessions at a Träger")
	assert.ErrorContains(t, insertParentSession(ctx, db, account.ID, nil, organization.ID), "chk_tokens_session_owner")
	var table *string
	require.NoError(t, db.NewRaw(`SELECT to_regclass('auth.traeger_parent_access')::text`).Scan(ctx, &table))
	assert.Nil(t, table, "the down drops the table")
	var unused bool
	require.NoError(t, db.NewRaw(`SELECT auth.account_is_unused(?, ?)`, account.ID, account.Email).Scan(ctx, &unused))
	assert.True(t, unused, "the down restores the retention of 1.15.646")

	require.NoError(t, traegerParentAccessUp(ctx, db))
	assert.NoError(t, insertParentSession(ctx, db, account.ID, nil, organization.ID))
	_, err = db.ExecContext(ctx, `DELETE FROM auth.tokens WHERE account_id = ?`, account.ID)
	require.NoError(t, err)
}

// The new erasure differs only by the deletion of the new access records.
func TestTraegerParentAccessErasureExtendsAccountErasure(t *testing.T) {
	t.Parallel()
	before := "DELETE FROM auth.tokens AS target WHERE target.account_id = ANY (erased_ids);"
	after := before + "\n\t\t\tDELETE FROM auth.traeger_parent_access AS target WHERE target.account_id = ANY (erased_ids);"
	require.Equal(t, 1, strings.Count(directPermissionAccountErasureUpSQL, before))
	assert.Equal(t, strings.Replace(directPermissionAccountErasureUpSQL, before, after, 1), traegerParentAccessAccountErasureSQL)
}
