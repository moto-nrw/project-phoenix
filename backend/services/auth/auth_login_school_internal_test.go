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
	"sync"
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
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

	service = setupInternalAuthService(t, db)
	tenantID = testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	unique := testpkg.UniqueSuffix()
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

	service = setupInternalAuthService(t, db)

	tenantID, _ = testpkg.CreateTestTenant(t, db)
	unique := testpkg.UniqueSuffix()
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
// phoenix_admin transaction, which is what its FOR SHARE reads require. It
// returns the claims payload the guard assembled there — the token is built
// from that, not from anything loaded before the transaction.
func runMintGuard(t *testing.T, service *Service, db *bun.DB, accountID, tenantID int64) (*accountMetadata, error) {
	t.Helper()
	var claims *accountMetadata
	guard := service.schoolMintGuard(accountID, tenantID, &claims)
	var guardErr error
	txErr := tenant.WithAdminTx(context.Background(), db, func(ctx context.Context, _ bun.Tx) error {
		// nil account: schoolMintGuard re-reads and locks the row itself,
		// exactly as it does behind persistTokenInTransaction.
		guardErr = guard(ctx, nil)
		return nil
	})
	require.NoError(t, txErr)
	return claims, guardErr
}

func TestSchoolMintGuard_LiveSessionPasses(t *testing.T) {
	t.Parallel()

	service, db, account, tenantID := newMintGuardFixture(t)
	claims, err := runMintGuard(t, service, db, account.ID, tenantID)
	require.NoError(t, err)
	// The guard is also where the JWT payload comes from — a nil payload on a
	// passing guard would mint a token with no roles and no permissions.
	require.NotNil(t, claims)
	assert.Equal(t, tenant.ScopeSchool, claims.scope)
	assert.Equal(t, tenantID, claims.tenantID)
}

func TestSchoolMintGuard_ClaimsDropPermissionRevokedMidFlight(t *testing.T) {
	t.Parallel()

	// The finding this pins: the mint paths used to build their claims from
	// metadata loaded BEFORE the guarded transaction. The guard re-checked
	// membership and portal role, but not what the account may DO — so a
	// permission revoked while a login (or an MFA exchange, which can sit
	// minutes in between) was in flight travelled into the access token and
	// stayed valid until it expired.
	service, db, account, tenantID := newMintGuardFixture(t)

	role := testpkg.CreateTestRoleForTenant(t, db, "school-extra", tenantID)
	permission := testpkg.CreateTestPermission(t, db, "school-extra", "school_extra", "read")
	t.Cleanup(func() { testpkg.CleanupRoleRecords(t, db, role.ID) })
	t.Cleanup(func() { testpkg.CleanupPermissionRecords(t, db, permission.ID) })

	rolePermission := &authModels.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
	_, err := db.NewInsert().Model(rolePermission).ModelTableExpr(`auth.role_permissions`).Exec(context.Background())
	require.NoError(t, err)

	extraRole := &authModels.AccountRole{AccountID: account.ID, RoleID: role.ID}
	extraRole.SetTenantID(tenantID)
	_, err = db.NewInsert().Model(extraRole).ModelTableExpr(`auth.account_roles`).Exec(context.Background())
	require.NoError(t, err)

	// What the entry point sees, in its own already-committed transaction.
	// Permissions travel into the JWT as "resource:action", not by name.
	permissionClaim := permission.GetFullName()
	before, err := service.loadSchoolMetadataForTenant(context.Background(), account, tenantID)
	require.NoError(t, err)
	require.Contains(t, before.permissionStrs, permissionClaim,
		"fixture check: the extra permission must be visible before the revocation")

	// The revocation lands between that load and the mint. The lehrkraft role
	// stays, so the guard still authorizes the session — only the permission
	// set changed, which is exactly the case the old code missed.
	_, err = db.Exec(
		"DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ? AND role_id = ?",
		account.ID, tenantID, role.ID)
	require.NoError(t, err)

	claims, err := runMintGuard(t, service, db, account.ID, tenantID)

	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.NotContains(t, claims.permissionStrs, permissionClaim,
		"claims must come from inside the guard, so a permission revoked mid-flight never reaches the token")
}

func TestSchoolMintGuard_RefusesRevokedMembership(t *testing.T) {
	t.Parallel()

	// Membership is revoked by flipping account_tenants.status. Everything
	// else about the account stays valid, which is exactly why the guard has
	// to look at this row rather than trust the earlier resolution.
	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec(
		"UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?",
		account.ID, tenantID)
	require.NoError(t, err)

	_, guardErr := runMintGuard(t, service, db, account.ID, tenantID)
	assert.ErrorIs(t, guardErr, ErrTenantAccessDenied)
}

