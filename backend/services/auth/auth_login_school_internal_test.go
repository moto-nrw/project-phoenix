package auth

// Failure-path coverage for the school-portal role resolution (#2207).
// These branches decide whether a transient DB failure logs a Lehrkraft out
// for good ("no portal role", a terminal 403) or surfaces as a retryable
// error — so they are exercised by injecting repository failures on top of
// a real tenant/account fixture instead of hoping for a DB blip.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// --- repository failure injectors -------------------------------------------------

// failingAccountTenantRepo fails the active-mapping enumeration and delegates
// everything else to the real repository.
type failingAccountTenantRepo struct {
	authModels.AccountTenantRepository
	err error
}

func (r failingAccountTenantRepo) FindActiveByAccountID(context.Context, int64) ([]authModels.AccountTenant, error) {
	return nil, r.err
}

// failingSchoolRepo fails the school lookup from the failFrom-th call on
// (1-based). The school-metadata path looks the school up twice — org id,
// then liveness — which is what failFrom = 2 targets.
type failingSchoolRepo struct {
	platformModels.SchoolRepository
	err      error
	failFrom int
	calls    *int
}

func (r failingSchoolRepo) FindByID(ctx context.Context, id int64) (*platformModels.School, error) {
	*r.calls++
	if *r.calls >= r.failFrom {
		return nil, r.err
	}
	return r.SchoolRepository.FindByID(ctx, id)
}

// failingAccountRoleRepo fails the per-tenant role lookup.
type failingAccountRoleRepo struct {
	authModels.AccountRoleRepository
	err error
}

func (r failingAccountRoleRepo) FindByAccountIDForTenant(context.Context, int64, int64) ([]*authModels.AccountRole, error) {
	return nil, r.err
}

// --- fixture ----------------------------------------------------------------------

const schoolPortalFixturePassword = "Test1234%" //nolint:gosec // test credential

// newSchoolPortalFixture registers an account mapped to a fresh active school
// and returns the service under test. The account holds NO role — every test
// below either fails before the role check or injects the role repository.
func newSchoolPortalFixture(t *testing.T) (service *Service, account *authModels.Account, tenantID int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service = setupInternalAuthService(t, db)
	tenantID = testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	unique := time.Now().UnixNano()
	account, err := service.Register(
		testpkg.TenantContext(tenantID),
		fmt.Sprintf("school-fail-%d@test.local", unique),
		fmt.Sprintf("school-fail-%d", unique),
		schoolPortalFixturePassword,
		nil, 0,
	)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	return service, account, tenantID
}

// --- findSchoolPortalTenantForAccount ----------------------------------------------

