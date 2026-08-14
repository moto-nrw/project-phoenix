package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestTokenRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("creates token with valid data", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenCreate")
		defer cleanupAccountRecords(t, db, account.ID)

		token := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(time.Hour),
		}

		err := repo.Create(ctx, token)
		require.NoError(t, err)
		assert.NotZero(t, token.ID)

		testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)
	})

	t.Run("creates mobile token", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "mobileToken")
		defer cleanupAccountRecords(t, db, account.ID)

		token := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(15 * time.Minute),
			Mobile:    true,
		}

		err := repo.Create(ctx, token)
		require.NoError(t, err)
		assert.True(t, token.Mobile)

		testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)
	})
}

func TestTokenRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("finds existing token", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenFindByID")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)

		found, err := repo.FindByID(ctx, token.ID)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
	})

	t.Run("returns error for non-existent token", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestTokenRepository_FindByToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("finds token by token string", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenFindByToken")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)

		found, err := repo.FindByToken(ctx, token.Token)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
	})

	t.Run("returns error for non-existent token string", func(t *testing.T) {
		_, err := repo.FindByToken(ctx, "nonexistent-token-string")
		require.Error(t, err)
	})
}

func TestTokenRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("updates token identifier", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenUpdate")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)

		identifier := "updated-identifier"
		token.Identifier = &identifier
		err := repo.Update(ctx, token)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, token.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.Identifier)
		assert.Equal(t, identifier, *found.Identifier)
	})
}

func TestTokenRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("deletes existing token", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenDelete")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)

		err := repo.Delete(ctx, token.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, token.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestTokenRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("lists all tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenList")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)

		tokens, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, tokens)
	})
}

func TestTokenRepository_FindByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("finds tokens by account ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenByAccount")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)

		tokens, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, tokens)

		var found bool
		for _, tk := range tokens {
			if tk.ID == token.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for account with no tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "noTokens")
		defer cleanupAccountRecords(t, db, account.ID)

		tokens, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, tokens)
	})
}

// ============================================================================
// Token Cleanup Tests
// ============================================================================

func TestTokenRepository_DeleteExpiredTokens(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("deletes expired tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "expiredToken")
		defer cleanupAccountRecords(t, db, account.ID)

		// Create expired token directly using raw SQL (bypassing validation)
		expiredTokenStr := uuid.Must(uuid.NewV4()).String()
		var expiredTokenID int64
		err := db.NewRaw(`
			INSERT INTO auth.tokens (account_id, token, expiry, mobile, family_id, tenant_id)
			VALUES (?, ?, ?, false, ?, 1)
			RETURNING id
		`, account.ID, expiredTokenStr, time.Now().Add(-time.Hour), uuid.Must(uuid.NewV4()).String()).
			Scan(ctx, &expiredTokenID)
		require.NoError(t, err)

		// Create valid token
		validToken := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", validToken.ID)

		// Delete expired tokens
		deleted, err := repo.DeleteExpiredTokens(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, 1)

		// Verify expired token is gone
		_, err = repo.FindByID(ctx, expiredTokenID)
		require.Error(t, err)

		// Verify valid token still exists
		_, err = repo.FindByID(ctx, validToken.ID)
		require.NoError(t, err)
	})
}

func TestTokenRepository_HasLiveTokensCreatedAfter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "liveAfterToken")
	defer cleanupAccountRecords(t, db, account.ID)

	token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
	defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token.ID)

	has, err := repo.HasLiveTokensCreatedAfter(ctx, account.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.True(t, has)

	has, err = repo.HasLiveTokensCreatedAfter(ctx, account.ID, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, has)
}

