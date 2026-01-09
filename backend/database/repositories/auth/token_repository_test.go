package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupTokenRepo(t *testing.T, db *bun.DB) auth.TokenRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Token
}

// cleanupTokenRecords removes tokens directly
func cleanupTokenRecords(t *testing.T, db *bun.DB, tokenIDs ...int64) {
	t.Helper()
	if len(tokenIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("auth.tokens").
		Where("id IN (?)", bun.In(tokenIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup tokens: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestTokenRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

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

		cleanupTokenRecords(t, db, token.ID)
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

		cleanupTokenRecords(t, db, token.ID)
	})
}

func TestTokenRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("finds existing token", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenFindByID")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupTokenRecords(t, db, token.ID)

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

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("finds token by token string", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenFindByToken")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupTokenRecords(t, db, token.ID)

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

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("updates token identifier", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenUpdate")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupTokenRecords(t, db, token.ID)

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

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

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

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("lists all tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenList")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupTokenRecords(t, db, token.ID)

		tokens, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, tokens)
	})
}

func TestTokenRepository_FindByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("finds tokens by account ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "tokenByAccount")
		token := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupAccountRecords(t, db, account.ID)
		defer cleanupTokenRecords(t, db, token.ID)

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

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("deletes expired tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "expiredToken")
		defer cleanupAccountRecords(t, db, account.ID)

		// Create expired token directly using raw SQL (bypassing validation)
		expiredTokenStr := uuid.Must(uuid.NewV4()).String()
		var expiredTokenID int64
		err := db.NewRaw(`
			INSERT INTO auth.tokens (account_id, token, expiry, mobile, family_id)
			VALUES (?, ?, ?, false, ?)
			RETURNING id
		`, account.ID, expiredTokenStr, time.Now().Add(-time.Hour), uuid.Must(uuid.NewV4()).String()).
			Scan(ctx, &expiredTokenID)
		require.NoError(t, err)

		// Create valid token
		validToken := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		defer cleanupTokenRecords(t, db, validToken.ID)

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

func TestTokenRepository_DeleteByAccountID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("deletes all tokens for account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deleteByAccount")
		token1 := testpkg.CreateTestToken(t, db, account.ID, "refresh")
		token2 := testpkg.CreateTestToken(t, db, account.ID, "access")
		defer cleanupAccountRecords(t, db, account.ID)

		err := repo.DeleteByAccountID(ctx, account.ID)
		require.NoError(t, err)

		// Verify tokens are gone
		_, err = repo.FindByID(ctx, token1.ID)
		require.Error(t, err)
		_, err = repo.FindByID(ctx, token2.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Token Family Tests
// ============================================================================

func TestTokenRepository_FindByFamilyID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

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
		defer cleanupTokenRecords(t, db, token1.ID)

		token2 := &auth.Token{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(time.Hour),
			FamilyID:  familyID,
		}
		err = repo.Create(ctx, token2)
		require.NoError(t, err)
		defer cleanupTokenRecords(t, db, token2.ID)

		// Find by family
		tokens, err := repo.FindByFamilyID(ctx, familyID)
		require.NoError(t, err)
		assert.Len(t, tokens, 2)
	})
}

func TestTokenRepository_DeleteByFamilyID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

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
		err = repo.DeleteByFamilyID(ctx, familyID)
		require.NoError(t, err)

		// Verify tokens are gone
		tokens, err := repo.FindByFamilyID(ctx, familyID)
		require.NoError(t, err)
		assert.Empty(t, tokens)
	})
}

func TestTokenRepository_CleanupOldTokensForAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupTokenRepo(t, db)
	ctx := context.Background()

	t.Run("keeps only specified number of tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "cleanupTokens")
		defer cleanupAccountRecords(t, db, account.ID)

		// Create 5 tokens
		var tokenIDs []int64
		for i := 0; i < 5; i++ {
			token := &auth.Token{
				AccountID: account.ID,
				Token:     fmt.Sprintf("cleanup-token-%d-%d", time.Now().UnixNano(), i),
				Expiry:    time.Now().Add(time.Hour),
			}
			err := repo.Create(ctx, token)
			require.NoError(t, err)
			tokenIDs = append(tokenIDs, token.ID)
			time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		}
		defer func() {
			for _, id := range tokenIDs {
				cleanupTokenRecords(t, db, id)
			}
		}()

		// Cleanup, keeping only 2
		err := repo.CleanupOldTokensForAccount(ctx, account.ID, 2)
		require.NoError(t, err)

		// Verify only 2 remain
		tokens, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(tokens), 2)
	})
}
