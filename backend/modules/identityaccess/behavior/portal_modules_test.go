// The parents portal follows the module Eltern-App, the school portal the
// module Unterricht am Vormittag (#4895): a school that has not booked the
// module issues and renews no session of that portal.
package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const (
	moduleElternApp           = "module.eltern_app"
	moduleUnterrichtVormittag = "module.unterricht_vormittag"
)

func cancelModule(t *testing.T, db *bun.DB, tenantID int64, key string) {
	t.Helper()
	testpkg.SetTestTenantSetting(t, db, tenantID, key, "false")
}

func bookModule(t *testing.T, db *bun.DB, tenantID int64, key string) {
	t.Helper()
	testpkg.SetTestTenantSetting(t, db, tenantID, key, "true")
}

func requirePortalNotBooked(t *testing.T, err error, module string) {
	t.Helper()
	require.Error(t, err)
	notBooked, ok := identityaccess.PortalNotBooked(err)
	require.Truef(t, ok, "want PortalNotBookedError, got %v", err)
	assert.Equal(t, module, notBooked.Module)
	assert.NotEmpty(t, notBooked.ModuleLabel, "the refusal names the module for the explanation")
}

// newGuardianAccount registers an account with the guardian role at every
// given school.
func newGuardianAccount(t *testing.T, db *bun.DB, service testAuthService, prefix string, tenantIDs ...int64) (email string, accountID int64) {
	t.Helper()
	email, username := uniqueTestCredentials(prefix)
	account, err := service.Register(testpkg.TenantContext(tenantIDs[0]), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	for _, tenantID := range tenantIDs {
		testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
		assignSeededRole(t, db, account.ID, tenantID, "guardian")
	}
	return email, account.ID
}

func TestLoginSchool_PortalNotBooked_Refused(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-unbooked", tenantID)
	cancelModule(t, db, tenantID, moduleUnterrichtVormittag)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	assert.Nil(t, result)
	requirePortalNotBooked(t, err, "unterricht_vormittag")
}

func TestLoginSchool_OtherSchoolsBookingDoesNotLeak(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	booked, _ := newSchoolTenant(t, db)
	cancelled, _ := newSchoolTenant(t, db)
	bookedEmail, _ := createLehrkraftAccount(t, db, service, "school-booked", booked)
	cancelledEmail, _ := createLehrkraftAccount(t, db, service, "school-cancelled", cancelled)
	cancelModule(t, db, cancelled, moduleUnterrichtVormittag)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), bookedEmail, testPassword, "", "", "")
	require.NoError(t, err, "the cancelled booking of another school must not lock this one")
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, result.Status)

	_, err = service.LoginSchoolWithMFAGate(context.Background(), cancelledEmail, testPassword, "", "", "")
	requirePortalNotBooked(t, err, "unterricht_vormittag")
}

func TestRefreshToken_SchoolScope_PortalCancelled_RefusedAndResumesAfterBooking(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-refresh-unbooked", tenantID)
	login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")
	require.NoError(t, err)

	cancelModule(t, db, tenantID, moduleUnterrichtVormittag)
	_, _, err = service.RefreshTokenWithAudit(context.Background(), login.RefreshToken, "", "", "")
	requirePortalNotBooked(t, err, "unterricht_vormittag")

	bookModule(t, db, tenantID, moduleUnterrichtVormittag)
	_, refreshToken, err := service.RefreshTokenWithAudit(context.Background(), login.RefreshToken, "", "", "")
	require.NoError(t, err, "the refusal must leave the presented refresh token intact")
	assert.NotEmpty(t, refreshToken)
}

func TestIssueSchoolTokens_PortalNotBooked_Refused(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := newSchoolTenant(t, db)
	_, accountID := createLehrkraftAccount(t, db, service, "school-issue-unbooked", tenantID)
	cancelModule(t, db, tenantID, moduleUnterrichtVormittag)

	_, _, err := service.IssueSchoolTokensForAuthenticatedAccount(context.Background(), accountID, tenantID, "", "")

	requirePortalNotBooked(t, err, "unterricht_vormittag")
}

func TestLoginParent_PortalNotBooked_Refused(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	email, _ := newGuardianAccount(t, db, service, "parent-unbooked", tenantID)
	cancelModule(t, db, tenantID, moduleElternApp)

	_, _, err := service.LoginParent(context.Background(), email, testPassword)

	requirePortalNotBooked(t, err, "eltern_app")
}

func TestLoginParent_OneBookedSchoolOpensThePortal(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	cancelled, _ := testpkg.CreateTestTenant(t, db)
	booked, _ := testpkg.CreateTestTenant(t, db)
	email, accountID := newGuardianAccount(t, db, service, "parent-two-schools", cancelled, booked)
	cancelModule(t, db, cancelled, moduleElternApp)

	accessToken, refreshToken, err := service.LoginParent(context.Background(), email, testPassword)

	require.NoError(t, err, "a guardian with one school that booked Eltern-App keeps the portal")
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	var pinnedTenant int64
	require.NoError(t, db.NewRaw(`SELECT tenant_id FROM auth.tokens WHERE portal_scope = 'parent' AND account_id = ?
		ORDER BY id DESC LIMIT 1`, accountID).Scan(context.Background(), &pinnedTenant))
	assert.Equal(t, booked, pinnedTenant, "the session is pinned to a school that booked the portal")
}

func TestRefreshToken_ParentScope_PortalCancelled_RefusedAndResumesAfterBooking(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	email, _ := newGuardianAccount(t, db, service, "parent-refresh-unbooked", tenantID)
	_, refreshToken, err := service.LoginParent(context.Background(), email, testPassword)
	require.NoError(t, err)

	cancelModule(t, db, tenantID, moduleElternApp)
	_, _, err = service.RefreshToken(context.Background(), refreshToken)
	requirePortalNotBooked(t, err, "eltern_app")

	bookModule(t, db, tenantID, moduleElternApp)
	_, renewed, err := service.RefreshToken(context.Background(), refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, renewed)
}

func TestTenantLogin_IgnoresThePortalModules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("tenant-portal-modules")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	assignSeededRole(t, db, account.ID, tenantID, "user")
	cancelModule(t, db, tenantID, moduleElternApp)
	cancelModule(t, db, tenantID, moduleUnterrichtVormittag)

	_, refreshToken, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err, "the team portal is Basis")
	_, _, err = service.RefreshToken(ctx, refreshToken)
	require.NoError(t, err)
}