func TestTokenRepository_RotationHandoffLifecycle(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "rotationHandoff")
	defer cleanupAccountRecords(t, db, account.ID)
	familyID := uuid.Must(uuid.NewV4()).String()
	predecessor := &auth.Token{
		AccountID:  account.ID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(time.Hour),
		FamilyID:   familyID,
		Generation: 0,
	}
	successor := &auth.Token{
		AccountID:  account.ID,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(time.Hour),
		FamilyID:   familyID,
		Generation: 1,
	}
	require.NoError(t, repo.Create(ctx, predecessor))
	require.NoError(t, repo.Create(ctx, successor))

	rotatedAt := time.Now().Add(-10 * time.Minute)
	recoveryProofHash := make([]byte, 32)
	recoveryProofHash[0] = 1
	require.NoError(t, repo.MarkRotated(ctx, predecessor.ID, successor.Token, recoveryProofHash, rotatedAt))
	stored, err := repo.FindByToken(ctx, predecessor.Token)
	require.NoError(t, err)
	require.NotNil(t, stored.RotatedAt)
	require.NotNil(t, stored.ReplacementToken)
	assert.Equal(t, successor.Token, *stored.ReplacementToken)
	assert.Equal(t, recoveryProofHash, stored.RecoveryProofHash)

	// Rotation age alone must never erase replay evidence while the token can
	// still be presented as a valid JWT.
	require.NoError(t, repo.DeleteExpiredRotatedForAccount(ctx, account.ID, time.Now()))
	_, err = repo.FindByToken(ctx, predecessor.Token)
	require.NoError(t, err)
	current, err := repo.FindByToken(ctx, successor.Token)
	require.NoError(t, err)
	assert.Equal(t, 1, current.Generation)
}

func TestTokenRepository_DeleteByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("deletes all tokens for account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deleteByAccount")
		token1 := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		token2 := testpkg.CreateTestToken(t, db, account.ID, "access")
		defer cleanupAccountRecords(t, db, account.ID)

		deleted, err := repo.DeleteByAccountIDReturning(ctx, account.ID)
		require.NoError(t, err)
		require.Len(t, deleted, 2)

		// Verify tokens are gone
		_, err = repo.FindByID(ctx, token1.ID)
		require.Error(t, err)
		_, err = repo.FindByID(ctx, token2.ID)
		require.Error(t, err)
	})
}

func TestTokenRepository_DeleteByAccountIDReturningKeepsOtherSchoolInAdminTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	account := testpkg.CreateTestAccount(t, db, "adminTxTenantDelete")
	defer cleanupAccountRecords(t, db, account.ID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)

	local := testpkg.CreateTestTokenForTenant(t, db, tenantID, account.ID)
	other := &auth.Token{
		AccountID:   account.ID,
		Token:       uuid.Must(uuid.NewV4()).String(),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: auth.PortalScopeTenant,
	}
	other.SetTenantID(secondaryTenantID)
	_, err := db.NewInsert().Model(other).ModelTableExpr("auth.tokens").Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		deleted, delErr := repo.DeleteByAccountIDReturning(adminCtx, account.ID)
		require.NoError(t, delErr)
		require.Len(t, deleted, 1)
		require.Equal(t, local.ID, deleted[0].ID)
		return nil
	}))

	_, err = repo.FindByID(testpkg.TenantContext(secondaryTenantID), other.ID)
	require.NoError(t, err)
}

func TestTokenRepository_DeleteAllByAccountIDReturningIgnoresTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	account := testpkg.CreateTestAccount(t, db, "deleteAllAccountTokens")
	defer cleanupAccountRecords(t, db, account.ID)
	secondaryTenantID, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)

	_ = testpkg.CreateTestTokenForTenant(t, db, tenantID, account.ID)
	other := &auth.Token{
		AccountID:   account.ID,
		Token:       uuid.Must(uuid.NewV4()).String(),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: auth.PortalScopeTenant,
	}
	other.SetTenantID(secondaryTenantID)
	_, err := db.NewInsert().Model(other).ModelTableExpr("auth.tokens").Exec(context.Background())
	require.NoError(t, err)

	deleted, err := repo.DeleteAllByAccountIDReturning(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, deleted, 2)
}