func TestFindSchoolPortalTenant_MappingLookupError_Propagates(t *testing.T) {
	service, account, _ := newSchoolPortalFixture(t)
	dbErr := errors.New("connection reset")
	service.repos.AccountTenant = failingAccountTenantRepo{
		AccountTenantRepository: service.repos.AccountTenant,
		err:                     dbErr,
	}

	hasRole, tenantID, err := service.findSchoolPortalTenantForAccount(context.Background(), account.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	assert.False(t, hasRole)
	assert.Zero(t, tenantID)
}

func TestFindSchoolPortalTenant_SchoolLookupError_Propagates(t *testing.T) {
	// A DB failure while checking whether the mapped school is alive must
	// not be flattened into "no portal role" — that would be a terminal
	// 403 for a Lehrkraft whose school is perfectly fine.
	service, account, _ := newSchoolPortalFixture(t)
	dbErr := errors.New("connection reset")
	calls := 0
	service.repos.School = failingSchoolRepo{
		SchoolRepository: service.repos.School,
		err:              dbErr,
		failFrom:         1,
		calls:            &calls,
	}

	hasRole, _, err := service.findSchoolPortalTenantForAccount(context.Background(), account.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	assert.False(t, hasRole)
}

func TestFindSchoolPortalTenant_SchoolNotFound_SkipsMapping(t *testing.T) {
	// A hard-deleted school behind a stale mapping is not an error — the
	// scan moves on and reports "no portal role".
	service, account, _ := newSchoolPortalFixture(t)
	calls := 0
	service.repos.School = failingSchoolRepo{
		SchoolRepository: service.repos.School,
		err:              sql.ErrNoRows,
		failFrom:         1,
		calls:            &calls,
	}

	hasRole, tenantID, err := service.findSchoolPortalTenantForAccount(context.Background(), account.ID)

	require.NoError(t, err, "a missing school must be skipped, not surfaced as an error")
	assert.False(t, hasRole)
	assert.Zero(t, tenantID)
}

func TestFindSchoolPortalTenant_RoleLookupError_Propagates(t *testing.T) {
	service, account, _ := newSchoolPortalFixture(t)
	dbErr := errors.New("connection reset")
	service.repos.AccountRole = failingAccountRoleRepo{
		AccountRoleRepository: service.repos.AccountRole,
		err:                   dbErr,
	}

	hasRole, _, err := service.findSchoolPortalTenantForAccount(context.Background(), account.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	assert.False(t, hasRole)
}

func TestFindSchoolPortalTenant_RoleLookupNotFound_SkipsMapping(t *testing.T) {
	service, account, _ := newSchoolPortalFixture(t)
	service.repos.AccountRole = failingAccountRoleRepo{
		AccountRoleRepository: service.repos.AccountRole,
		err:                   sql.ErrNoRows,
	}

	hasRole, _, err := service.findSchoolPortalTenantForAccount(context.Background(), account.ID)

	require.NoError(t, err, "an empty role result must not error out the scan")
	assert.False(t, hasRole)
}

func TestLoginSchool_MappingLookupError_SurfacesAsAuthError(t *testing.T) {
	// The login wrapper turns the enumeration failure into an AuthError
	// carrying the DB cause, not the portal-role sentinel: the handler must
	// answer 500, never a terminal 403.
	service, account, _ := newSchoolPortalFixture(t)
	dbErr := errors.New("connection reset")
	service.repos.AccountTenant = failingAccountTenantRepo{
		AccountTenantRepository: service.repos.AccountTenant,
		err:                     dbErr,
	}

	result, err := service.LoginSchoolWithMFAGate(
		context.Background(), account.Email, schoolPortalFixturePassword, "", "", "")

	require.Error(t, err)
	assert.Nil(t, result)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, dbErr)
	assert.NotErrorIs(t, authErr.Err, ErrAccountNoSchoolPortalRole)
}

// --- hasSchoolPortalRoleAtTenant ---------------------------------------------------

func TestHasSchoolPortalRoleAtTenant_RoleLookupError_Propagates(t *testing.T) {
	// Unlike the swallow-and-warn role hydration, this check propagates —
	// on the refresh path a masked error would log the user out for good.
	service, account, tenantID := newSchoolPortalFixture(t)
	dbErr := errors.New("connection reset")
	service.repos.AccountRole = failingAccountRoleRepo{
		AccountRoleRepository: service.repos.AccountRole,
		err:                   dbErr,
	}

	hasRole, err := service.hasSchoolPortalRoleAtTenant(context.Background(), account.ID, tenantID)

	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr)
	assert.False(t, hasRole)
}

func TestHasSchoolPortalRoleAtTenant_RoleLookupNotFound_ReportsNoRole(t *testing.T) {
	service, account, tenantID := newSchoolPortalFixture(t)
	service.repos.AccountRole = failingAccountRoleRepo{
		AccountRoleRepository: service.repos.AccountRole,
		err:                   sql.ErrNoRows,
	}

	hasRole, err := service.hasSchoolPortalRoleAtTenant(context.Background(), account.ID, tenantID)

	require.NoError(t, err, "an empty role result is not an error")
	assert.False(t, hasRole)
}

// --- loadSchoolMetadataForTenant ---------------------------------------------------

func TestLoadSchoolMetadataForTenant_MetadataError_Propagates(t *testing.T) {
	service, account, tenantID := newSchoolPortalFixture(t)
	calls := 0
	service.repos.School = failingSchoolRepo{
		SchoolRepository: service.repos.School,
		err:              sql.ErrNoRows,
		failFrom:         1,
		calls:            &calls,
	}

	metadata, err := service.loadSchoolMetadataForTenant(context.Background(), account, tenantID)

	require.Error(t, err)
	assert.Nil(t, metadata)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

func TestLoadSchoolMetadataForTenant_LivenessLookupError_Propagates(t *testing.T) {
	// The shared metadata load succeeds, then the school-liveness lookup
	// fails: a transient error there must not be mistaken for a dead school
	// (which would be a terminal 404 for a live one).
	service, account, tenantID := newSchoolPortalFixture(t)
	dbErr := errors.New("connection reset")
	calls := 0
	service.repos.School = failingSchoolRepo{
		SchoolRepository: service.repos.School,
		err:              dbErr,
		failFrom:         2,
		calls:            &calls,
	}

	metadata, err := service.loadSchoolMetadataForTenant(context.Background(), account, tenantID)

	require.Error(t, err)
	assert.Nil(t, metadata)
	assert.ErrorIs(t, err, dbErr)
}

// --- schoolMintGuard ---------------------------------------------------------------

// newMintGuardFixture builds a Lehrkraft who may legitimately hold a school
// session: active account, active mapping, lehrkraft role, live school. Each
// test below then breaks exactly one of those facts.
//
// The school row itself gets deactivated and soft-deleted here, so the tenant
// must be one nobody else shares — CreateTestTenant, not a literal id.
func newMintGuardFixture(t *testing.T) (service *Service, db *bun.DB, account *authModels.Account, tenantID int64) {
	t.Helper()
	db = testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	service = setupInternalAuthService(t, db)

	tenantID, _ = testpkg.CreateTestTenant(t, db)
	unique := time.Now().UnixNano()
	account, err := service.Register(
		testpkg.TenantContext(tenantID),
		fmt.Sprintf("school-guard-%d@test.local", unique),
		fmt.Sprintf("school-guard-%d", unique),
		schoolPortalFixturePassword,
		nil, 0,
	)
	require.NoError(t, err)
	// Cleanup order matters: the account rows reference the school, so the
	// school teardown has to run after them (t.Cleanup is LIFO).
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, tenantID) })
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })

	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)

	return service, db, account, tenantID
}

