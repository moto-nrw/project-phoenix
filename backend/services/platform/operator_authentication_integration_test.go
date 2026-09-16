package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Operator login, the MFA-proven token issue, refresh, profile and password
// changes are Identity & Access flows (#3252). The operator routes still
// reach them through the retained OperatorAuthService, which delegates to
// the port the service root binds to the module. These tests drive that
// whole chain and pin the retained error shapes the routes render; the
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

func TestIntegration_OperatorLogin_GatesAndMintsThroughTheModule(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildAuthService(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-login"), testPassword)

	// The operator MFA service is composed: an operator without an
	// enrollment receives the enrollment-scoped token, never a session.
	result, err := service.LoginWithMFAGate(ctx, operator.Email, testPassword, "127.0.0.1", "ua", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, platformSvc.OperatorLoginStatusMFAEnrollmentRequired, result.Status)
	assert.True(t, result.MFAEnrollmentRequired)
	assert.NotEmpty(t, result.AccessToken)
	assert.Empty(t, result.RefreshToken)
	require.NotNil(t, result.Operator)
	assert.Equal(t, operator.ID, result.Operator.ID)
	assert.Equal(t, operator.Email, result.Operator.Email)
	assert.Equal(t, 0, countOperatorSessions(t, db, operator.ID), "no refresh session before MFA enrollment")

	_, err = service.LoginWithMFAGate(ctx, operator.Email, "wrong-password", "127.0.0.1", "ua", "")
	var invalidCredentials *platformSvc.InvalidCredentialsError
	require.ErrorAs(t, err, &invalidCredentials)
	_, err = service.LoginWithMFAGate(ctx, operatorLoginEmail("nobody"), testPassword, "127.0.0.1", "ua", "")
	require.ErrorAs(t, err, &invalidCredentials, "an unknown e-mail is indistinguishable from a wrong password")

	// The MFA-proven mint is the path every second factor ends in.
	access, refresh, err := service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	assert.Equal(t, 1, countOperatorSessions(t, db, operator.ID))
	assert.Equal(t, 1, countOperatorAudit(t, db, operator.ID, "login"), "the mint writes the platform audit row through the Audit owner")

	var lastLogin *time.Time
	err = db.NewSelect().TableExpr("platform.operators").ColumnExpr("last_login").Where("id = ?", operator.ID).Scan(ctx, &lastLogin)
	require.NoError(t, err)
	assert.NotNil(t, lastLogin)

	_, _, err = service.IssueTokensForAuthenticatedOperator(ctx, operator.ID+1_000_000, "127.0.0.1", "ua")
	var notFound *platformSvc.OperatorNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, operator.ID+1_000_000, notFound.OperatorID)
}

func TestIntegration_OperatorRefresh_RotatesAndRevokesReplay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildAuthService(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-refresh"), testPassword)

	_, _, err := service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	predecessor := liveOperatorHandle(t, db, operator.ID)

	access, refresh, err := service.RefreshToken(ctx, operator.ID, predecessor)
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	successor := liveOperatorHandle(t, db, operator.ID)
	assert.NotEqual(t, predecessor, successor, "refresh rotates to a new opaque handle")
	assert.Equal(t, 2, countOperatorSessions(t, db, operator.ID), "the hand-off survives as replay evidence")
	var replacement string
	require.NoError(t, db.NewSelect().TableExpr("platform.operator_refresh_tokens").Column("replacement_token").Where("token = ?", predecessor).Scan(ctx, &replacement))
	assert.Equal(t, successor, replacement)

	var invalidToken *platformSvc.OperatorRefreshTokenInvalidError
	_, _, err = service.RefreshToken(ctx, operator.ID+1_000_000, successor)
	require.ErrorAs(t, err, &invalidToken, "another operator's handle is refused")

	// Without the recovery proof the request cannot prove possession of the
	// rotated predecessor: the replay revokes the family and commits.
	_, _, err = service.RefreshToken(ctx, operator.ID, predecessor)
	require.ErrorAs(t, err, &invalidToken)
	assert.Equal(t, 0, countOperatorSessions(t, db, operator.ID), "replay revocation commits")
	assert.Equal(t, 1, countOperatorAudit(t, db, operator.ID, "token_revoked"))

	changes := jsonColumn(t, db.NewSelect().TableExpr("platform.operator_audit_log").ColumnExpr("changes::text").
		Where("operator_id = ? AND action = ?", operator.ID, "token_revoked"))
	assert.Equal(t, "operator", changes["portal_scope"])
	assert.Equal(t, "replay_detected", changes["reason"])
	assert.EqualValues(t, 2, changes["revoked_token_count"])
	assert.NotEmpty(t, changes["family_fingerprint"])

	_, _, err = service.RefreshToken(ctx, operator.ID, successor)
	require.ErrorAs(t, err, &invalidToken, "the revoked successor is gone too")
}

