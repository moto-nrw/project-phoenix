package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Operator login, the MFA-proven token issue, refresh, profile and password
// changes run through the composed Identity & Access module (#3252); the
// rotation-recovery decisions are covered by the module's own tests.

func operatorLoginEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@test.local", prefix, time.Now().UnixNano())
}

func liveOperatorHandle(t *testing.T, db *bun.DB, operatorID int64) string {
	t.Helper()
	var token string
	err := db.NewSelect().
		TableExpr("platform.operator_refresh_tokens").
		Column("token").
		Where("operator_id = ?", operatorID).
		Where("rotated_at IS NULL").
		Limit(1).
		Scan(context.Background(), &token)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	return token
}

func countOperatorSessions(t *testing.T, db *bun.DB, operatorID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr("platform.operator_refresh_tokens").
		Where("operator_id = ?", operatorID).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

func countOperatorAudit(t *testing.T, db *bun.DB, operatorID int64, action string) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr("platform.operator_audit_log").
		Where("operator_id = ? AND action = ?", operatorID, action).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

// jsonColumn reads one JSON column of the newest matching row as text and
// decodes it; bun would treat a map scan target as a column map.
func jsonColumn(t *testing.T, query *bun.SelectQuery) map[string]any {
	t.Helper()
	var raw string
	require.NoError(t, query.OrderExpr("id DESC").Limit(1).Scan(context.Background(), &raw))
	var values map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &values))
	return values
}

func TestIntegration_OperatorLogin_GatesAndMintsThroughTheModule(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-login"), testPassword)

	// The operator MFA service is composed: an operator without an
	// enrollment receives the enrollment-scoped token, never a session.
	result, err := identity.LoginOperatorWithMFAGate(ctx, operator.Email, testPassword, "127.0.0.1", "ua", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status)
	assert.True(t, result.MFAEnrollmentRequired)
	assert.NotEmpty(t, result.AccessToken)
	assert.Empty(t, result.RefreshToken)
	require.NotNil(t, result.Operator)
	assert.Equal(t, operator.ID, result.Operator.ID)
	assert.Equal(t, operator.Email, result.Operator.Email)
	assert.Equal(t, 0, countOperatorSessions(t, db, operator.ID), "no refresh session before MFA enrollment")

	_, err = identity.LoginOperatorWithMFAGate(ctx, operator.Email, "wrong-password", "127.0.0.1", "ua", "")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvalidCredentials)
	_, err = identity.LoginOperatorWithMFAGate(ctx, operatorLoginEmail("nobody"), testPassword, "127.0.0.1", "ua", "")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInvalidCredentials, "an unknown e-mail is indistinguishable from a wrong password")

	// The MFA-proven mint is the path every second factor ends in.
	access, refresh, err := identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	assert.Equal(t, 1, countOperatorSessions(t, db, operator.ID))
	assert.Equal(t, 1, countOperatorAudit(t, db, operator.ID, "login"), "the mint writes the platform audit row through the Audit owner")

	var lastLogin *time.Time
	err = db.NewSelect().TableExpr("platform.operators").ColumnExpr("last_login").Where("id = ?", operator.ID).Scan(ctx, &lastLogin)
	require.NoError(t, err)
	assert.NotNil(t, lastLogin)

	_, _, err = identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID+1_000_000, "127.0.0.1", "ua")
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
}

func TestIntegration_OperatorRefresh_RotatesAndRevokesReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-refresh"), testPassword)

	_, _, err := identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	predecessor := liveOperatorHandle(t, db, operator.ID)

	access, refresh, err := identity.RefreshOperatorToken(ctx, operator.ID, predecessor)
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	successor := liveOperatorHandle(t, db, operator.ID)
	assert.NotEqual(t, predecessor, successor, "refresh rotates to a new opaque handle")
	assert.Equal(t, 2, countOperatorSessions(t, db, operator.ID), "the hand-off survives as replay evidence")
	var replacement string
	require.NoError(t, db.NewSelect().TableExpr("platform.operator_refresh_tokens").Column("replacement_token").Where("token = ?", predecessor).Scan(ctx, &replacement))
	assert.Equal(t, successor, replacement)

	_, _, err = identity.RefreshOperatorToken(ctx, operator.ID+1_000_000, successor)
	require.ErrorIs(t, err, identityaccess.ErrOperatorRefreshTokenInvalid, "another operator's handle is refused")

	// Without the recovery proof the request cannot prove possession of the
	// rotated predecessor: the replay revokes the family and commits.
	_, _, err = identity.RefreshOperatorToken(ctx, operator.ID, predecessor)
	require.ErrorIs(t, err, identityaccess.ErrOperatorRefreshTokenInvalid)
	assert.Equal(t, 0, countOperatorSessions(t, db, operator.ID), "replay revocation commits")
	assert.Equal(t, 1, countOperatorAudit(t, db, operator.ID, "token_revoked"))

	changes := jsonColumn(t, db.NewSelect().TableExpr("platform.operator_audit_log").ColumnExpr("changes::text").
		Where("operator_id = ? AND action = ?", operator.ID, "token_revoked"))
	assert.Equal(t, "operator", changes["portal_scope"])
	assert.Equal(t, "replay_detected", changes["reason"])
	assert.EqualValues(t, 2, changes["revoked_token_count"])
	assert.NotEmpty(t, changes["family_fingerprint"])

	_, _, err = identity.RefreshOperatorToken(ctx, operator.ID, successor)
	require.ErrorIs(t, err, identityaccess.ErrOperatorRefreshTokenInvalid, "the revoked successor is gone too")
}

