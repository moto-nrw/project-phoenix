// Package platform_test contains integration tests for operator email change
// that require a real database connection.
package behavior_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const operatorTestPassword = "SecurePass123!" //nolint:gosec // pragma: allowlist secret

// fastArgon2Params returns cheap Argon2id params for tests so hashing takes ~1ms
// instead of ~200ms (production DefaultParams).
var fastArgon2Params = &userpass.PasswordParams{
	Memory:      1024, // 1 MB
	Iterations:  1,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// buildServiceFactory composes the service root the way the server does.
func buildServiceFactory(t *testing.T, db *bun.DB) *services.Factory {
	t.Helper()
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	require.NoError(t, serviceFactory.SetTenantRuntime(testpkg.TenantRuntime(t, db)))
	return serviceFactory
}

// buildAuthService wires the operator flows the way the service root does.
func buildAuthService(t *testing.T, db *bun.DB) operatorServiceShim {
	t.Helper()
	return operatorServiceShim{module: buildServiceFactory(t, db).OperatorAuth}
}

// buildOperatorPasswordChange returns the operator password change the root
// composes on Identity & Access (#3252); the e-mail change tokens it
// invalidates stay with the retained service under test here.
func buildOperatorPasswordChange(t *testing.T, db *bun.DB) interface {
	ChangeOperatorPassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error
} {
	t.Helper()
	return buildServiceFactory(t, db).AccountAuthentication()
}

// createEmailChangeTestOperator creates an operator with a real Argon2id password hash.
func createEmailChangeTestOperator(t *testing.T, db *bun.DB, email string) (int64, string) {
	t.Helper()
	ctx := context.Background()

	hash, err := userpass.HashPassword(operatorTestPassword, fastArgon2Params)
	require.NoError(t, err, "Failed to hash test password")

	var operatorID int64
	err = db.NewRaw(
		`INSERT INTO platform.operators (email, password_hash, display_name, active)
		 VALUES (?, ?, ?, true) RETURNING id`,
		email, hash, "Test Operator",
	).Scan(ctx, &operatorID)
	require.NoError(t, err, "Failed to create test operator")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_refresh_tokens WHERE operator_id = ?`, operatorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_audit_log WHERE operator_id = ?`, operatorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operators WHERE id = ?`, operatorID)
	})

	return operatorID, hash
}

// =============================================================================
// Service-Level Integration Tests (InitiateEmailChange / ConfirmEmailChange)
// =============================================================================

func TestIntegration_EmailChange_InitiateAndConfirm_HappyPath(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("initiate-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	newEmail := fmt.Sprintf("newemail-%d@test.local", time.Now().UnixNano())

	// Initiate email change
	err := service.InitiateEmailChange(ctx, operatorID, newEmail, operatorTestPassword, clientIP)
	require.NoError(t, err)

	// Verify token was created
	var tokenStr string
	err = db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Column("token").
		Where("operator_id = ? AND used = FALSE", operatorID).
		Scan(ctx, &tokenStr)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	// Verify audit log for initiation
	var initiateAuditCount int
	initiateAuditCount, err = db.NewSelect().
		TableExpr("platform.operator_audit_log").
		Where("operator_id = ? AND action = ?", operatorID, "email_change_initiated").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, initiateAuditCount, "should have audit log for initiation")

	// Confirm email change
	confirmedEmail, err := service.ConfirmEmailChange(ctx, tokenStr, clientIP)
	require.NoError(t, err)
	assert.Equal(t, newEmail, confirmedEmail)

	// Verify operator email was updated
	var updatedEmail string
	err = db.NewSelect().
		TableExpr("platform.operators").
		Column("email").
		Where("id = ?", operatorID).
		Scan(ctx, &updatedEmail)
	require.NoError(t, err)
	assert.Equal(t, newEmail, updatedEmail)

	// Verify audit log for confirmation
	var confirmAuditCount int
	confirmAuditCount, err = db.NewSelect().
		TableExpr("platform.operator_audit_log").
		Where("operator_id = ? AND action = ?", operatorID, "email_change_confirmed").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, confirmAuditCount, "should have audit log for confirmation")
}

func TestIntegration_EmailChange_Initiate_WrongPassword(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("wrongpw-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	err := service.InitiateEmailChange(ctx, operatorID, "new@test.local", "wrong-password", clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorPasswordMismatch)
}

func TestIntegration_EmailChange_Initiate_SameEmail(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("sameemail-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	err := service.InitiateEmailChange(ctx, operatorID, email, operatorTestPassword, clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeSameEmail)
}

func TestIntegration_EmailChange_Initiate_EmailAlreadyInUse(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("op1-%d@test.local", ts)
	email2 := fmt.Sprintf("op2-%d@test.local", ts)
	operatorID, _ := createEmailChangeTestOperator(t, db, email1)
	createEmailChangeTestOperator(t, db, email2) // second operator owns email2

	// Anti-enumeration: service returns nil (silent success) when target email
	// is already taken, preventing an authenticated attacker from probing
	// whether an address exists. The real uniqueness check happens in
	// ConfirmEmailChange inside the transaction.
	err := service.InitiateEmailChange(ctx, operatorID, email2, operatorTestPassword, clientIP)
	require.NoError(t, err, "should silently succeed to prevent email enumeration")

	// Verify no token was created (the request was short-circuited)
	tokenCount, err := db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Where("operator_id = ?", operatorID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, tokenCount, "no token should be created when email is already taken")
}

func TestIntegration_EmailChange_Initiate_RateLimit(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("ratelimit-svc-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Exhaust rate limit (5 requests per hour)
	for i := range 5 {
		newEmail := fmt.Sprintf("rl-%d-%d@test.local", i, time.Now().UnixNano())
		err := service.InitiateEmailChange(ctx, operatorID, newEmail, operatorTestPassword, clientIP)
		require.NoError(t, err, "request %d should succeed", i)
	}

	// 6th request should be rate limited
	err := service.InitiateEmailChange(ctx, operatorID, "one-too-many@test.local", operatorTestPassword, clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeRateLimited)
}

func TestIntegration_EmailChange_Confirm_InvalidToken(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	_, err := service.ConfirmEmailChange(ctx, "nonexistent-token", clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeNotFound)
}

func TestIntegration_EmailChange_Confirm_EmailTakenBetweenInitiateAndConfirm(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	ts := time.Now().UnixNano()
	email1 := fmt.Sprintf("race1-%d@test.local", ts)
	email2 := fmt.Sprintf("race2-%d@test.local", ts)
	targetEmail := fmt.Sprintf("target-%d@test.local", ts)

	operatorID, _ := createEmailChangeTestOperator(t, db, email1)
	createEmailChangeTestOperator(t, db, email2)

	// Operator 1 initiates change to targetEmail
	err := service.InitiateEmailChange(ctx, operatorID, targetEmail, operatorTestPassword, clientIP)
	require.NoError(t, err)

	// Read the token
	var tokenStr string
	err = db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Column("token").
		Where("operator_id = ? AND used = FALSE", operatorID).
		Scan(ctx, &tokenStr)
	require.NoError(t, err)

	// Simulate: operator 2 takes targetEmail via direct DB update (as if they confirmed first)
	_, err = db.ExecContext(ctx,
		`UPDATE platform.operators SET email = ? WHERE email = ?`,
		targetEmail, email2,
	)
	require.NoError(t, err)

	// Operator 1 tries to confirm — should fail with EmailAlreadyInUseError
	_, err = service.ConfirmEmailChange(ctx, tokenStr, clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorEmailInUse)
}

func TestIntegration_EmailChange_ChangePassword_InvalidatesToken(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("pwchange-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	newEmail := fmt.Sprintf("pwchange-new-%d@test.local", time.Now().UnixNano())

	// 1. Initiate email change — creates an active token
	err := service.InitiateEmailChange(ctx, operatorID, newEmail, operatorTestPassword, clientIP)
	require.NoError(t, err)

	// 2. Read the token
	var tokenStr string
	err = db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Column("token").
		Where("operator_id = ? AND used = FALSE", operatorID).
		Scan(ctx, &tokenStr)
	require.NoError(t, err)
	require.NotEmpty(t, tokenStr)

	// 3. Change password — must atomically invalidate the outstanding token
	newPassword := "ChangedPass789!"
	err = buildOperatorPasswordChange(t, db).ChangeOperatorPassword(ctx, operatorID, operatorTestPassword, newPassword)
	require.NoError(t, err)

	// 4. Verify the token is now marked as used in the database
	var used bool
	err = db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Column("used").
		Where("token = ?", tokenStr).
		Scan(ctx, &used)
	require.NoError(t, err)
	assert.True(t, used, "email change token should be invalidated after password change")

	// 5. Attempting to confirm with the invalidated token should fail
	_, err = service.ConfirmEmailChange(ctx, tokenStr, clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorEmailChangeNotFound, "confirming an invalidated token should return EmailChangeTokenInvalidError")
}

// =============================================================================
// Service-Level Cleanup Tests
// =============================================================================

func TestIntegration_EmailChange_Cleanup_ServiceLevel(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()

	email := fmt.Sprintf("svc-cleanup-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Create an expired unused token and a stale used token via raw SQL
	expiredToken := uuid.Must(uuid.NewV4()).String()
	_, err := db.ExecContext(ctx,
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used)
		 VALUES (?, ?, ?, ?, false)`,
		operatorID, "expired@test.local", expiredToken, time.Now().Add(-10*time.Minute),
	)
	require.NoError(t, err)

	staleToken := uuid.Must(uuid.NewV4()).String()
	_, err = db.ExecContext(ctx,
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used, created_at)
		 VALUES (?, ?, ?, ?, true, NOW() - INTERVAL '2 hours')`,
		operatorID, "stale@test.local", staleToken, time.Now().Add(-90*time.Minute),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
	})

	// Run service-level cleanup (invalidate + delete)
	deleted, err := service.CleanupExpiredEmailChangeTokens(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1, "should delete at least the stale token")

	// Verify expired token was invalidated (used=true)
	var used bool
	err = db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Column("used").
		Where("token = ?", expiredToken).
		Scan(ctx, &used)
	require.NoError(t, err)
	assert.True(t, used, "expired token should be marked as used after cleanup")

	// Verify stale token was deleted
	count, err := db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Where("token = ?", staleToken).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "stale token should be deleted after cleanup")
}

// =============================================================================
// Inactive Operator Tests
// =============================================================================

func TestIntegration_EmailChange_Initiate_InactiveOperator(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("inactive-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Deactivate operator
	_, err := db.ExecContext(ctx, `UPDATE platform.operators SET active = false WHERE id = ?`, operatorID)
	require.NoError(t, err)

	err = service.InitiateEmailChange(ctx, operatorID, "new@test.local", operatorTestPassword, clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorInactive)
}

func TestIntegration_EmailChange_Confirm_InactiveOperator(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("confirm-inactive-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	newEmail := fmt.Sprintf("confirm-inactive-new-%d@test.local", time.Now().UnixNano())

	// Initiate while active
	err := service.InitiateEmailChange(ctx, operatorID, newEmail, operatorTestPassword, clientIP)
	require.NoError(t, err)

	// Read the token
	var tokenStr string
	err = db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Column("token").
		Where("operator_id = ? AND used = FALSE", operatorID).
		Scan(ctx, &tokenStr)
	require.NoError(t, err)

	// Deactivate operator between initiate and confirm
	_, err = db.ExecContext(ctx, `UPDATE platform.operators SET active = false WHERE id = ?`, operatorID)
	require.NoError(t, err)

	// Confirm should fail with OperatorInactiveError
	_, err = service.ConfirmEmailChange(ctx, tokenStr, clientIP)
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrOperatorInactive)
}

// =============================================================================
// Email Validation Tests
// =============================================================================

func TestIntegration_EmailChange_Initiate_InvalidEmailFormat(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("emailfmt-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	tests := []struct {
		name     string
		newEmail string
	}{
		{"NoTLD", "user@localhost"},
		{"NoAt", "not-an-email"},
		{"EmptyAfterTrim", "   "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := service.InitiateEmailChange(ctx, operatorID, tc.newEmail, operatorTestPassword, clientIP)
			require.Error(t, err, "invalid email '%s' should be rejected", tc.newEmail)
		})
	}
}
