package authpostgres_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Test Helpers
// ============================================================================

// createTestInvitationToken creates a test invitation token in the database.
func createTestInvitationToken(t *testing.T, db *bun.DB, email string, roleID, createdBy int64, expiresAt time.Time) *authmodels.InvitationToken {
	t.Helper()

	ctx := testpkg.Ctx(t)
	token := &authmodels.InvitationToken{
		Email:     email,
		Token:     uuid.Must(uuid.NewV4()).String(),
		RoleID:    roleID,
		ExpiresAt: expiresAt,
	}
	if createdBy > 0 {
		token.CreatedBy = &createdBy
	}
	token.SetTenantID(testpkg.Tenant(t))

	_, err := db.NewInsert().
		Model(token).
		ModelTableExpr(`auth.invitation_tokens`).
		Exec(ctx)
	require.NoError(t, err)

	return token
}

// FindByID Tests
// ============================================================================

func TestInvitationTokenRepository_FindByID_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "invite-by-id-role")
	creator := testpkg.CreateTestAccount(t, db, "invite-by-id-creator")

	// Create invitation token
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "findbyid@example.com", role.ID, creator.ID, expiry)

	// ACT
	found, err := repo.FindByID(ctx, invitation.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, invitation.ID, found.ID)
	assert.Equal(t, invitation.Email, found.Email)
}

func TestInvitationTokenRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// ACT
	_, err := repo.FindByID(ctx, int64(999999))

	// ASSERT
	require.Error(t, err)
}

// ============================================================================
// FindValidByToken Tests
// ============================================================================

// ============================================================================
// FindByEmail Tests
// ============================================================================

func TestInvitationTokenRepository_FindByEmail_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "email-search-role")
	creator := testpkg.CreateTestAccount(t, db, "email-search-creator")

	// Create invitation tokens for same email
	expiry := time.Now().Add(48 * time.Hour)
	email := "multiple@example.com"
	createTestInvitationToken(t, db, email, role.ID, creator.ID, expiry)
	createTestInvitationToken(t, db, email, role.ID, creator.ID, expiry)

	// ACT
	found, err := repo.FindByEmail(ctx, email)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(found), 2)
}

func TestInvitationTokenRepository_FindByEmail_CaseInsensitive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "case-insensitive-role")
	creator := testpkg.CreateTestAccount(t, db, "case-insensitive-creator")

	// Create invitation with lowercase email
	expiry := time.Now().Add(48 * time.Hour)
	createTestInvitationToken(t, db, "lowercase@example.com", role.ID, creator.ID, expiry)

	// ACT - search with uppercase
	found, err := repo.FindByEmail(ctx, "LOWERCASE@EXAMPLE.COM")

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, found)
}

// ============================================================================
// MarkAsUsed Tests
// ============================================================================

// ============================================================================
// InvalidateByEmail Tests
// ============================================================================

// ============================================================================
// DeleteExpired Tests
// ============================================================================

func TestInvitationTokenRepository_DeleteExpired_Success(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "delete-expired-role")
	creator := testpkg.CreateTestAccount(t, db, "delete-expired-creator")

	// Create expired invitation using raw SQL
	token := uuid.Must(uuid.NewV4()).String()
	var expiredID int64
	err := db.NewRaw(`
		INSERT INTO auth.invitation_tokens (email, token, role_id, created_by, expires_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, "expired-delete@example.com", token, role.ID, creator.ID, time.Now().Add(-1*time.Hour), testpkg.Tenant(t)).
		Scan(ctx, &expiredID)
	require.NoError(t, err)

	// An expired invitation from another tenant must not be swept by this cleanup.
	otherTenant := testpkg.NewTenantScope(t, db)
	otherToken := uuid.Must(uuid.NewV4()).String()
	var otherExpiredID int64
	err = db.NewRaw(`
		INSERT INTO auth.invitation_tokens (email, token, role_id, created_by, expires_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, ?)
		RETURNING id
	`, "other-expired-delete@example.com", otherToken, role.ID, creator.ID, time.Now().Add(-1*time.Hour), otherTenant.TenantID).
		Scan(otherTenant.Context(), &otherExpiredID)
	require.NoError(t, err)

	// Create valid invitation
	validExpiry := time.Now().Add(48 * time.Hour)
	validInv := createTestInvitationToken(t, db, "valid-delete@example.com", role.ID, creator.ID, validExpiry)

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

	// Verify another tenant's expired invitation still exists.
	_, err = repo.FindByID(otherTenant.Context(), otherExpiredID)
	assert.NoError(t, err)
}

// ============================================================================
// List Tests
// ============================================================================

func TestInvitationTokenRepository_List_NoFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "list-role")
	creator := testpkg.CreateTestAccount(t, db, "list-creator")

	// Create invitation
	expiry := time.Now().Add(48 * time.Hour)
	createTestInvitationToken(t, db, "list@example.com", role.ID, creator.ID, expiry)

	// ACT
	results, err := repo.List(ctx, nil)

	// ASSERT
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestInvitationTokenRepository_List_WithEmailFilter(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "list-email-role")
	creator := testpkg.CreateTestAccount(t, db, "list-email-creator")

	// Create invitation with specific email
	expiry := time.Now().Add(48 * time.Hour)
	uniqueEmail := "unique-list@example.com"
	createTestInvitationToken(t, db, uniqueEmail, role.ID, creator.ID, expiry)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "pending-filter-role")
	creator := testpkg.CreateTestAccount(t, db, "pending-filter-creator")

	// Create pending invitation
	expiry := time.Now().Add(48 * time.Hour)
	createTestInvitationToken(t, db, "pending@example.com", role.ID, creator.ID, expiry)

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

// ============================================================================
// Update Tests
// ============================================================================

func TestInvitationTokenRepository_Update_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// Create dependencies
	role := testpkg.CreateTestRole(t, db, "update-role")
	creator := testpkg.CreateTestAccount(t, db, "update-creator")

	// Create invitation
	expiry := time.Now().Add(48 * time.Hour)
	invitation := createTestInvitationToken(t, db, "update@example.com", role.ID, creator.ID, expiry)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).InvitationToken
	ctx := testpkg.Ctx(t)

	// ACT
	err := repo.Update(ctx, nil)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}
