package auth_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestPasswordResetTokenRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("creates password reset token with valid data", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "resetToken")

		token := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}

		err := repo.Create(ctx, token)
		require.NoError(t, err)
		assert.NotZero(t, token.ID)

	})

	t.Run("rejects nil token", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestPasswordResetTokenRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("finds existing token by ID", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "resetFindByID")

		tokenStr := uuid.Must(uuid.NewV4()).String()
		token := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     tokenStr,
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, token.ID)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestPasswordResetTokenRepository_FindByToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("finds token by token string", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "resetFindByToken")

		tokenStr := uuid.Must(uuid.NewV4()).String()
		token := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     tokenStr,
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		found, err := repo.FindByToken(ctx, tokenStr)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
	})

	t.Run("returns error for non-existent token string", func(t *testing.T) {
		_, err := repo.FindByToken(ctx, uuid.Must(uuid.NewV4()).String())
		require.Error(t, err)
	})
}

func TestPasswordResetTokenRepository_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("finds all tokens for account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "resetByAccount")

		// Create two tokens
		for i := 0; i < 2; i++ {
			token := &auth.PasswordResetToken{
				AccountID: account.ID,
				Token:     uuid.Must(uuid.NewV4()).String(),
				Expiry:    time.Now().Add(30 * time.Minute),
				Used:      false,
			}
			err := repo.Create(ctx, token)
			require.NoError(t, err)
		}

		tokens, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(tokens), 2)
	})

	t.Run("returns empty for account with no tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "resetNoTokens")

		tokens, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, tokens)
	})
}

func TestPasswordResetTokenRepository_FindValidByToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("finds valid token", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "validResetToken")

		tokenStr := uuid.Must(uuid.NewV4()).String()
		token := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     tokenStr,
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		found, err := repo.FindValidByToken(ctx, tokenStr)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
		assert.False(t, found.Used)
	})

	t.Run("rejects used token", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "usedResetToken")

		tokenStr := uuid.Must(uuid.NewV4()).String()
		token := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     tokenStr,
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		// Mark as used via the repository method
		err = repo.MarkAsUsed(ctx, token.ID)
		require.NoError(t, err)

		_, err = repo.FindValidByToken(ctx, tokenStr)
		require.Error(t, err)
	})
}

func TestPasswordResetTokenRepository_MarkAsUsed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("marks token as used", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "markUsed")

		token := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		err = repo.MarkAsUsed(ctx, token.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, token.ID)
		require.NoError(t, err)
		assert.True(t, found.Used)
	})
}

func TestPasswordResetTokenRepository_InvalidateTokensByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("marks all tokens as used for account", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "invalidateAll")

		// Create two tokens
		for i := 0; i < 2; i++ {
			token := &auth.PasswordResetToken{
				AccountID: account.ID,
				Token:     uuid.Must(uuid.NewV4()).String(),
				Expiry:    time.Now().Add(30 * time.Minute),
				Used:      false,
			}
			err := repo.Create(ctx, token)
			require.NoError(t, err)
		}

		// Invalidate all
		err := repo.InvalidateTokensByAccountID(ctx, account.ID)
		require.NoError(t, err)

		// Verify all marked as used
		tokens, err := repo.FindByAccountID(ctx, account.ID)
		require.NoError(t, err)
		for _, tk := range tokens {
			assert.True(t, tk.Used)
		}
	})
}

func TestPasswordResetTokenRepository_DeleteExpiredTokens(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("deletes expired and used tokens", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deleteExpired")

		// Create token and mark as used
		usedToken := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err := repo.Create(ctx, usedToken)
		require.NoError(t, err)
		usedTokenID := usedToken.ID

		// Mark as used via the repository method
		err = repo.MarkAsUsed(ctx, usedTokenID)
		require.NoError(t, err)

		// Create valid token
		validToken := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err = repo.Create(ctx, validToken)
		require.NoError(t, err)

		// Delete expired/used
		deleted, err := repo.DeleteExpiredTokens(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, 1)

		// Verify used is gone
		_, err = repo.FindByID(ctx, usedTokenID)
		require.Error(t, err)

		// Verify valid still exists
		_, err = repo.FindByID(ctx, validToken.ID)
		require.NoError(t, err)
	})
}

func TestPasswordResetTokenRepository_UpdateDeliveryResult(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).PasswordResetToken
	ctx := testpkg.Ctx(t)

	t.Run("updates delivery metadata", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "deliveryResult")

		token := &auth.PasswordResetToken{
			AccountID: account.ID,
			Token:     uuid.Must(uuid.NewV4()).String(),
			Expiry:    time.Now().Add(30 * time.Minute),
			Used:      false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		sentAt := time.Now()
		emailError := "SMTP connection failed"
		err = repo.UpdateDeliveryResult(ctx, token.ID, &sentAt, &emailError, 1)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, token.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.EmailSentAt)
		assert.NotNil(t, found.EmailError)
		assert.Equal(t, 1, found.EmailRetryCount)
	})
}
