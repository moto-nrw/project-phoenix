package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repoplatform "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOperatorEmailChangeTokenRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repoplatform.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		email := fmt.Sprintf("create-ok-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		token := &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   "new@test.local",
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)
		assert.NotZero(t, token.ID, "token ID should be set after create")
	})

	t.Run("NilToken", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("ValidationFailure_EmptyToken", func(t *testing.T) {
		email := fmt.Sprintf("create-val-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		token := &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   "new@test.local",
			Token:      "", // empty token string
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token value is required")
	})

	t.Run("ValidationFailure_ExpiredToken", func(t *testing.T) {
		email := fmt.Sprintf("create-exp-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		token := &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   "new@test.local",
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(-10 * time.Minute), // expired
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("ValidationFailure_MissingOperatorID", func(t *testing.T) {
		token := &platform.OperatorEmailChangeToken{
			OperatorID: 0, // missing
			NewEmail:   "new@test.local",
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "operator ID is required")
	})

	t.Run("ValidationFailure_MissingNewEmail", func(t *testing.T) {
		email := fmt.Sprintf("create-noemail-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		token := &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   "", // missing
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "new email is required")
	})
}

func TestOperatorEmailChangeTokenRepository_UpdateDeliveryResult(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repoplatform.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("delivery-%d@test.local", time.Now().UnixNano())
	operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

	// Helper to create a fresh token for each sub-test.
	createToken := func(t *testing.T) *platform.OperatorEmailChangeToken {
		t.Helper()
		// Invalidate previous tokens so the unique index doesn't block us.
		_ = repo.InvalidateByOperatorID(ctx, operatorID)

		token := &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   fmt.Sprintf("delivery-new-%d@test.local", time.Now().UnixNano()),
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)
		return token
	}

	t.Run("Success_WithSentAt", func(t *testing.T) {
		token := createToken(t)
		sentAt := time.Now().Truncate(time.Second)

		err := repo.UpdateDeliveryResult(ctx, token.ID, &sentAt, nil, 1)
		require.NoError(t, err)

		// Verify the fields were updated
		var emailSentAt *time.Time
		var emailError *string
		var retryCount int
		err = db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Column("email_sent_at", "email_error", "email_retry_count").
			Where("id = ?", token.ID).
			Scan(ctx, &emailSentAt, &emailError, &retryCount)
		require.NoError(t, err)
		require.NotNil(t, emailSentAt)
		assert.WithinDuration(t, sentAt, *emailSentAt, time.Second)
		assert.Nil(t, emailError)
		assert.Equal(t, 1, retryCount)
	})

	t.Run("Success_WithError", func(t *testing.T) {
		token := createToken(t)
		errMsg := "SMTP timeout"

		err := repo.UpdateDeliveryResult(ctx, token.ID, nil, &errMsg, 3)
		require.NoError(t, err)

		var emailSentAt *time.Time
		var emailError *string
		var retryCount int
		err = db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Column("email_sent_at", "email_error", "email_retry_count").
			Where("id = ?", token.ID).
			Scan(ctx, &emailSentAt, &emailError, &retryCount)
		require.NoError(t, err)
		assert.Nil(t, emailSentAt, "sentAt should be NULL when not provided")
		require.NotNil(t, emailError)
		assert.Equal(t, "SMTP timeout", *emailError)
		assert.Equal(t, 3, retryCount)
	})

	t.Run("Success_ClearsBothFields", func(t *testing.T) {
		token := createToken(t)

		// First set an error
		errMsg := "temporary failure"
		err := repo.UpdateDeliveryResult(ctx, token.ID, nil, &errMsg, 1)
		require.NoError(t, err)

		// Then clear both (success with nil sentAt and nil error)
		err = repo.UpdateDeliveryResult(ctx, token.ID, nil, nil, 2)
		require.NoError(t, err)

		var emailSentAt *time.Time
		var emailError *string
		var retryCount int
		err = db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Column("email_sent_at", "email_error", "email_retry_count").
			Where("id = ?", token.ID).
			Scan(ctx, &emailSentAt, &emailError, &retryCount)
		require.NoError(t, err)
		assert.Nil(t, emailSentAt)
		assert.Nil(t, emailError, "error should be cleared")
		assert.Equal(t, 2, retryCount)
	})

	t.Run("TruncatesLongErrorMessage", func(t *testing.T) {
		token := createToken(t)

		// Create a message longer than 1024 characters
		longErr := ""
		for len(longErr) < 2000 {
			longErr += "x"
		}
		err := repo.UpdateDeliveryResult(ctx, token.ID, nil, &longErr, 1)
		require.NoError(t, err)

		var emailError *string
		err = db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Column("email_error").
			Where("id = ?", token.ID).
			Scan(ctx, &emailError)
		require.NoError(t, err)
		require.NotNil(t, emailError)
		assert.LessOrEqual(t, len(*emailError), 1024, "error message should be truncated to 1024 chars")
	})
}

func TestOperatorEmailChangeTokenRepository_ConsumeByToken(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repoplatform.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	t.Run("NonexistentToken_ReturnsNilNil", func(t *testing.T) {
		token, err := repo.ConsumeByToken(ctx, "definitely-does-not-exist")
		require.NoError(t, err)
		assert.Nil(t, token)
	})

	t.Run("AlreadyUsedToken_ReturnsNilNil", func(t *testing.T) {
		email := fmt.Sprintf("consume-used-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		tokenStr := uuid.Must(uuid.NewV4()).String()
		_, err := db.ExecContext(ctx,
			`INSERT INTO platform.operator_email_change_tokens
			 (operator_id, new_email, token, expiry, used)
			 VALUES (?, ?, ?, ?, true)`,
			operatorID, "consumed@test.local", tokenStr, time.Now().Add(30*time.Minute),
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE operator_id = ?`, operatorID)
		})

		token, err := repo.ConsumeByToken(ctx, tokenStr)
		require.NoError(t, err)
		assert.Nil(t, token, "used token should not be consumable")
	})
}

func TestOperatorEmailChangeTokenRepository_InvalidateByOperatorID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repoplatform.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	t.Run("NoTokens_NoError", func(t *testing.T) {
		email := fmt.Sprintf("inv-none-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		err := repo.InvalidateByOperatorID(ctx, operatorID)
		require.NoError(t, err, "invalidating with no tokens should not error")
	})

	t.Run("InvalidatesActiveToken", func(t *testing.T) {
		email := fmt.Sprintf("inv-active-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		token := &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   "inv-target@test.local",
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		err = repo.InvalidateByOperatorID(ctx, operatorID)
		require.NoError(t, err)

		// Verify the token is now used
		var used bool
		err = db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Column("used").
			Where("id = ?", token.ID).
			Scan(ctx, &used)
		require.NoError(t, err)
		assert.True(t, used, "token should be marked as used after invalidation")
	})
}

func TestOperatorEmailChangeTokenRepository_InvalidateExpiredTokens(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repoplatform.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	t.Run("NoExpiredTokens", func(t *testing.T) {
		count, err := repo.InvalidateExpiredTokens(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("InvalidatesExpiredUnused", func(t *testing.T) {
		email := fmt.Sprintf("inv-exp-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		tokenStr := uuid.Must(uuid.NewV4()).String()
		_, err := db.ExecContext(ctx,
			`INSERT INTO platform.operator_email_change_tokens
			 (operator_id, new_email, token, expiry, used)
			 VALUES (?, ?, ?, ?, false)`,
			operatorID, "exp@test.local", tokenStr, time.Now().Add(-5*time.Minute),
		)
		require.NoError(t, err)

		count, err := repo.InvalidateExpiredTokens(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)

		var used bool
		err = db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Column("used").
			Where("token = ?", tokenStr).
			Scan(ctx, &used)
		require.NoError(t, err)
		assert.True(t, used)
	})
}

func TestOperatorEmailChangeTokenRepository_DeleteStaleTokens(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repoplatform.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	t.Run("NoStaleTokens", func(t *testing.T) {
		count, err := repo.DeleteStaleTokens(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})

	t.Run("DeletesOldUsedToken", func(t *testing.T) {
		email := fmt.Sprintf("del-stale-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		tokenStr := uuid.Must(uuid.NewV4()).String()
		_, err := db.ExecContext(ctx,
			`INSERT INTO platform.operator_email_change_tokens
			 (operator_id, new_email, token, expiry, used, created_at)
			 VALUES (?, ?, ?, ?, true, NOW() - INTERVAL '2 hours')`,
			operatorID, "stale@test.local", tokenStr, time.Now().Add(-90*time.Minute),
		)
		require.NoError(t, err)

		count, err := repo.DeleteStaleTokens(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 1)

		rowCount, err := db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Where("token = ?", tokenStr).
			Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, rowCount, "stale token should be deleted")
	})

	t.Run("PreservesRecentToken", func(t *testing.T) {
		email := fmt.Sprintf("del-recent-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		tokenStr := uuid.Must(uuid.NewV4()).String()
		_, err := db.ExecContext(ctx,
			`INSERT INTO platform.operator_email_change_tokens
			 (operator_id, new_email, token, expiry, used)
			 VALUES (?, ?, ?, ?, true)`,
			operatorID, "recent@test.local", tokenStr, time.Now().Add(-5*time.Minute),
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.ExecContext(ctx, `DELETE FROM platform.operator_email_change_tokens WHERE token = ?`, tokenStr)
		})

		_, err = repo.DeleteStaleTokens(ctx)
		require.NoError(t, err)

		rowCount, err := db.NewSelect().
			TableExpr("platform.operator_email_change_tokens").
			Where("token = ?", tokenStr).
			Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, rowCount, "recent used token should be preserved")
	})
}

func TestOperatorEmailChangeTokenRepository_CountRecentByOperatorID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repoplatform.NewOperatorEmailChangeTokenRepository(db)
	ctx := context.Background()

	t.Run("NoTokens_ReturnsZero", func(t *testing.T) {
		email := fmt.Sprintf("count-none-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		count, err := repo.CountRecentByOperatorID(ctx, operatorID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("CountsAllTokensInWindow", func(t *testing.T) {
		email := fmt.Sprintf("count-all-%d@test.local", time.Now().UnixNano())
		operatorID := testpkg.CreateTestOperatorWithEmail(t, db, email, "Test Operator").ID

		// Insert 3 tokens (2 used, 1 active) — all recent
		for i := range 2 {
			tokenStr := uuid.Must(uuid.NewV4()).String()
			_, err := db.ExecContext(ctx,
				`INSERT INTO platform.operator_email_change_tokens
				 (operator_id, new_email, token, expiry, used)
				 VALUES (?, ?, ?, ?, true)`,
				operatorID, fmt.Sprintf("count%d@test.local", i), tokenStr, time.Now().Add(30*time.Minute),
			)
			require.NoError(t, err)
		}
		// One active token
		token := &platform.OperatorEmailChangeToken{
			OperatorID: operatorID,
			NewEmail:   "count-active@test.local",
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     time.Now().Add(30 * time.Minute),
			Used:       false,
		}
		err := repo.Create(ctx, token)
		require.NoError(t, err)

		count, err := repo.CountRecentByOperatorID(ctx, operatorID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 3, count, "should count all tokens regardless of used status")
	})
}
