package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestInvitationToken creates a test invitation token in the database.
func createTestInvitationToken(t *testing.T, db *bun.DB, email string, roleID, createdBy int64, expiresAt time.Time) *auth.InvitationToken {
	t.Helper()

	ctx := context.Background()
	token := &auth.InvitationToken{
		Email:     email,
		Token:     uuid.Must(uuid.NewV4()).String(),
		RoleID:    roleID,
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}

	_, err := db.NewInsert().
		Model(token).
		ModelTableExpr(`auth.invitation_tokens`).
		Exec(ctx)
	require.NoError(t, err)

	return token
}

// cleanupInvitationTokens removes invitation tokens by ID.
func cleanupInvitationTokens(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	testpkg.CleanupTableRecords(t, db, "auth.invitation_tokens", ids...)
}

// ============================================================================
// FindByToken Tests
// ============================================================================

func TestInvitationTokenRepository_FindByToken_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "invite-test-role")
	creator := testpkg.CreateTestAccount(t, db, "invite-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation token
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "test@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	found, err := repo.FindByToken(ctx, invitation.Token)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, invitation.ID, found.ID)
	assert.Equal(t, invitation.Email, found.Email)
	assert.Equal(t, invitation.Token, found.Token)
}

func TestInvitationTokenRepository_FindByToken_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// ACT
	_, err := repo.FindByToken(ctx, "nonexistent-token")

	// ASSERT
	require.Error(t, err)
}

// ============================================================================
// FindByID Tests
// ============================================================================

func TestInvitationTokenRepository_FindByID_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "invite-by-id-role")
	creator := testpkg.CreateTestAccount(t, db, "invite-by-id-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation token
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "findbyid@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	found, err := repo.FindByID(ctx, invitation.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, invitation.ID, found.ID)
	assert.Equal(t, invitation.Email, found.Email)
}

func TestInvitationTokenRepository_FindByID_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// ACT
	_, err := repo.FindByID(ctx, int64(999999))

	// ASSERT
	require.Error(t, err)
}

// ============================================================================
// FindValidByToken Tests
// ============================================================================

func TestInvitationTokenRepository_FindValidByToken_Valid(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "valid-token-role")
	creator := testpkg.CreateTestAccount(t, db, "valid-token-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create valid (not expired, not used) invitation token
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "valid@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	found, err := repo.FindValidByToken(ctx, invitation.Token, time.Now())

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, invitation.ID, found.ID)
	assert.Nil(t, found.UsedAt)
}

func TestInvitationTokenRepository_FindValidByToken_Expired(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "expired-token-role")
	creator := testpkg.CreateTestAccount(t, db, "expired-token-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create expired invitation using raw SQL to bypass validation
	token := uuid.Must(uuid.NewV4()).String()
	var invitationID int64
	err := db.NewRaw(`
		INSERT INTO auth.invitation_tokens (email, token, role_id, created_by, expires_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, "expired@example.com", token, role.ID, creator.ID, time.Now().Add(-1*time.Hour)).
		Scan(ctx, &invitationID)
	require.NoError(t, err)
	defer cleanupInvitationTokens(t, db, invitationID)

	// ACT
	_, err = repo.FindValidByToken(ctx, token, time.Now())

	// ASSERT
	require.Error(t, err) // Should not find expired token
}

func TestInvitationTokenRepository_FindValidByToken_Used(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "used-token-role")
	creator := testpkg.CreateTestAccount(t, db, "used-token-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create used invitation
	token := uuid.Must(uuid.NewV4()).String()
	usedAt := time.Now()
	var invitationID int64
	err := db.NewRaw(`
		INSERT INTO auth.invitation_tokens (email, token, role_id, created_by, expires_at, used_at)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, "used@example.com", token, role.ID, creator.ID, time.Now().Add(48*time.Hour), usedAt).
		Scan(ctx, &invitationID)
	require.NoError(t, err)
	defer cleanupInvitationTokens(t, db, invitationID)

	// ACT
	_, err = repo.FindValidByToken(ctx, token, time.Now())

	// ASSERT
	require.Error(t, err) // Should not find used token
}