func TestSchoolMintGuard_RefusesRevokedPortalRole(t *testing.T) {
	t.Parallel()

	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec(
		"DELETE FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?",
		account.ID, tenantID)
	require.NoError(t, err)

	_, guardErr := runMintGuard(t, service, db, account.ID, tenantID)
	assert.ErrorIs(t, guardErr, ErrAccountNoSchoolPortalRole)
}

func TestSchoolMintGuard_RefusesDeactivatedSchool(t *testing.T) {
	t.Parallel()

	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec("UPDATE platform.schools SET active = false WHERE id = ?", tenantID)
	require.NoError(t, err)

	_, guardErr := runMintGuard(t, service, db, account.ID, tenantID)
	assert.ErrorIs(t, guardErr, ErrTenantNotFound)
}

func TestSchoolMintGuard_RefusesSoftDeletedSchool(t *testing.T) {
	t.Parallel()

	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec("UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?", tenantID)
	require.NoError(t, err)

	_, guardErr := runMintGuard(t, service, db, account.ID, tenantID)
	assert.ErrorIs(t, guardErr, ErrTenantNotFound)
}

func TestCreateRefreshTokenWithRetryGuarded_GuardRefusalWritesNoToken(t *testing.T) {
	t.Parallel()

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
		context.Background(), account, tenantID, tenant.ScopeSchool, service.schoolMintGuard(account.ID, tenantID, new(*accountMetadata)))

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

// --- schoolClaimsPayloadInTx -------------------------------------------------------