func TestIntegration_OperatorRefresh_RefusesDeactivatedOperator(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildAuthService(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-inactive"), testPassword)
	_, _, err := service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	handle := liveOperatorHandle(t, db, operator.ID)

	_, err = db.NewUpdate().Table("platform.operators").Set("active = false").Where("id = ?", operator.ID).Exec(ctx)
	require.NoError(t, err)

	var inactive *platformSvc.OperatorInactiveError
	_, _, err = service.RefreshToken(ctx, operator.ID, handle)
	require.ErrorAs(t, err, &inactive)
	assert.Equal(t, operator.ID, inactive.OperatorID)
	assert.Equal(t, 1, countOperatorSessions(t, db, operator.ID), "the refusal rolls back without rotating")
	_, _, err = service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.ErrorAs(t, err, &inactive)

	inactive = nil
	_, err = service.LoginWithMFAGate(ctx, operator.Email, testPassword, "127.0.0.1", "ua", "")
	require.ErrorAs(t, err, &inactive, "valid credentials of a deactivated operator are refused")
	assert.Equal(t, operator.ID, inactive.OperatorID)
}

func TestIntegration_OperatorPasswordChange_RevokesSessions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildAuthService(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperatorWithPassword(t, db, operatorLoginEmail("operator-password"), testPassword)
	_, _, err := service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	_, _, err = service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	handle := liveOperatorHandle(t, db, operator.ID)

	var mismatch *platformSvc.PasswordMismatchError
	require.ErrorAs(t, service.ChangePassword(ctx, operator.ID, "wrong", "ChangedPass789!"), &mismatch)
	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, service.ChangePassword(ctx, operator.ID, testPassword, "weak"), &invalid)
	assert.Equal(t, 2, countOperatorSessions(t, db, operator.ID), "a refused change keeps the sessions")

	require.NoError(t, service.ChangePassword(ctx, operator.ID, testPassword, "ChangedPass789!"))
	assert.Equal(t, 0, countOperatorSessions(t, db, operator.ID), "a password change revokes every operator refresh session")
	var invalidToken *platformSvc.OperatorRefreshTokenInvalidError
	_, _, err = service.RefreshToken(ctx, operator.ID, handle)
	require.ErrorAs(t, err, &invalidToken)
	assert.Equal(t, 2, countOperatorAudit(t, db, operator.ID, "token_revoked"), "one revocation entry per family")

	result, err := service.LoginWithMFAGate(ctx, operator.Email, "ChangedPass789!", "127.0.0.1", "ua", "")
	require.NoError(t, err, "the new password verifies")
	assert.Equal(t, platformSvc.OperatorLoginStatusMFAEnrollmentRequired, result.Status)
}

func TestIntegration_OperatorUpdateProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildAuthService(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperator(t, db)

	updated, err := service.UpdateProfile(ctx, operator.ID, "  Renamed Operator  ")
	require.NoError(t, err)
	assert.Equal(t, "Renamed Operator", updated.DisplayName)
	assert.Equal(t, operator.Email, updated.Email)

	var invalid *platformSvc.InvalidDataError
	_, err = service.UpdateProfile(ctx, operator.ID, "   ")
	require.ErrorAs(t, err, &invalid)
	var notFound *platformSvc.OperatorNotFoundError
	_, err = service.UpdateProfile(ctx, operator.ID+1_000_000, "Ghost")
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, operator.ID+1_000_000, notFound.OperatorID)
}