func TestTokenRepository_DeleteByAccountIDCreatedAtOrBeforeIncludesRefreshSuccessor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cutoffRefreshSuccessor")
	defer cleanupAccountRecords(t, db, account.ID)

	familyID := uuid.Must(uuid.NewV4()).String()
	newFamilyID := uuid.Must(uuid.NewV4()).String()
	cutoff := time.Now().Add(-time.Minute)
	_, err := db.NewRaw(`
		INSERT INTO auth.tokens (account_id, token, expiry, tenant_id, portal_scope, family_id, generation, created_at)
		VALUES
			(?, ?, ?, 1, ?, ?, 0, ?),
			(?, ?, ?, 1, ?, ?, 1, ?),
			(?, ?, ?, 1, ?, ?, 1, ?),
			(?, ?, ?, 1, ?, ?, 0, ?)
	`,
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), auth.PortalScopeTenant, familyID, cutoff.Add(-time.Minute),
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), auth.PortalScopeTenant, familyID, cutoff.Add(time.Minute),
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), auth.PortalScopeTenant, uuid.Must(uuid.NewV4()).String(), cutoff.Add(time.Minute),
		account.ID, uuid.Must(uuid.NewV4()).String(), time.Now().Add(time.Hour), auth.PortalScopeTenant, newFamilyID, cutoff.Add(time.Minute),
	).Exec(ctx)
	require.NoError(t, err)

	deleted, err := repo.DeleteByAccountIDCreatedAtOrBeforeReturning(ctx, account.ID, cutoff)
	require.NoError(t, err)
	require.Len(t, deleted, 3)

	remaining, err := repo.FindByFamilyID(ctx, newFamilyID)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, 0, remaining[0].Generation)
}

// ============================================================================
// Token Family Tests
// ============================================================================

func TestTokenRepository_FindByFamilyID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("finds tokens by family ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenFamily")
		defer cleanupAccountRecords(t, db, account.ID)

		familyID := uuid.Must(uuid.NewV4()).String()

		// Create tokens in same family
		token1 := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(time.Hour),
			FamilyID:  familyID,
		}
		err := repo.Create(ctx, token1)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token1.ID)

		token2 := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(time.Hour),
			FamilyID:  familyID,
		}
		err = repo.Create(ctx, token2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token2.ID)

		// Find by family
		tokens, err := repo.FindByFamilyID(ctx, familyID)
		require.NoError(t, err)
		assert.Len(t, tokens, 2)
	})
}

func TestTokenRepository_DeleteByFamilyID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("deletes all tokens in family", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deleteFamily")
		defer cleanupAccountRecords(t, db, account.ID)

		familyID := uuid.Must(uuid.NewV4()).String()

		// Create tokens in same family
		token1 := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(time.Hour),
			FamilyID:  familyID,
		}
		err := repo.Create(ctx, token1)
		require.NoError(t, err)

		token2 := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(time.Hour),
			FamilyID:  familyID,
		}
		err = repo.Create(ctx, token2)
		require.NoError(t, err)

		// Delete family
		deleted, err := repo.DeleteByFamilyIDReturning(ctx, familyID)
		require.NoError(t, err)
		require.Len(t, deleted, 2)

		// Verify tokens are gone
		tokens, err := repo.FindByFamilyID(ctx, familyID)
		require.NoError(t, err)
		assert.Empty(t, tokens)
	})
}

func TestTokenRepository_CleanupOldTokensForAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cleanupTokens")
	defer cleanupAccountRecords(t, db, account.ID)

	var activeTokens []*auth.Token
	for i := 0; i < 5; i++ {
		token := &auth.Token{
			AccountID: account.ID,
			Token:     fmt.Sprintf("cleanup-token-%d-%d", time.Now().UnixNano(), i),
			Expiry:    time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, token))
		activeTokens = append(activeTokens, token)
	}

	rotated := &auth.Token{
		AccountID: account.ID,
		Token:     uuid.Must(uuid.NewV4()).String(),
		Expiry:    time.Now().Add(time.Hour),
		FamilyID:  uuid.Must(uuid.NewV4()).String(),
	}
	require.NoError(t, repo.Create(ctx, rotated))
	require.NoError(t, repo.MarkRotated(ctx, rotated.ID, activeTokens[0].Token, make([]byte, 32), time.Now()))

	expired := &auth.Token{
		AccountID: account.ID,
		Token:     uuid.Must(uuid.NewV4()).String(),
		Expiry:    time.Now().Add(-time.Hour),
	}
	require.NoError(t, repo.Create(ctx, expired))

	deleted, err := repo.CleanupOldTokensForAccountReturning(ctx, account.ID, auth.PortalScopeUnknown, 5)
	require.NoError(t, err)
	require.Empty(t, deleted)
	for _, token := range activeTokens {
		_, err := repo.FindByID(ctx, token.ID)
		require.NoError(t, err, "rotated and expired rows must not displace active sessions")
	}

	newest := &auth.Token{
		AccountID: account.ID,
		Token:     uuid.Must(uuid.NewV4()).String(),
		Expiry:    time.Now().Add(time.Hour),
	}
	require.NoError(t, repo.Create(ctx, newest))
	deleted, err = repo.CleanupOldTokensForAccountReturning(ctx, account.ID, auth.PortalScopeUnknown, 5)
	require.NoError(t, err)
	require.Len(t, deleted, 1)

	var currentIDs []int64
	err = db.NewSelect().
		TableExpr("auth.tokens").
		Column("id").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("expiry > ?", time.Now()).
		OrderExpr("id ASC").
		Scan(ctx, &currentIDs)
	require.NoError(t, err)
	assert.Len(t, currentIDs, 5)
	assert.NotContains(t, currentIDs, activeTokens[0].ID, "the oldest active session must be evicted")
	assert.Contains(t, currentIDs, newest.ID, "the newly created session must survive cap enforcement")

	_, err = repo.FindByID(ctx, rotated.ID)
	require.NoError(t, err, "session-cap cleanup must leave recovery handoffs alone")
	_, err = repo.FindByID(ctx, expired.ID)
	require.NoError(t, err, "session-cap cleanup must leave expired-token cleanup to its own lifecycle")
}