func TestIntegration_OperatorRefresh_RefusesDeactivatedOperator(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-inactive"), testPassword)
	_, _, err := identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	handle := liveOperatorHandle(t, db, operator.ID)

	_, err = db.NewUpdate().Table("platform.operators").Set("active = false").Where("id = ?", operator.ID).Exec(ctx)
	require.NoError(t, err)

	_, _, err = identity.RefreshOperatorToken(ctx, operator.ID, handle)
	require.ErrorIs(t, err, identityaccess.ErrOperatorInactive)
	assert.Equal(t, 1, countOperatorSessions(t, db, operator.ID), "the refusal rolls back without rotating")
	_, _, err = identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInactive)
	_, err = identity.LoginOperatorWithMFAGate(ctx, operator.Email, testPassword, "127.0.0.1", "ua", "")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInactive, "valid credentials of a deactivated operator are refused")
}

func TestIntegration_OperatorPasswordChange_RevokesSessions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-password"), testPassword)
	_, _, err := identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	_, _, err = identity.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	handle := liveOperatorHandle(t, db, operator.ID)

	require.ErrorIs(t, identity.ChangeOperatorPassword(ctx, operator.ID, "wrong", "ChangedPass789!"), identityaccess.ErrOperatorPasswordMismatch)
	var invalid *identityaccess.InvalidInputError
	require.ErrorAs(t, identity.ChangeOperatorPassword(ctx, operator.ID, testPassword, "weak"), &invalid)
	assert.Equal(t, 2, countOperatorSessions(t, db, operator.ID), "a refused change keeps the sessions")

	require.NoError(t, identity.ChangeOperatorPassword(ctx, operator.ID, testPassword, "ChangedPass789!"))
	assert.Equal(t, 0, countOperatorSessions(t, db, operator.ID), "a password change revokes every operator refresh session")
	_, _, err = identity.RefreshOperatorToken(ctx, operator.ID, handle)
	require.ErrorIs(t, err, identityaccess.ErrOperatorRefreshTokenInvalid)
	assert.Equal(t, 2, countOperatorAudit(t, db, operator.ID, "token_revoked"), "one revocation entry per family")

	result, err := identity.LoginOperatorWithMFAGate(ctx, operator.Email, "ChangedPass789!", "127.0.0.1", "ua", "")
	require.NoError(t, err, "the new password verifies")
	assert.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status)
}

func TestIntegration_OperatorUpdateProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := buildOperatorIdentity(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperator(t, db)

	updated, err := identity.UpdateOperatorProfile(ctx, operator.ID, "  Renamed Operator  ")
	require.NoError(t, err)
	assert.Equal(t, "Renamed Operator", updated.DisplayName)
	assert.Equal(t, operator.Email, updated.Email)

	var invalid *identityaccess.InvalidInputError
	_, err = identity.UpdateOperatorProfile(ctx, operator.ID, "   ")
	require.ErrorAs(t, err, &invalid)
	_, err = identity.UpdateOperatorProfile(ctx, operator.ID+1_000_000, "Ghost")
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)
}

// Operator login, refresh, profile and password changes and the
// operator-led school access of accounts are Identity & Access flows the
// service root composes over the retained owners (#3252). The integration
// tests below drive them through the composed public module.

var testClientIP = net.IPv4(127, 0, 0, 1)

// testSchoolID is the school (= tenant) the test owns.
func testSchoolID(tb testing.TB) int64 { return testpkg.Tenant(tb) }

// buildOperatorIdentity composes the service root and returns the public
// Identity & Access module the operator routes call.
func buildOperatorIdentity(t *testing.T, db *bun.DB) *identityaccess.Module {
	t.Helper()
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTestsWithConfig(repoFactory, db, slog.Default(), authTestFactoryConfig(false))
	require.NoError(t, err, "Failed to create service factory")
	require.NoError(t, serviceFactory.SetTenantRuntime(testpkg.TenantRuntime(t, db)))
	module := serviceFactory.AccountAuthentication()
	require.NotNil(t, module, "the factory composes the operator flows on the identity module")
	return module
}

// accessGrant carries the grant fields the cases vary; the account, school,
// operator and address are the call's own arguments.
type accessGrant struct {
	RoleID    int64
	FirstName string
	LastName  string
	Position  string
}

// operatorAccountAccess keeps the cases in the call shape of the operator
// school-access routes: account and school first, then the acting operator
// and the request address.
type operatorAccountAccess struct {
	module identityaccess.OperatorAccountAccess
}

func buildOperatorAccountAccess(t *testing.T, db *bun.DB) operatorAccountAccess {
	t.Helper()
	return operatorAccountAccess{module: buildOperatorIdentity(t, db)}
}

func (a operatorAccountAccess) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]identityaccess.AccountTenantAccess, error) {
	return a.module.ListAccountTenantAccess(ctx, accountID)
}

func (a operatorAccountAccess) GrantAccountTenantAccess(ctx context.Context, accountID, schoolID int64, grant accessGrant, operatorID int64, clientIP net.IP) ([]identityaccess.AccountTenantAccess, error) {
	return a.module.GrantAccountTenantAccess(ctx, identityaccess.GrantAccountTenantAccess{
		AccountID: accountID, SchoolID: schoolID, RoleID: grant.RoleID,
		FirstName: grant.FirstName, LastName: grant.LastName, Position: grant.Position,
		OperatorID: operatorID, ClientIP: clientIP.String(),
	})
}

func (a operatorAccountAccess) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP net.IP) ([]identityaccess.AccountTenantAccess, error) {
	return a.module.UpdateAccountTenantRole(ctx, accountID, schoolID, roleID, operatorID, clientIP.String())
}

func (a operatorAccountAccess) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP net.IP) ([]identityaccess.AccountTenantAccess, error) {
	return a.module.RevokeAccountTenantAccess(ctx, accountID, schoolID, operatorID, clientIP.String())
}
