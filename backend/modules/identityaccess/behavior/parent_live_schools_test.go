// A school the operator switched inactive or deleted locks out its staff;
// the parents portal follows the same gate. Parent sessions open and stay
// pinned only at a live guardian school.
package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func softDeleteSchool(t *testing.T, db *bun.DB, tenantID int64) {
	t.Helper()
	_, err := db.Exec("UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?", tenantID)
	require.NoError(t, err)
}

func TestLoginParent_NoLiveGuardianSchool_Refused(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	inactive, _ := testpkg.CreateTestTenant(t, db)
	deleted, _ := testpkg.CreateTestTenant(t, db)
	inactiveEmail, _ := newGuardianAccount(t, db, service, "parent-inactive-school", inactive)
	deletedEmail, _ := newGuardianAccount(t, db, service, "parent-deleted-school", deleted)
	setSchoolActive(t, db, inactive, false)
	softDeleteSchool(t, db, deleted)

	for name, email := range map[string]string{"inactive": inactiveEmail, "deleted": deletedEmail} {
		accessToken, refreshToken, err := service.LoginParent(context.Background(), email, testPassword)
		require.ErrorIsf(t, err, identityaccess.ErrAccountNoGuardianRole, "%s school", name)
		assert.Empty(t, accessToken, name)
		assert.Empty(t, refreshToken, name)
	}
}

func TestLoginParent_PinsTheSessionToALiveSchool(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	inactive, _ := testpkg.CreateTestTenant(t, db)
	live, _ := testpkg.CreateTestTenant(t, db)
	email, accountID := newGuardianAccount(t, db, service, "parent-live-and-inactive", inactive, live)
	setSchoolActive(t, db, inactive, false)

	_, refreshToken, err := service.LoginParent(context.Background(), email, testPassword)
	require.NoError(t, err, "one live guardian school keeps the portal")
	var pinnedTenant int64
	require.NoError(t, db.NewRaw(`SELECT tenant_id FROM auth.tokens WHERE portal_scope = 'parent' AND account_id = ?
		ORDER BY id DESC LIMIT 1`, accountID).Scan(context.Background(), &pinnedTenant))
	assert.Equal(t, live, pinnedTenant, "the session is pinned to the live school")

	_, renewed, err := service.RefreshToken(context.Background(), refreshToken)
	require.NoError(t, err, "the session renews while its school is live")
	assert.NotEmpty(t, renewed)
}

func TestRefreshToken_ParentScope_SchoolSwitchedInactive_Refused(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	email, _ := newGuardianAccount(t, db, service, "parent-refresh-inactive", tenantID)
	_, refreshToken, err := service.LoginParent(context.Background(), email, testPassword)
	require.NoError(t, err)

	setSchoolActive(t, db, tenantID, false)
	_, _, err = service.RefreshToken(context.Background(), refreshToken)
	require.ErrorIs(t, err, identityaccess.ErrTenantNotFound)

	setSchoolActive(t, db, tenantID, true)
	_, renewed, err := service.RefreshToken(context.Background(), refreshToken)
	require.NoError(t, err, "the refusal leaves the presented refresh token intact")
	assert.NotEmpty(t, renewed)
}