// ============================================================================
// FindByEmail Tests
// ============================================================================

func TestInvitationTokenRepository_FindByEmail_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "email-search-role")
	creator := testpkg.CreateTestAccount(t, db, "email-search-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation tokens for same email
	expiry := time.Now().Add(48 * time.Hour)
	email := "multiple@example.com"
	inv1 := createTestInvitationToken(t, db, email, role.ID, creator.ID, expiry)
	inv2 := createTestInvitationToken(t, db, email, role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, inv1.ID, inv2.ID)

	// ACT
	found, err := repo.FindByEmail(ctx, email)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(found), 2)
}

func TestInvitationTokenRepository_FindByEmail_CaseInsensitive(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "case-insensitive-role")
	creator := testpkg.CreateTestAccount(t, db, "case-insensitive-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation with lowercase email
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "lowercase@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT - search with uppercase
	found, err := repo.FindByEmail(ctx, "LOWERCASE@EXAMPLE.COM")

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, found)
}

// ============================================================================
// MarkAsUsed Tests
// ============================================================================

func TestInvitationTokenRepository_MarkAsUsed_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "mark-used-role")
	creator := testpkg.CreateTestAccount(t, db, "mark-used-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation token
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "markused@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	err := repo.MarkAsUsed(ctx, invitation.ID)

	// ASSERT
	require.NoError(t, err)

	// Verify token is now marked as used
	found, err := repo.FindByID(ctx, invitation.ID)
	require.NoError(t, err)
	assert.NotNil(t, found.UsedAt)
}

// ============================================================================
// InvalidateByEmail Tests
// ============================================================================

func TestInvitationTokenRepository_InvalidateByEmail_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "invalidate-role")
	creator := testpkg.CreateTestAccount(t, db, "invalidate-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create multiple unused invitations for same email
	expiry := time.Now().Add(48 * time.Hour)
	email := "invalidate@example.com"
	inv1 := createTestInvitationToken(t, db, email, role.ID, creator.ID, expiry)
	inv2 := createTestInvitationToken(t, db, email, role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, inv1.ID, inv2.ID)

	// ACT
	count, err := repo.InvalidateByEmail(ctx, email)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 2)

	// Verify both are now marked as used
	found1, _ := repo.FindByID(ctx, inv1.ID)
	found2, _ := repo.FindByID(ctx, inv2.ID)
	assert.NotNil(t, found1.UsedAt)
	assert.NotNil(t, found2.UsedAt)
}

// ============================================================================
// DeleteExpired Tests
// ============================================================================

func TestInvitationTokenRepository_DeleteExpired_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "delete-expired-role")
	creator := testpkg.CreateTestAccount(t, db, "delete-expired-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create expired invitation using raw SQL
	token := uuid.Must(uuid.NewV4()).String()
	var expiredID int64
	err := db.NewRaw(`
		INSERT INTO auth.invitation_tokens (email, token, role_id, created_by, expires_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id
	`, "expired-delete@example.com", token, role.ID, creator.ID, time.Now().Add(-1*time.Hour)).
		Scan(ctx, &expiredID)
	require.NoError(t, err)

	// Create valid invitation
	validExpiry := time.Now().Add(48 * time.Hour)
	validInv := createTestInvitationToken(t, db, "valid-delete@example.com", role.ID, creator.ID, validExpiry)
	defer cleanupInvitationTokens(t, db, validInv.ID)

	// ACT
	deleted, err := repo.DeleteExpired(ctx, time.Now())

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1)

	// Verify expired is gone
	_, err = repo.FindByID(ctx, expiredID)
	assert.Error(t, err)

	// Verify valid still exists
	_, err = repo.FindByID(ctx, validInv.ID)
	assert.NoError(t, err)
}