// insertPersonForAccountAtTenant links a person row at ONE school to an
// account. Accounts that work at two schools have one row per school, which is
// what makes the tenant the lookup runs under decide which name a token gets.
func insertPersonForAccountAtTenant(t *testing.T, db *bun.DB, tenantID, accountID int64, firstName, lastName string) {
	t.Helper()
	person := &userModels.Person{
		FirstName: firstName,
		LastName:  lastName,
		AccountID: &accountID,
	}
	person.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(context.Background()))
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM users.persons WHERE id = ?`, person.ID)
	})
}

func TestSchoolClaimsPayload_RunsNoLivenessGate(t *testing.T) {
	t.Parallel()

	// The refresh path assembles its claims inside the rotation transaction,
	// where schoolRefreshMintGuard has just checked liveness under the account
	// lock. The claims loader must therefore NOT gate again: a second opinion
	// taken microseconds later can only differ by racing, and the mint it would
	// abort was already authorized. loadSchoolMetadataForTenant (every pre-mint
	// caller) must still refuse the same school.
	service, db, account, tenantID := newMintGuardFixture(t)
	_, err := db.Exec("UPDATE platform.schools SET active = false WHERE id = ?", tenantID)
	require.NoError(t, err)

	gated, err := service.loadSchoolMetadataForTenant(context.Background(), account, tenantID)
	require.Error(t, err, "the pre-mint path must refuse a deactivated school")
	assert.Nil(t, gated)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)

	var payload *accountMetadata
	require.NoError(t, tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
		payload, err = service.schoolClaimsPayloadInTx(adminCtx, account, tenantID)
		return err
	}), "the in-rotation claims load must not re-gate: the guard just authorized this mint")
	require.NotNil(t, payload)
	assert.Equal(t, tenant.ScopeSchool, payload.scope)
	assert.Equal(t, tenantID, payload.tenantID)
}

// --- loadAccountMetadataForTenant: person names ------------------------------------

func TestLoadAccountMetadataForTenant_PersonNameComesFromTargetSchool(t *testing.T) {
	t.Parallel()

	// A school switch runs inside the request of the school being LEFT, so
	// the ambient context names the source school. The person lookup is
	// tenant-filtered, so without pinning the target the new token carried
	// the name from the old school.
	service, db, account, sourceTenantID := newMintGuardFixture(t)

	targetTenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, targetTenantID) })
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)

	insertPersonForAccountAtTenant(t, db, sourceTenantID, account.ID, "Quelle", "Schule")
	insertPersonForAccountAtTenant(t, db, targetTenantID, account.ID, "Ziel", "Schule")

	// Ambient context = the school being left, exactly as during a switch.
	ctx := tenant.WithTenantID(context.Background(), sourceTenantID)

	metadata, err := service.loadAccountMetadataForTenant(ctx, account, targetTenantID)

	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, "Ziel", metadata.firstName,
		"the token minted for the target school must carry that school's person name")
	assert.Equal(t, targetTenantID, metadata.tenantID)
}

func TestLoadAccountMetadataForTenant_NoPersonAtTargetFallsBack(t *testing.T) {
	t.Parallel()

	// Not every account has a person row at the school it is being minted
	// for — an org-scope Träger user reaches a school through organization
	// membership while their person row stays at their home school. Those
	// tokens must keep their name rather than losing it to the new filter.
	service, db, account, homeTenantID := newMintGuardFixture(t)

	targetTenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, targetTenantID) })
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)

	insertPersonForAccountAtTenant(t, db, homeTenantID, account.ID, "Traeger", "Buero")

	metadata, err := service.loadAccountMetadataForTenant(context.Background(), account, targetTenantID)

	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, "Traeger", metadata.firstName,
		"with no person row at the target school the lookup must fall back, not blank the name")
}

// failingPersonRepo fails the account → person lookup and delegates the rest.
type failingPersonRepo struct {
	userModels.PersonRepository
	err error
}

func (r failingPersonRepo) FindByAccountID(context.Context, int64) (*userModels.Person, error) {
	return nil, r.err
}

func TestLoadAccountMetadataForTenant_PersonLookupError_Propagates(t *testing.T) {
	t.Parallel()

	// A failed person lookup used to be indistinguishable from "this account
	// has no person row here", which sent the mint into the cross-school
	// fallback on the strength of a DB error — and could stamp another
	// school's name into the token. It has to fail the mint instead.
	service, _, account, tenantID := newMintGuardFixture(t)

	lookupErr := errors.New("person lookup unavailable")
	service.repos.Person = failingPersonRepo{PersonRepository: service.repos.Person, err: lookupErr}

	metadata, err := service.loadAccountMetadataForTenant(context.Background(), account, tenantID)

	require.Error(t, err)
	assert.ErrorIs(t, err, lookupErr, "a failed person lookup must surface, not turn into a nameless token")
	assert.Nil(t, metadata)
}

func TestLoadAccountMetadataForTenant_AmbiguousNameAcrossSchools_YieldsNoName(t *testing.T) {
	t.Parallel()

	// No person row at the target school, but two OTHER schools the account is
	// mapped to disagree about the name. The old unscoped fallback had no
	// ORDER BY and took whichever row the database returned first — a coin
	// flip between two schools' data. Refusing to guess is the only correct
	// answer here; the name is cosmetic, picking the wrong school's is not.
	service, db, account, homeTenantID := newMintGuardFixture(t)

	secondTenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, secondTenantID) })
	testpkg.MapAccountToTenant(t, db, account.ID, secondTenantID)

	targetTenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, targetTenantID) })
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)

	insertPersonForAccountAtTenant(t, db, homeTenantID, account.ID, "Erste", "Schule")
	insertPersonForAccountAtTenant(t, db, secondTenantID, account.ID, "Zweite", "Schule")

	metadata, err := service.loadAccountMetadataForTenant(context.Background(), account, targetTenantID)

	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Empty(t, metadata.firstName, "an ambiguous name must be dropped, never guessed")
	assert.Empty(t, metadata.lastName)
}

func TestLoadAccountMetadataForTenant_FallbackIgnoresUnmappedSchools(t *testing.T) {
	t.Parallel()

	// The fallback consults only schools the account is ACTIVELY mapped to.
	// A person row at a school the account has no mapping to must not reach
	// the token — that row belongs to someone else's tenant boundary.
	service, db, account, homeTenantID := newMintGuardFixture(t)

	strangerTenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, strangerTenantID) })
	insertPersonForAccountAtTenant(t, db, strangerTenantID, account.ID, "Fremde", "Schule")

	targetTenantID, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, targetTenantID) })
	testpkg.MapAccountToTenant(t, db, account.ID, targetTenantID)

	insertPersonForAccountAtTenant(t, db, homeTenantID, account.ID, "Eigene", "Schule")

	metadata, err := service.loadAccountMetadataForTenant(context.Background(), account, targetTenantID)

	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, "Eigene", metadata.firstName,
		"only the mapped school's person row may fill the name")
}

// schoolTestIP / schoolTestUserAgent stand in for request metadata on the
// non-password mint paths; both must be non-empty or the audit write is
// skipped and the assertions below would pass vacuously.
const (
	schoolTestIP        = "203.0.113.11"
	schoolTestUserAgent = "school-portal-internal-test"
)

// --- schoolMintGuard: MFA re-check ---------------------------------------------------

// runMintGuardWithOptions is runMintGuard for the guard variants that carry
// extra in-transaction checks.
func runMintGuardWithOptions(t *testing.T, service *Service, db *bun.DB, accountID, tenantID int64, opts ...schoolMintOption) (*accountMetadata, error) {
	t.Helper()
	var claims *accountMetadata
	guard := service.schoolMintGuard(accountID, tenantID, &claims, opts...)
	var guardErr error
	txErr := tenant.WithAdminTx(context.Background(), db, func(ctx context.Context, _ bun.Tx) error {
		guardErr = guard(ctx, nil)
		return nil
	})
	require.NoError(t, txErr)
	return claims, guardErr
}

// assignAdminRoleAtTenant grants the seeded admin system role at the tenant.
// It has to be that role and not a fixture one: MFAPolicy's `required_admins`
// predicate matches on the literal name "admin", and CreateTestRoleForTenant
// uniquifies every name it is given.
func assignAdminRoleAtTenant(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	assignment := &authModels.AccountRole{AccountID: accountID, RoleID: adminSystemRoleID(t, db)}
	assignment.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(assignment).ModelTableExpr(`auth.account_roles`).Exec(context.Background())
	require.NoError(t, err)
	// CleanupAuthFixtures removes auth.account_roles by account_id; the system
	// role row itself is seeded and must survive.
}

// adminSystemRoleID resolves the seeded admin system role.
func adminSystemRoleID(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", "admin").
		Where("is_system = TRUE").
		Scan(context.Background(), &roleID), "seeded admin system role must exist")
	return roleID
}

// staticPolicyResolver is the resolver a mint guard gets in tests that only
// care about the verdict. It records the context it was called with so a test
// can assert WHERE the policy was read — inside the mint transaction, not
// before it.
func staticPolicyResolver(policy MFAPolicy, calls *[]context.Context) mfaPolicyResolver {
	return func(ctx context.Context) (MFAPolicy, error) {
		if calls != nil {
			*calls = append(*calls, ctx)
		}
		return policy, nil
	}
}

func TestSchoolMintGuard_MFARequirementAppearingMidLogin_AbortsMint(t *testing.T) {
	t.Parallel()

	// The finding this pins: the MFA gate ran BEFORE the mint transaction and
	// was never revisited. Under `required_admins` an admin role granted after
	// the gate said "not required" — and before the token was written — handed
	// out an admin session that never saw a second factor.
	service, db, account, tenantID := newMintGuardFixture(t)
	assignAdminRoleAtTenant(t, db, account.ID, tenantID)

	claims, guardErr := runMintGuardWithOptions(t, service, db, account.ID, tenantID,
		withMFAGateRecheck(staticPolicyResolver(MFAPolicyForMode(configModels.MFAModeRequiredAdmins), nil)))

	require.ErrorIs(t, guardErr, errSchoolMFARequiredAtMint)
	assert.Nil(t, claims, "an aborted mint must not publish claims")
}

func TestSchoolMintGuard_MFARecheckPassesForNonAdmin(t *testing.T) {
	t.Parallel()

	// The same policy against a plain Lehrkraft: nothing changed under the
	// lock, so the mint proceeds. Without this the re-check would refuse every
	// login at a `required_admins` school.
	service, db, account, tenantID := newMintGuardFixture(t)

	claims, guardErr := runMintGuardWithOptions(t, service, db, account.ID, tenantID,
		withMFAGateRecheck(staticPolicyResolver(MFAPolicyForMode(configModels.MFAModeRequiredAdmins), nil)))

	require.NoError(t, guardErr)
	require.NotNil(t, claims)
	assert.Equal(t, tenant.ScopeSchool, claims.scope)
}

func TestSchoolMintGuard_MFAPolicyIsReadInsideTheMintTransaction(t *testing.T) {
	t.Parallel()

	// A policy resolved BEFORE the transaction is the stale input the re-check
	// exists to replace: a school switching security.mfa_mode from off to
	// required mid-login would otherwise be evaluated against "off". The guard
	// must therefore RESOLVE, not merely re-apply — and it must do so on the
	// transaction whose locks make the answer binding.
	service, db, account, tenantID := newMintGuardFixture(t)

	var seen []context.Context
	claims, guardErr := runMintGuardWithOptions(t, service, db, account.ID, tenantID,
		withMFAGateRecheck(staticPolicyResolver(MFAPolicyForMode(configModels.MFAModeOff), &seen)))

	require.NoError(t, guardErr)
	require.NotNil(t, claims)
	require.Len(t, seen, 1, "the policy must be resolved exactly once per mint")
	tx, ok := modelBase.TxFromContext(seen[0])
	assert.True(t, ok && tx != nil,
		"the policy must be resolved on the mint transaction, not from a pre-transaction snapshot")
}

func TestSchoolMintGuard_UnreadableMFAPolicyFailsClosed(t *testing.T) {
	t.Parallel()

	// Fail-closed, exactly like the pre-transaction gate: if the policy cannot
	// be read we do not know whether a second factor is owed, so this mint is
	// refused with the 503 sentinel instead of proceeding as "not required".
	service, db, account, tenantID := newMintGuardFixture(t)

	claims, guardErr := runMintGuardWithOptions(t, service, db, account.ID, tenantID,
		withMFAGateRecheck(func(context.Context) (MFAPolicy, error) {
			return MFAPolicy{}, errors.New("settings unavailable")
		}))

	require.ErrorIs(t, guardErr, ErrMFAStatusUnavailable)
	assert.Nil(t, claims, "an aborted mint must not publish claims")
}

func TestSchoolMintGuard_WithoutRecheckAdminRoleStillMints(t *testing.T) {
	t.Parallel()

	// The MFA exchange and the school switch pass no policy: their second
	// factor is settled (or, for the switch, was never gated here). An admin
	// role must not turn those mints into a refusal.
	service, db, account, tenantID := newMintGuardFixture(t)
	assignAdminRoleAtTenant(t, db, account.ID, tenantID)

	claims, guardErr := runMintGuard(t, service, db, account.ID, tenantID)

	require.NoError(t, guardErr)
	require.NotNil(t, claims)
}

// --- account lookup on the non-password mint paths -----------------------------------

// failingAccountRepo fails FindByID and delegates everything else — including
// FindByIDForUpdate, which the mint guard uses — to the real repository.
type failingAccountRepo struct {
	authModels.AccountRepository
	err error
}

func (r failingAccountRepo) FindByID(context.Context, any) (*authModels.Account, error) {
	return nil, r.err
}

func TestIssueSchoolTokens_AccountLookupError_IsNotACredentialFailure(t *testing.T) {
	t.Parallel()

	// A dropped connection while loading the account is not "wrong
	// credentials". Collapsing the two told a Lehrkraft who had just entered a
	// correct email code that their login was invalid (401) and hid a database
	// outage from every alert that watches 5xx.
	service, _, account, tenantID := newMintGuardFixture(t)
	dbErr := errors.New("connection reset")
	service.repos.Account = failingAccountRepo{AccountRepository: service.repos.Account, err: dbErr}

	_, _, err := service.IssueSchoolTokensForAuthenticatedAccount(
		context.Background(), account.ID, tenantID, schoolTestIP, schoolTestUserAgent)

	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, dbErr)
	assert.NotErrorIs(t, authErr.Err, ErrAccountNotFound,
		"a transient DB failure must not be reported as an unknown account")
}

func TestIssueSchoolTokens_AccountMissing_ReportsNotFound(t *testing.T) {
	t.Parallel()

	// The genuine no-such-row case keeps its sentinel — that is the one the
	// handler is right to answer 401 to.
	service, _, account, tenantID := newMintGuardFixture(t)
	service.repos.Account = failingAccountRepo{AccountRepository: service.repos.Account, err: sql.ErrNoRows}

	_, _, err := service.IssueSchoolTokensForAuthenticatedAccount(
		context.Background(), account.ID, tenantID, schoolTestIP, schoolTestUserAgent)

	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrAccountNotFound)
}

func TestSwitchSchool_AccountLookupError_IsNotACredentialFailure(t *testing.T) {
	t.Parallel()

	service, _, account, tenantID := newMintGuardFixture(t)
	dbErr := errors.New("connection reset")
	service.repos.Account = failingAccountRepo{AccountRepository: service.repos.Account, err: dbErr}

	_, _, err := service.SwitchSchool(
		context.Background(), account.ID, fmt.Sprintf("tenant-%d", tenantID), schoolTestIP, schoolTestUserAgent)

	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, dbErr)
	assert.NotErrorIs(t, authErr.Err, ErrAccountNotFound)
}

// --- the MFA policy lock -------------------------------------------------------

// lockOrderRecorder collects the order in which a mint takes its locks, so a
// test can assert WHICH lock came first rather than only that both were taken.
type lockOrderRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *lockOrderRecorder) record(event string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *lockOrderRecorder) taken() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// recordingAccountLockRepo notes the account row lock and delegates.
type recordingAccountLockRepo struct {
	authModels.AccountRepository
	recorder *lockOrderRecorder
}

func (r recordingAccountLockRepo) FindByIDForUpdate(ctx context.Context, id int64) (*authModels.Account, error) {
	r.recorder.record("account")
	return r.AccountRepository.FindByIDForUpdate(ctx, id)
}

// withRecordedPolicyLock wires a settings double that records (or fails) the
// shared MFA-policy lock the mint takes.
func withRecordedPolicyLock(t *testing.T, service *Service, recorder *lockOrderRecorder, lockErr error) {
	t.Helper()
	service.settings = &configtest.Mock{
		LockMFAPolicySharedForTenantFn: func(ctx context.Context, _ int64) error {
			recorder.record("policy")
			// The real helper skips silently without an ambient transaction, so
			// a mint that took the lock on the wrong context would look locked
			// and be unprotected. Pin that it is the mint's own transaction.
			tx, ok := modelBase.TxFromContext(ctx)
			assert.True(t, ok && tx != nil,
				"the policy lock must be taken on the mint transaction")
			return lockErr
		},
	}
	service.repos.Account = recordingAccountLockRepo{
		AccountRepository: service.repos.Account,
		recorder:          recorder,
	}
}

func TestSchoolMintGuard_TakesMFAPolicyLockBeforeTheAccountRow(t *testing.T) {
	t.Parallel()

	// Re-reading the policy inside the mint transaction orders nothing: an
	// admin enabling MFA can commit right after that read and the token still
	// goes out unchallenged. The mint therefore pins security.mfa_mode for the
	// whole transaction — and it has to do so BEFORE it locks auth.accounts.
	// Writing config.setting_values takes a foreign-key lock on the writing
	// admin's own account row, so the reverse order deadlocks an admin who
	// flips mfa_mode while their own school login is in flight.
	service, db, account, tenantID := newMintGuardFixture(t)
	recorder := &lockOrderRecorder{}
	withRecordedPolicyLock(t, service, recorder, nil)

	claims, guardErr := runMintGuardWithOptions(t, service, db, account.ID, tenantID,
		withMFAGateRecheck(staticPolicyResolver(MFAPolicyForMode(configModels.MFAModeOff), nil)))

	require.NoError(t, guardErr)
	require.NotNil(t, claims)
	assert.Equal(t, []string{"policy", "account"}, recorder.taken(),
		"the policy lock must be the first lock of the mint transaction")
}

func TestSchoolMintGuard_UnavailableMFAPolicyLockFailsClosed(t *testing.T) {
	t.Parallel()

	// A lock we could not take means the policy read that follows it is
	// unordered against a concurrent write — exactly the state this closes.
	// Refuse the mint (503) instead of deciding the gate on it.
	service, db, account, tenantID := newMintGuardFixture(t)
	recorder := &lockOrderRecorder{}
	withRecordedPolicyLock(t, service, recorder, errors.New("lock unavailable"))

	claims, guardErr := runMintGuardWithOptions(t, service, db, account.ID, tenantID,
		withMFAGateRecheck(staticPolicyResolver(MFAPolicyForMode(configModels.MFAModeOff), nil)))

	require.ErrorIs(t, guardErr, ErrMFAStatusUnavailable)
	assert.Nil(t, claims, "an aborted mint must not publish claims")
	assert.Equal(t, []string{"policy"}, recorder.taken(),
		"a failed policy lock must abort before the mint touches any row")
}

func TestSchoolMintGuard_WithoutRecheckTakesNoPolicyLock(t *testing.T) {
	t.Parallel()

	// The MFA exchange and the school switch have their second factor settled
	// (or never gated it here). Taking the policy lock there would serialize
	// those mints against every mfa_mode write for nothing.
	service, db, account, tenantID := newMintGuardFixture(t)
	recorder := &lockOrderRecorder{}
	withRecordedPolicyLock(t, service, recorder, nil)

	claims, guardErr := runMintGuard(t, service, db, account.ID, tenantID)

	require.NoError(t, guardErr)
	require.NotNil(t, claims)
	assert.Equal(t, []string{"account"}, recorder.taken())
}