func TestTokenRepository_CleanupOldTokensForAccount_IgnoresOtherPortal(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cleanupPortalTokens")
	defer cleanupAccountRecords(t, db, account.ID)

	parent := &auth.Token{
		AccountID:   account.ID,
		Token:       uuid.Must(uuid.NewV4()).String(),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: auth.PortalScopeParent,
	}
	require.NoError(t, repo.Create(ctx, parent))

	for i := 0; i < 5; i++ {
		token := &auth.Token{
			AccountID:   account.ID,
			Token:       fmt.Sprintf("portal-cap-token-%d-%d", time.Now().UnixNano(), i),
			Expiry:      time.Now().Add(time.Hour),
			PortalScope: auth.PortalScopeTenant,
		}
		require.NoError(t, repo.Create(ctx, token))
	}

	deleted, err := repo.CleanupOldTokensForAccountReturning(ctx, account.ID, auth.PortalScopeTenant, 5)
	require.NoError(t, err)
	require.Empty(t, deleted)

	_, err = repo.FindByID(ctx, parent.ID)
	require.NoError(t, err, "a parent-portal session must not be evicted by the tenant session cap")
}

func TestTokenRepository_CleanupOldTokensForAccount_TenantCapLeavesUnknownIsolated(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cleanupUnknownPortalTokens")
	defer cleanupAccountRecords(t, db, account.ID)

	var unknown []*auth.Token
	for i := 0; i < 5; i++ {
		token := &auth.Token{
			AccountID:   account.ID,
			Token:       fmt.Sprintf("unknown-cap-token-%d-%d", time.Now().UnixNano(), i),
			Expiry:      time.Now().Add(time.Hour),
			PortalScope: auth.PortalScopeUnknown,
		}
		require.NoError(t, repo.Create(ctx, token))
		unknown = append(unknown, token)
	}

	newest := &auth.Token{
		AccountID:   account.ID,
		Token:       uuid.Must(uuid.NewV4()).String(),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: auth.PortalScopeTenant,
	}
	require.NoError(t, repo.Create(ctx, newest))

	deleted, err := repo.CleanupOldTokensForAccountReturning(ctx, account.ID, auth.PortalScopeTenant, 5)
	require.NoError(t, err)
	require.Empty(t, deleted)

	for _, token := range unknown {
		_, err = repo.FindByID(ctx, token.ID)
		require.NoError(t, err, "unknown sessions must stay outside a known portal cap")
	}
	_, err = repo.FindByID(ctx, newest.ID)
	require.NoError(t, err)
}

