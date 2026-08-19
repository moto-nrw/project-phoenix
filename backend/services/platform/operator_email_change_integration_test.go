// Package platform_test contains integration tests for operator email change
// that require a real database connection.
package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const testPassword = "SecurePass123!"

// fastArgon2Params returns cheap Argon2id params for tests so hashing takes ~1ms
// instead of ~200ms (production DefaultParams).
var fastArgon2Params = &userpass.PasswordParams{
	Memory:      1024, // 1 MB
	Iterations:  1,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// buildAuthService creates a fully-wired OperatorAuthService via the standard factory.
func buildAuthService(t *testing.T, db *bun.DB) platformSvc.OperatorAuthService {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.OperatorAuth
}

// createEmailChangeTestOperator creates an operator with a real Argon2id password hash.
func createEmailChangeTestOperator(t *testing.T, db *bun.DB, email string) (int64, string) {
	t.Helper()
	ctx := context.Background()

	hash, err := userpass.HashPassword(testPassword, fastArgon2Params)
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

// insertRawToken inserts a token row directly for repository-level tests.
func insertRawToken(t *testing.T, db *bun.DB, operatorID int64, newEmail string, expiry time.Time, used bool) int64 {
	t.Helper()
	ctx := context.Background()

	tokenStr := uuid.Must(uuid.NewV4()).String()
	var tokenID int64
	err := db.NewRaw(
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		operatorID, newEmail, tokenStr, expiry, used,
	).Scan(ctx, &tokenID)
	require.NoError(t, err, "Failed to insert raw token")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE id = ?`, tokenID)
	})

	return tokenID
}

// =============================================================================
// Rate Limit Counting Tests
// =============================================================================

func TestIntegration_EmailChange_RateLimitCounting(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("ratelimit-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Insert 4 tokens within the last hour (3 used, 1 unused).
	// The partial unique index allows at most ONE active (used=false) token
	// per operator, so only the last token is left active.
	now := time.Now()
	for i := range 4 {
		used := i < 3 // first 3 used, last one active
		insertRawToken(t, db, operatorID, fmt.Sprintf("new%d@test.local", i), now.Add(30*time.Minute), used)
	}

	// Rate limit counts ALL tokens (including invalidated) within the hour window
	count, err := repo.CountRecentByOperatorID(ctx, operatorID, now.Add(-1*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 4, count, "rate limit should count all tokens regardless of used status")
}

func TestIntegration_EmailChange_RateLimitIgnoresOldTokens(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("ratelimit-old-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Insert a token with created_at > 2 hours ago via raw SQL
	tokenStr := uuid.Must(uuid.NewV4()).String()
	_, err := db.ExecContext(ctx,
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used, created_at)
		 VALUES (?, ?, ?, ?, false, NOW() - INTERVAL '2 hours')`,
		operatorID, "old@test.local", tokenStr, time.Now().Add(30*time.Minute),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
	})

	// Should not count the old token
	count, err := repo.CountRecentByOperatorID(ctx, operatorID, time.Now().Add(-1*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, count, "tokens older than the rate limit window should not be counted")
}

// =============================================================================
// Token Consumption Tests
// =============================================================================

func TestIntegration_EmailChange_ConsumeToken_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("consume-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Create a valid token
	tokenStr := uuid.Must(uuid.NewV4()).String()
	token := &platform.OperatorEmailChangeToken{
		OperatorID: operatorID,
		NewEmail:   "consumed@test.local",
		Token:      tokenStr,
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}
	err := repo.Create(ctx, token)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE id = ?`, token.ID)
	})

	// Consume it
	consumed, err := repo.ConsumeByToken(ctx, tokenStr)
	require.NoError(t, err)
	require.NotNil(t, consumed)
	assert.Equal(t, operatorID, consumed.OperatorID)
	assert.Equal(t, "consumed@test.local", consumed.NewEmail)
	assert.True(t, consumed.Used)

	// Second consumption should return nil (already used)
	consumed2, err := repo.ConsumeByToken(ctx, tokenStr)
	require.NoError(t, err)
	assert.Nil(t, consumed2, "token should not be consumable twice")
}

func TestIntegration_EmailChange_ConsumeToken_Expired(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("expired-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Create an expired token via raw SQL (model Validate() rejects expired tokens)
	tokenStr := uuid.Must(uuid.NewV4()).String()
	_, err := db.ExecContext(ctx,
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used)
		 VALUES (?, ?, ?, ?, false)`,
		operatorID, "expired@test.local", tokenStr, time.Now().Add(-10*time.Minute),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
	})

	consumed, err := repo.ConsumeByToken(ctx, tokenStr)
	require.NoError(t, err)
	assert.Nil(t, consumed, "expired token should not be consumable")
}