// ============================================================================
// List Tests
// ============================================================================

func TestInvitationTokenRepository_List_NoFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "list-role")
	creator := testpkg.CreateTestAccount(t, db, "list-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "list@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	results, err := repo.List(ctx, nil)

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestInvitationTokenRepository_List_WithEmailFilter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "list-email-role")
	creator := testpkg.CreateTestAccount(t, db, "list-email-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation with specific email
	expiry := time.Now().Add(48 * time.Hour)
	uniqueEmail := "unique-list@example.com"
	invitation := createTestInvitationToken(t, db, uniqueEmail, role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	results, err := repo.List(ctx, map[string]interface{}{
		"email": uniqueEmail,
	})

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	for _, inv := range results {
		assert.Equal(t, uniqueEmail, inv.Email)
	}
}

func TestInvitationTokenRepository_List_WithPendingFilter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "pending-filter-role")
	creator := testpkg.CreateTestAccount(t, db, "pending-filter-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create pending invitation
	expiry := time.Now().Add(48 * time.Hour)
	pendingInv := createTestInvitationToken(t, db, "pending@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, pendingInv.ID)

	// ACT
	results, err := repo.List(ctx, map[string]interface{}{
		"pending": true,
	})

	// ASSERT
	require.NoError(t, err)
	for _, inv := range results {
		assert.Nil(t, inv.UsedAt)
		assert.True(t, inv.ExpiresAt.After(time.Now()))
	}
}

// ============================================================================
// UpdateDeliveryResult Tests
// ============================================================================

func TestInvitationTokenRepository_UpdateDeliveryResult_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "delivery-result-role")
	creator := testpkg.CreateTestAccount(t, db, "delivery-result-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "delivery@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	sentAt := time.Now()
	err := repo.UpdateDeliveryResult(ctx, invitation.ID, &sentAt, nil, 1)

	// ASSERT
	require.NoError(t, err)

	// Verify delivery result was updated
	found, err := repo.FindByID(ctx, invitation.ID)
	require.NoError(t, err)
	assert.NotNil(t, found.EmailSentAt)
	assert.Nil(t, found.EmailError)
	assert.Equal(t, 1, found.EmailRetryCount)
}

func TestInvitationTokenRepository_UpdateDeliveryResult_WithError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "delivery-error-role")
	creator := testpkg.CreateTestAccount(t, db, "delivery-error-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "delivery-err@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	emailError := "SMTP connection failed"
	err := repo.UpdateDeliveryResult(ctx, invitation.ID, nil, &emailError, 2)

	// ASSERT
	require.NoError(t, err)

	// Verify error was recorded
	found, err := repo.FindByID(ctx, invitation.ID)
	require.NoError(t, err)
	assert.Nil(t, found.EmailSentAt)
	assert.NotNil(t, found.EmailError)
	assert.Equal(t, 2, found.EmailRetryCount)
}

// ============================================================================
// Update Tests
// ============================================================================

func TestInvitationTokenRepository_Update_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "update-role")
	creator := testpkg.CreateTestAccount(t, db, "update-creator")
	defer testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID)
	defer testpkg.CleanupAuthFixtures(t, db, creator.ID)

	// Create invitation
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "update@example.com", role.ID, creator.ID, expiry)
	defer cleanupInvitationTokens(t, db, invitation.ID)

	// ACT
	newFirstName := "John"
	newLastName := "Doe"
	invitation.FirstName = &newFirstName
	invitation.LastName = &newLastName
	err := repo.Update(ctx, invitation)

	// ASSERT
	require.NoError(t, err)

	// Verify update
	found, err := repo.FindByID(ctx, invitation.ID)
	require.NoError(t, err)
	assert.Equal(t, newFirstName, *found.FirstName)
	assert.Equal(t, newLastName, *found.LastName)
}

func TestInvitationTokenRepository_Update_NilReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).InvitationToken
	ctx := context.Background()

	// ACT
	err := repo.Update(ctx, nil)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}