func TestTokenRepository_CleanupOldTokensForAccount_StaffGroupSharesTenantAndOrg(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cleanupStaffGroupTokens")
	defer cleanupAccountRecords(t, db, account.ID)

	var orgTokens []*auth.Token
	for i := 0; i < 5; i++ {
		token := &auth.Token{
			AccountID:   account.ID,
			Token:       fmt.Sprintf("org-cap-token-%d-%d", time.Now().UnixNano(), i),
			Expiry:      time.Now().Add(time.Hour),
			PortalScope: auth.PortalScopeOrg,
		}
		require.NoError(t, repo.Create(ctx, token))
		orgTokens = append(orgTokens, token)
	}

	newest := &auth.Token{
		AccountID:   account.ID,
		Token:       uuid.Must(uuid.NewV4()).String(),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: auth.PortalScopeTenant,
	}
	require.NoError(t, repo.Create(ctx, newest))

	deleted, err := repo.CleanupOldTokensForAccountReturning(ctx, account.ID, auth.PortalScopeTenant, 5)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	require.Equal(t, orgTokens[0].ID, deleted[0].ID)

	_, err = repo.FindByID(ctx, newest.ID)
	require.NoError(t, err)
}

func TestTokenRepository_CleanupOldTokensForAccount_AdminTxIgnoresTenantFilter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cleanupAdminCapTokens")
	defer cleanupAccountRecords(t, db, account.ID)
	testpkg.EnsureTestTenant(t, db, 2)
	testpkg.MapAccountToTenant(t, db, account.ID, 2)

	var firstSchool []*auth.Token
	for i := 0; i < 5; i++ {
		token := &auth.Token{
			AccountID:   account.ID,
			Token:       fmt.Sprintf("admin-cap-t1-%d-%d", time.Now().UnixNano(), i),
			Expiry:      time.Now().Add(time.Hour),
			PortalScope: auth.PortalScopeTenant,
		}
		require.NoError(t, repo.Create(ctx, token))
		firstSchool = append(firstSchool, token)
	}
	other := &auth.Token{
		AccountID:   account.ID,
		Token:       uuid.Must(uuid.NewV4()).String(),
		Expiry:      time.Now().Add(time.Hour),
		PortalScope: auth.PortalScopeTenant,
	}
	other.SetTenantID(2)
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		return repo.Create(adminCtx, other)
	}))

	var deleted []*auth.Token
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		var err error
		deleted, err = repo.CleanupOldTokensForAccountReturning(adminCtx, account.ID, auth.PortalScopeTenant, 5)
		return err
	}))
	require.Len(t, deleted, 1)
	require.Equal(t, firstSchool[0].ID, deleted[0].ID)
}

func TestTokenRepository_CleanupOldTokensForAccount_EmptyPortalScopeUsesUnknown(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cleanupEmptyPortalScope")
	defer cleanupAccountRecords(t, db, account.ID)

	var created []*auth.Token
	for i := 0; i < 6; i++ {
		token := &auth.Token{
			AccountID: account.ID,
			Token:     fmt.Sprintf("empty-scope-token-%d-%d", time.Now().UnixNano(), i),
			Expiry:    time.Now().Add(time.Hour),
		}
		require.NoError(t, repo.Create(ctx, token))
		created = append(created, token)
	}

	deleted, err := repo.CleanupOldTokensForAccountReturning(ctx, account.ID, "", 5)
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	require.Equal(t, created[0].ID, deleted[0].ID)
	require.Equal(t, auth.PortalScopeUnknown, deleted[0].PortalScope)
}

func TestTokenRepository_DeleteExpiredRotatedForAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)
	account := testpkg.CreateTestAccount(t, db, "cleanupAccountHandoffs")
	defer cleanupAccountRecords(t, db, account.ID)

	createHandoff := func(expiry time.Time) (*auth.Token, *auth.Token) {
		familyID := uuid.Must(uuid.NewV4()).String()
		predecessor := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    expiry,
			FamilyID:  familyID,
		}
		successor := &auth.Token{
			AccountID:  account.ID,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   familyID,
			Generation: 1,
		}
		require.NoError(t, repo.Create(ctx, predecessor))
		require.NoError(t, repo.Create(ctx, successor))
		require.NoError(t, repo.MarkRotated(ctx, predecessor.ID, successor.Token, make([]byte, 32), time.Now().Add(-10*time.Minute)))
		return predecessor, successor
	}

	expiredPredecessor, expiredSuccessor := createHandoff(time.Now().Add(-time.Minute))
	validPredecessor, validSuccessor := createHandoff(time.Now().Add(time.Hour))
	require.NoError(t, repo.DeleteExpiredRotatedForAccount(ctx, account.ID, time.Now()))

	_, err := repo.FindByID(ctx, expiredPredecessor.ID)
	assert.Error(t, err)
	for _, token := range []*auth.Token{expiredSuccessor, validPredecessor, validSuccessor} {
		_, err = repo.FindByID(ctx, token.ID)
		require.NoError(t, err, "cleanup must preserve current sessions and unexpired replay evidence")
	}
}