// runMintGuard executes the guard the way the mint does: inside a
// phoenix_admin transaction, which is what its FOR SHARE reads require.
func runMintGuard(t *testing.T, service *Service, db *bun.DB, accountID, tenantID int64) error {
	t.Helper()
	guard := service.schoolMintGuard(accountID, tenantID)
	var guardErr error
	txErr := tenant.WithAdminTx(context.Background(), db, func(ctx context.Context, _ bun.Tx) error {
		guardErr = guard(ctx)
		return nil
	})
	require.NoError(t, txErr)
	return guardErr
}

func TestSchoolMintGuard_LiveSessionPasses(t *testing.T) {
	service, db, account, tenantID := newMintGuardFixture(t)
	require.NoError(t, runMintGuard(t, service, db, account.ID, tenantID))
}

func TestSchoolMintGuard_RefusesRevokedMembership(t *testing.T) {
	// Membership is revoked by flipping account_tenants.status. Everything
	// else about the account stays valid, which is exactly why the guard has
	// to look at this row rather than trust the earlier resolution.
	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec(
		"UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?",
		account.ID, tenantID)
	require.NoError(t, err)

	assert.ErrorIs(t, runMintGuard(t, service, db, account.ID, tenantID), ErrTenantAccessDenied)
}

func TestSchoolMintGuard_RefusesRevokedPortalRole(t *testing.T) {
	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec(
		"DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?",
		account.ID, tenantID)
	require.NoError(t, err)

	assert.ErrorIs(t, runMintGuard(t, service, db, account.ID, tenantID), ErrAccountNoSchoolPortalRole)
}

func TestSchoolMintGuard_RefusesDeactivatedSchool(t *testing.T) {
	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec("UPDATE platform.schools SET active = false WHERE id = ?", tenantID)
	require.NoError(t, err)

	assert.ErrorIs(t, runMintGuard(t, service, db, account.ID, tenantID), ErrTenantNotFound)
}

func TestSchoolMintGuard_RefusesSoftDeletedSchool(t *testing.T) {
	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec("UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?", tenantID)
	require.NoError(t, err)

	assert.ErrorIs(t, runMintGuard(t, service, db, account.ID, tenantID), ErrTenantNotFound)
}

func TestCreateRefreshTokenWithRetryGuarded_GuardRefusalWritesNoToken(t *testing.T) {
	// The point of the guard is that it runs INSIDE the persistence
	// transaction: a refusal must abort the whole thing, leaving no token
	// row behind and surfacing the sentinel untouched (not buried under the
	// generic "login transaction" wrapper).
	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec(
		"UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?",
		account.ID, tenantID)
	require.NoError(t, err)

	before := countAccountTokens(t, db, account.ID)

	token, err := service.createRefreshTokenWithRetryGuarded(
		context.Background(), account, tenantID, service.schoolMintGuard(account.ID, tenantID))

	require.Error(t, err)
	assert.Nil(t, token)
	assert.ErrorIs(t, err, ErrTenantAccessDenied)
	assert.Equal(t, before, countAccountTokens(t, db, account.ID),
		"a refused mint must not leave a refresh token behind")
}

func countAccountTokens(t *testing.T, db *bun.DB, accountID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", accountID).
		Count(context.Background())
	require.NoError(t, err)
	return count
}
