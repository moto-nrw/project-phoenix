// Tenant-portal half of the school-portal split (#2207 PR 3). Since the
// cutover removed the tenant-side class-day mount, an account whose only role
// at a school is a school-portal role has nothing left to reach in the OGS
// portal — so every path that mints or renews a TENANT session refuses it and
// points at moto schule.
//
// The guard sits at four places on purpose: sealing only the password login
// would leave the MFA hand-off and an already-issued refresh token as open
// side doors.
package auth_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// registerAccountAtTenant creates an account mapped to a fresh tenant and
// returns its credentials plus the tenant it belongs to.
func registerAccountAtTenant(t *testing.T, db *bun.DB, service auth.AuthService, prefix string) (email string, accountID, tenantID int64, slug string) {
	t.Helper()

	tenantID = testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	email, username := uniqueTestCredentials(prefix)
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)

	// No per-row cleanup: since #2419 every test binary runs against its own
	// database clone, which is dropped afterwards.
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	return email, account.ID, tenantID, fmt.Sprintf("t%d", tenantID)
}

func TestTenantLogin_SchoolPortalOnlyAccountIsRefused(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	email, accountID, tenantID, slug := registerAccountAtTenant(t, db, service, "school-only")
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantID)

	_, _, err := service.LoginWithAudit(context.Background(), email, testPassword, "", "", slug)

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMustUseSchoolPortal,
		"a Lehrkraft-only account must be sent to moto schule, not into the OGS portal")
}

func TestTenantLoginWithMFAGate_SchoolPortalOnlyAccountIsRefused(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	email, accountID, tenantID, slug := registerAccountAtTenant(t, db, service, "school-only-mfa")
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantID)

	// The MFA-aware login is what the HTTP handler actually calls; the
	// password-only sibling above is the legacy path.
	_, err := service.LoginWithMFAGate(context.Background(), email, testPassword, "", "", slug, "")

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMustUseSchoolPortal)
}

func TestTenantLogin_DualRoleLehrkraftPassesThrough(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	email, accountID, tenantID, slug := registerAccountAtTenant(t, db, service, "dual-role")
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantID)
	// Same person is also a Betreuungskraft at this school — small schools do
	// exactly this, and the cutover must not lock them out of the OGS portal.
	testpkg.AssignSystemRoleByName(t, db, accountID, tenantID, "user")

	accessToken, refreshToken, err := service.LoginWithAudit(context.Background(), email, testPassword, "", "", slug)

	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
}

func TestRefreshToken_SchoolPortalOnlyAccountIsRefused(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	email, accountID, tenantID, slug := registerAccountAtTenant(t, db, service, "refresh-school-only")
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantID)
	testpkg.AssignSystemRoleByName(t, db, accountID, tenantID, "user")

	_, refreshToken, err := service.LoginWithAudit(context.Background(), email, testPassword, "", "", slug)
	require.NoError(t, err)

	// The caregiver role is withdrawn while the session is still open. A
	// tenant session that keeps renewing itself would outlive the very role
	// that justified it.
	testpkg.RemoveSystemRoleByName(t, db, accountID, tenantID, "user")

	_, _, err = service.RefreshTokenWithAudit(context.Background(), refreshToken, "", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMustUseSchoolPortal)
}