func TestTokenRepository_GetLatestTokenInFamily(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("gets token with highest generation", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "familyLatest")
		defer cleanupAccountRecords(t, db, account.ID)

		familyID := uuid.Must(uuid.NewV4()).String()

		// Create tokens with different generations
		token1 := &auth.Token{
			AccountID:  account.ID,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   familyID,
			Generation: 1,
		}
		err := repo.Create(ctx, token1)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token1.ID)

		token2 := &auth.Token{
			AccountID:  account.ID,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   familyID,
			Generation: 2,
		}
		err = repo.Create(ctx, token2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token2.ID)

		token3 := &auth.Token{
			AccountID:  account.ID,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(time.Hour),
			FamilyID:   familyID,
			Generation: 3,
		}
		err = repo.Create(ctx, token3)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", token3.ID)

		// Get latest
		latest, err := repo.GetLatestTokenInFamily(ctx, familyID)
		require.NoError(t, err)
		assert.Equal(t, token3.ID, latest.ID)
		assert.Equal(t, 3, latest.Generation)
	})

	t.Run("returns error for non-existent family", func(t *testing.T) {
		_, err := repo.GetLatestTokenInFamily(ctx, uuid.Must(uuid.NewV4()).String())
		require.Error(t, err)
	})
}

func TestTokenRepository_ListWithFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("filters by mobile", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "mobileFilter")
		defer cleanupAccountRecords(t, db, account.ID)

		mobileToken := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(time.Hour),
			Mobile:    true,
		}
		err := repo.Create(ctx, mobileToken)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", mobileToken.ID)

		tokens, err := repo.List(ctx, map[string]interface{}{
			"mobile": true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, tokens)

		var found bool
		for _, tk := range tokens {
			if tk.ID == mobileToken.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by active", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "activeFilter")
		defer cleanupAccountRecords(t, db, account.ID)

		activeToken := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer testpkg.CleanupTableRecords(t, db, "auth.tokens", activeToken.ID)

		tokens, err := repo.List(ctx, map[string]interface{}{
			"active": true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, tokens)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestTokenRepository_CreateValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token
	ctx := testpkg.TenantContext(1)

	t.Run("rejects nil token", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// ============================================================================
// DeleteByTenantID Tests
// ============================================================================

func TestTokenRepository_DeleteByTenantID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Token

	t.Run("deletes tokens for tenant and returns count", func(t *testing.T) {
		// ARRANGE — create account + token scoped to a dedicated test tenant
		tenantID := int64(42)
		testpkg.EnsureTestTenant(t, db, tenantID)

		account := testpkg.CreateTestAccount(t, db, "deleteByTenant")
		defer cleanupAccountRecords(t, db, account.ID)

		token := testpkg.CreateTestTokenForTenant(t, db, tenantID, account.ID)
		// No defer cleanup needed — DeleteByTenantID is expected to remove it

		// ACT
		ctx := testpkg.TenantContext(tenantID)
		deleted, err := repo.DeleteByTenantIDReturning(ctx, tenantID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, deleted, 1)

		// Verify the token is actually gone
		_, findErr := repo.FindByID(ctx, token.ID)
		assert.Error(t, findErr, "token should no longer exist after DeleteByTenantID")
	})

	t.Run("returns zero when tenant has no tokens", func(t *testing.T) {
		// Use a tenant that exists but has no tokens
		tenantID := int64(43)
		testpkg.EnsureTestTenant(t, db, tenantID)
		ctx := testpkg.TenantContext(tenantID)

		deleted, err := repo.DeleteByTenantIDReturning(ctx, tenantID)
		require.NoError(t, err)
		assert.Empty(t, deleted)
	})
}