func TestIntegration_EmailChange_ConsumeToken_InvalidToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	consumed, err := repo.ConsumeByToken(ctx, "nonexistent-token-uuid")
	require.NoError(t, err)
	assert.Nil(t, consumed, "nonexistent token should return nil, not error")
}

// =============================================================================
// Partial Unique Index Tests
// =============================================================================

func TestIntegration_EmailChange_UniqueIndex_OneActivePerOperator(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("unique-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// First active token succeeds
	token1 := &platform.OperatorEmailChangeToken{
		OperatorID: operatorID,
		NewEmail:   "first@test.local",
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}
	err := repo.Create(ctx, token1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
	})

	// Second active token for same operator should fail (unique index violation)
	token2 := &platform.OperatorEmailChangeToken{
		OperatorID: operatorID,
		NewEmail:   "second@test.local",
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}
	err = repo.Create(ctx, token2)
	require.Error(t, err, "second active token should violate the partial unique index")
}

func TestIntegration_EmailChange_UniqueIndex_AllowsAfterInvalidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("invalidate-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Create first token
	token1 := &platform.OperatorEmailChangeToken{
		OperatorID: operatorID,
		NewEmail:   "first@test.local",
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}
	err := repo.Create(ctx, token1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
	})

	// Invalidate it
	err = repo.InvalidateByOperatorID(ctx, operatorID)
	require.NoError(t, err)

	// Now a new active token should be allowed
	token2 := &platform.OperatorEmailChangeToken{
		OperatorID: operatorID,
		NewEmail:   "second@test.local",
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}
	err = repo.Create(ctx, token2)
	require.NoError(t, err, "new token should be allowed after previous one is invalidated")
}

// =============================================================================
// Cleanup Job Tests
// =============================================================================

func TestIntegration_EmailChange_Cleanup_InvalidateExpired(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("cleanup-exp-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Create an expired but unused token via raw SQL
	tokenStr := uuid.Must(uuid.NewV4()).String()
	_, err := db.ExecContext(ctx,
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used)
		 VALUES (?, ?, ?, ?, false)`,
		operatorID, "expired@test.local", tokenStr, time.Now().Add(-5*time.Minute),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
	})

	// Run invalidation
	invalidated, err := repo.InvalidateExpiredTokens(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, invalidated, 1, "should have invalidated at least one expired token")

	// Verify the token is now marked as used
	var used bool
	err = db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Column("used").
		Where("token = ?", tokenStr).
		Scan(ctx, &used)
	require.NoError(t, err)
	assert.True(t, used, "expired token should be marked as used after invalidation")
}

func TestIntegration_EmailChange_Cleanup_DeleteStaleTokens(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("cleanup-stale-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Insert a stale token: used=true, created_at > 1 hour ago
	tokenStr := uuid.Must(uuid.NewV4()).String()
	_, err := db.ExecContext(ctx,
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used, created_at)
		 VALUES (?, ?, ?, ?, true, NOW() - INTERVAL '2 hours')`,
		operatorID, "stale@test.local", tokenStr, time.Now().Add(-90*time.Minute),
	)
	require.NoError(t, err)

	// Run deletion
	deleted, err := repo.DeleteStaleTokens(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1, "should have deleted at least one stale token")

	// Verify the token no longer exists
	count, err := db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Where("token = ?", tokenStr).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "stale token should be deleted")
}

func TestIntegration_EmailChange_Cleanup_PreservesRecentTokens(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := platformRepo.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("cleanup-recent-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Insert a recently-used token (created_at < 1 hour ago, used=true)
	// This should NOT be deleted because it's within the rate limit window.
	tokenStr := uuid.Must(uuid.NewV4()).String()
	_, err := db.ExecContext(ctx,
		`INSERT INTO platform.operator_email_change_tokens
		 (operator_id, new_email, token, expiry, used)
		 VALUES (?, ?, ?, ?, true)`,
		operatorID, "recent@test.local", tokenStr, time.Now().Add(-5*time.Minute),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
	})

	// Delete should not touch recently-created tokens
	_, err = repo.DeleteStaleTokens(ctx)
	require.NoError(t, err)

	count, err := db.NewSelect().
		TableExpr("platform.operator_email_change_tokens").
		Where("token = ?", tokenStr).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "recently-used token within rate limit window should be preserved")
}

// =============================================================================
// Service-Level Integration Tests (InitiateEmailChange / ConfirmEmailChange)
// =============================================================================

func TestIntegration_EmailChange_InitiateAndConfirm_HappyPath(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("initiate-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	newEmail := fmt.Sprintf("newemail-%d@test.local", time.Now().UnixNano())

	// Initiate email change
	err := service.InitiateEmailChange(ctx, operatorID, newEmail, testPassword, clientIP)
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
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("wrongpw-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	err := service.InitiateEmailChange(ctx, operatorID, "new@test.local", "wrong-password", clientIP)
	require.Error(t, err)
	var pwErr *platformSvc.PasswordMismatchError
	assert.ErrorAs(t, err, &pwErr)
}

func TestIntegration_EmailChange_Initiate_SameEmail(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("sameemail-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	err := service.InitiateEmailChange(ctx, operatorID, email, testPassword, clientIP)
	require.Error(t, err)
	var sameErr *platformSvc.EmailChangeSameEmailError
	assert.ErrorAs(t, err, &sameErr)
}

func TestIntegration_EmailChange_Initiate_EmailAlreadyInUse(t *testing.T) {
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
	err := service.InitiateEmailChange(ctx, operatorID, email2, testPassword, clientIP)
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
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("ratelimit-svc-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Exhaust rate limit (5 requests per hour)
	for i := range 5 {
		newEmail := fmt.Sprintf("rl-%d-%d@test.local", i, time.Now().UnixNano())
		err := service.InitiateEmailChange(ctx, operatorID, newEmail, testPassword, clientIP)
		require.NoError(t, err, "request %d should succeed", i)
	}

	// 6th request should be rate limited
	err := service.InitiateEmailChange(ctx, operatorID, "one-too-many@test.local", testPassword, clientIP)
	require.Error(t, err)
	var rlErr *platformSvc.EmailChangeRateLimitError
	assert.ErrorAs(t, err, &rlErr)
}

func TestIntegration_EmailChange_Confirm_InvalidToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	_, err := service.ConfirmEmailChange(ctx, "nonexistent-token", clientIP)
	require.Error(t, err)
	var tokenErr *platformSvc.EmailChangeTokenInvalidError
	assert.ErrorAs(t, err, &tokenErr)
}

func TestIntegration_EmailChange_Confirm_EmailTakenBetweenInitiateAndConfirm(t *testing.T) {
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
	err := service.InitiateEmailChange(ctx, operatorID, targetEmail, testPassword, clientIP)
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
	var inUseErr *platformSvc.EmailAlreadyInUseError
	assert.ErrorAs(t, err, &inUseErr)
}

func TestIntegration_EmailChange_ChangePassword_InvalidatesToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("pwchange-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	newEmail := fmt.Sprintf("pwchange-new-%d@test.local", time.Now().UnixNano())

	// 1. Initiate email change — creates an active token
	err := service.InitiateEmailChange(ctx, operatorID, newEmail, testPassword, clientIP)
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
	err = service.ChangePassword(ctx, operatorID, testPassword, newPassword)
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
	var tokenErr *platformSvc.EmailChangeTokenInvalidError
	assert.ErrorAs(t, err, &tokenErr, "confirming an invalidated token should return EmailChangeTokenInvalidError")
}

// =============================================================================
// Service-Level Cleanup Tests
// =============================================================================

func TestIntegration_EmailChange_Cleanup_ServiceLevel(t *testing.T) {
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
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("inactive-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	// Deactivate operator
	_, err := db.ExecContext(ctx, `UPDATE platform.operators SET active = false WHERE id = ?`, operatorID)
	require.NoError(t, err)

	err = service.InitiateEmailChange(ctx, operatorID, "new@test.local", testPassword, clientIP)
	require.Error(t, err)
	var inactiveErr *platformSvc.OperatorInactiveError
	assert.ErrorAs(t, err, &inactiveErr)
}

func TestIntegration_EmailChange_Confirm_InactiveOperator(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	clientIP := net.IPv4(127, 0, 0, 1)

	email := fmt.Sprintf("confirm-inactive-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	newEmail := fmt.Sprintf("confirm-inactive-new-%d@test.local", time.Now().UnixNano())

	// Initiate while active
	err := service.InitiateEmailChange(ctx, operatorID, newEmail, testPassword, clientIP)
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
	var inactiveErr *platformSvc.OperatorInactiveError
	assert.ErrorAs(t, err, &inactiveErr)
}

// =============================================================================
// Email Validation Tests
// =============================================================================

func TestIntegration_EmailChange_Initiate_InvalidEmailFormat(t *testing.T) {
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
			err := service.InitiateEmailChange(ctx, operatorID, tc.newEmail, testPassword, clientIP)
			require.Error(t, err, "invalid email '%s' should be rejected", tc.newEmail)
		})
	}
}
