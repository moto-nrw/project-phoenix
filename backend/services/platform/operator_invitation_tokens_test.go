package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func ensureOperatorInvitationTokenTable(tb testing.TB, db *bun.DB) {
	tb.Helper()

	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS platform.operator_invitation_tokens (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			email TEXT NOT NULL,
			token TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_by BIGINT NOT NULL REFERENCES platform.operators(id) ON DELETE CASCADE,
			display_name TEXT,
			email_sent_at TIMESTAMPTZ,
			email_error TEXT,
			email_retry_count INT NOT NULL DEFAULT 0
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_operator_invitation_tokens_token
			ON platform.operator_invitation_tokens(token);
		CREATE INDEX IF NOT EXISTS idx_operator_invitation_tokens_email
			ON platform.operator_invitation_tokens(email);
		CREATE INDEX IF NOT EXISTS idx_operator_invitation_tokens_expires_at
			ON platform.operator_invitation_tokens(expires_at);
		CREATE INDEX IF NOT EXISTS idx_operator_invitation_tokens_created_by
			ON platform.operator_invitation_tokens(created_by);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_operator_invitation_tokens_one_active_per_email
			ON platform.operator_invitation_tokens(email)
			WHERE used_at IS NULL;
	`)
	require.NoError(tb, err, "failed to ensure operator invitation token table exists")
}

func setupOperatorInvitationTokensTest(
	t *testing.T,
) (*bun.DB, platformSvc.OperatorInvitationTokens) {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	ensureOperatorInvitationTokenTable(t, db)

	tokens, err := services.NewOperatorInvitationTokensForTests(db)
	require.NoError(t, err)
	return db, tokens
}

func newTestToken(email string, createdBy int64) *platform.OperatorInvitationToken {
	return &platform.OperatorInvitationToken{
		Email:     email,
		Token:     uuid.Must(uuid.NewV4()).String(),
		ExpiresAt: time.Now().Add(48 * time.Hour),
		CreatedBy: createdBy,
	}
}

// =====================================================================
// Create Tests
// =====================================================================

func TestOperatorInvitationTokens_Create_Success(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("create-test@example.com", op.ID)

	err := repo.Create(context.Background(), token)
	require.NoError(t, err)
	assert.NotZero(t, token.ID)
	assert.NotZero(t, token.CreatedAt)
}

func TestOperatorInvitationTokens_Create_NilToken(t *testing.T) {
	t.Parallel()

	_, repo := setupOperatorInvitationTokensTest(t)

	err := repo.Create(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestOperatorInvitationTokens_Create_ValidationError(t *testing.T) {
	t.Parallel()

	_, repo := setupOperatorInvitationTokensTest(t)

	// Missing required fields
	token := &platform.OperatorInvitationToken{}
	err := repo.Create(context.Background(), token)
	require.Error(t, err)
}

// An invitation mail must never carry a link that is already dead, so the
// owner refuses to store one (#2722).
func TestOperatorInvitationTokens_Create_RefusesExpiredToken(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)
	token := newTestToken(fmt.Sprintf("expired-create-%d@example.com", time.Now().UnixNano()), op.ID)
	token.ExpiresAt = time.Now().Add(-time.Second)

	err := repo.Create(context.Background(), token)
	require.ErrorContains(t, err, "token has already expired")
	assert.Zero(t, token.ID, "a refused invitation gets no identity")
	found, err := repo.FindValidByToken(context.Background(), token.Token)
	require.NoError(t, err)
	assert.Nil(t, found)
	exists, err := db.NewSelect().Table("platform.operator_invitation_tokens").Where("token = ?", token.Token).Exists(context.Background())
	require.NoError(t, err)
	assert.False(t, exists, "nothing is stored")
}

// =====================================================================
// FindByID Tests
// =====================================================================

func TestOperatorInvitationTokens_FindByID_Success(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("findbyid@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	found, err := repo.FindByID(context.Background(), token.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, token.ID, found.ID)
	assert.Equal(t, "findbyid@example.com", found.Email)
}

func TestOperatorInvitationTokens_FindByID_NotFound(t *testing.T) {
	t.Parallel()

	_, repo := setupOperatorInvitationTokensTest(t)

	found, err := repo.FindByID(context.Background(), 999999)
	require.NoError(t, err)
	assert.Nil(t, found)
}

// =====================================================================
// FindValidByToken Tests
// =====================================================================

func TestOperatorInvitationTokens_FindValidByToken_Success(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("findvalid@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	found, err := repo.FindValidByToken(context.Background(), token.Token)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, token.ID, found.ID)
}

func TestOperatorInvitationTokens_FindValidByToken_Expired(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	// Insert directly with past expiry (bypassing model validation)
	expiredToken := &platform.OperatorInvitationToken{
		Email:     "expired-valid@example.com",
		Token:     uuid.Must(uuid.NewV4()).String(),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedBy: op.ID,
	}
	_, err := db.NewInsert().Model(expiredToken).Exec(context.Background())
	require.NoError(t, err)

	found, err := repo.FindValidByToken(context.Background(), expiredToken.Token)
	require.NoError(t, err)
	assert.Nil(t, found, "expired token should not be found as valid")
}

func TestOperatorInvitationTokens_FindValidByToken_Used(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("used-valid@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	// Mark as used
	_, err := repo.MarkAsUsed(context.Background(), token.ID)
	require.NoError(t, err)

	found, err := repo.FindValidByToken(context.Background(), token.Token)
	require.NoError(t, err)
	assert.Nil(t, found, "used token should not be found as valid")
}

func TestOperatorInvitationTokens_FindValidByToken_NotFound(t *testing.T) {
	t.Parallel()

	_, repo := setupOperatorInvitationTokensTest(t)

	found, err := repo.FindValidByToken(context.Background(), "nonexistent-token")
	require.NoError(t, err)
	assert.Nil(t, found)
}

// =====================================================================
// ConsumeByToken Tests
// =====================================================================

func TestOperatorInvitationTokens_ConsumeByToken_Success(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("consume@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	consumed, err := repo.ConsumeByToken(context.Background(), token.Token)
	require.NoError(t, err)
	require.NotNil(t, consumed)
	assert.Equal(t, token.ID, consumed.ID)
	assert.NotNil(t, consumed.UsedAt, "consumed token should have UsedAt set")
}

func TestOperatorInvitationTokens_ConsumeByToken_AlreadyConsumed(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("double-consume@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	// First consume succeeds
	consumed, err := repo.ConsumeByToken(context.Background(), token.Token)
	require.NoError(t, err)
	require.NotNil(t, consumed)

	// Second consume returns nil (already consumed)
	consumed2, err := repo.ConsumeByToken(context.Background(), token.Token)
	require.NoError(t, err)
	assert.Nil(t, consumed2, "second consume should return nil")
}

func TestOperatorInvitationTokens_ConsumeByToken_NotFound(t *testing.T) {
	t.Parallel()

	_, repo := setupOperatorInvitationTokensTest(t)

	consumed, err := repo.ConsumeByToken(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, consumed)
}

// =====================================================================
// MarkAsUsed Tests
// =====================================================================

func TestOperatorInvitationTokens_MarkAsUsed_Success(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("markused@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	marked, err := repo.MarkAsUsed(context.Background(), token.ID)
	require.NoError(t, err)
	assert.True(t, marked)

	// Verify it's now used
	found, err := repo.FindByID(context.Background(), token.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.NotNil(t, found.UsedAt)
}

func TestOperatorInvitationTokens_MarkAsUsed_AlreadyUsed(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("markused-twice@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	// First mark succeeds
	marked, err := repo.MarkAsUsed(context.Background(), token.ID)
	require.NoError(t, err)
	assert.True(t, marked)

	// Second mark returns false (already used)
	marked2, err := repo.MarkAsUsed(context.Background(), token.ID)
	require.NoError(t, err)
	assert.False(t, marked2)
}

// =====================================================================
// ListPending Tests
// =====================================================================

// The operator invitation list reads the redeemable links in one statement,
// however many there are.
func TestOperatorInvitationTokens_ListPendingQueryBudget(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)
	add := func(from, to int) {
		for i := from; i < to; i++ {
			token := newTestToken(fmt.Sprintf("budget%d-%d@example.com", i, time.Now().UnixNano()), op.ID)
			require.NoError(t, repo.Create(context.Background(), token))
		}
	}
	add(0, 3)

	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(context.Background())
	run := func() []string {
		counter.Reset()
		_, err := repo.ListPending(ctx)
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large), "the listing must not grow a statement per link")
	testpkg.AssertQueryBudget(t, "services.platform.pending_operator_invitations.reads", large)
}

func TestOperatorInvitationTokens_ListPending(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	// Create two pending tokens
	token1 := newTestToken("pending1@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token1))

	// Need to invalidate the first to avoid unique constraint (one active per email)
	// Use a different email for the second
	token2 := newTestToken("pending2@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token2))

	// Create one used token (should NOT appear in pending)
	token3 := newTestToken("used-pending@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token3))
	_, err := repo.MarkAsUsed(context.Background(), token3.ID)
	require.NoError(t, err)

	pending, err := repo.ListPending(context.Background())
	require.NoError(t, err)

	// Find our test tokens in the results (other tests may leave data)
	var foundIDs []int64
	for _, p := range pending {
		if p.CreatedBy == op.ID {
			foundIDs = append(foundIDs, p.ID)
		}
	}

	assert.Len(t, foundIDs, 2, "should have exactly 2 pending tokens for our test operator")
	assert.Contains(t, foundIDs, token1.ID)
	assert.Contains(t, foundIDs, token2.ID)
}

// =====================================================================
// ExtendExpiry Tests
// =====================================================================

func TestOperatorInvitationTokens_ExtendExpiry_Success(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("extend@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	newExpiry := time.Now().Add(96 * time.Hour)
	extended, err := repo.ExtendExpiry(context.Background(), token.ID, newExpiry)
	require.NoError(t, err)
	assert.True(t, extended)

	found, err := repo.FindByID(context.Background(), token.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.WithinDuration(t, newExpiry, found.ExpiresAt, 2*time.Second)
}

func TestOperatorInvitationTokens_ExtendExpiry_AlreadyUsed(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("extend-used@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	_, err := repo.MarkAsUsed(context.Background(), token.ID)
	require.NoError(t, err)

	newExpiry := time.Now().Add(96 * time.Hour)
	extended, err := repo.ExtendExpiry(context.Background(), token.ID, newExpiry)
	require.NoError(t, err)
	assert.False(t, extended, "should not extend expiry of used token")
}

// =====================================================================
// InvalidateByEmail Tests
// =====================================================================

func TestOperatorInvitationTokens_InvalidateByEmail(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	targetEmail := uuid.Must(uuid.NewV4()).String() + "-invalidate@example.com"
	token := newTestToken(targetEmail, op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	invalidated, err := repo.InvalidateByEmail(context.Background(), targetEmail)
	require.NoError(t, err)
	assert.Equal(t, 1, invalidated)

	// Verify the token is now used
	found, err := repo.FindByID(context.Background(), token.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.NotNil(t, found.UsedAt)
}

func TestOperatorInvitationTokens_InvalidateByEmail_NoMatch(t *testing.T) {
	t.Parallel()

	_, repo := setupOperatorInvitationTokensTest(t)

	invalidated, err := repo.InvalidateByEmail(context.Background(), "nonexistent-invalidate@example.com")
	require.NoError(t, err)
	assert.Equal(t, 0, invalidated)
}

// =====================================================================
// UpdateDeliveryResult Tests
// =====================================================================

func TestOperatorInvitationTokens_UpdateDeliveryResult_Success(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("delivery@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	sentAt := time.Now()
	err := repo.UpdateDeliveryResult(context.Background(), token.ID, &sentAt, nil, 1)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), token.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.NotNil(t, found.EmailSentAt)
	assert.Nil(t, found.EmailError)
	assert.Equal(t, 1, found.EmailRetryCount)
}

func TestOperatorInvitationTokens_UpdateDeliveryResult_WithError(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("delivery-err@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	errText := "SMTP timeout"
	err := repo.UpdateDeliveryResult(context.Background(), token.ID, nil, &errText, 3)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), token.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Nil(t, found.EmailSentAt)
	require.NotNil(t, found.EmailError)
	assert.Equal(t, "SMTP timeout", *found.EmailError)
	assert.Equal(t, 3, found.EmailRetryCount)
}

func TestOperatorInvitationTokens_UpdateDeliveryResult_ClearsOnReset(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	token := newTestToken("delivery-reset@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), token))

	// First set error state
	errText := "failure"
	err := repo.UpdateDeliveryResult(context.Background(), token.ID, nil, &errText, 2)
	require.NoError(t, err)

	// Then reset (e.g. on resend)
	err = repo.UpdateDeliveryResult(context.Background(), token.ID, nil, nil, 0)
	require.NoError(t, err)

	found, err := repo.FindByID(context.Background(), token.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Nil(t, found.EmailSentAt)
	assert.Nil(t, found.EmailError)
	assert.Equal(t, 0, found.EmailRetryCount)
}

// =====================================================================
// DeleteExpired Tests
// =====================================================================

func TestOperatorInvitationTokens_DeleteExpired(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	op := testpkg.CreateTestOperator(t, db)

	// Insert an expired token directly (bypassing model validation)
	expiredToken := &platform.OperatorInvitationToken{
		Email:     uuid.Must(uuid.NewV4()).String() + "-expired-del@example.com",
		Token:     uuid.Must(uuid.NewV4()).String(),
		ExpiresAt: time.Now().Add(-24 * time.Hour),
		CreatedBy: op.ID,
	}
	_, err := db.NewInsert().Model(expiredToken).Exec(context.Background())
	require.NoError(t, err)

	// Also create a valid (non-expired) token
	validToken := newTestToken("valid-del@example.com", op.ID)
	require.NoError(t, repo.Create(context.Background(), validToken))

	deleted, err := repo.DeleteExpired(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1, "should delete at least our expired token")

	// Verify expired token is gone
	found, err := repo.FindByID(context.Background(), expiredToken.ID)
	require.NoError(t, err)
	assert.Nil(t, found, "expired token should be deleted")

	// Verify valid token still exists
	found, err = repo.FindByID(context.Background(), validToken.ID)
	require.NoError(t, err)
	assert.NotNil(t, found, "valid token should not be deleted")
}

// =====================================================================
// CountRecentByCreatedBy Tests (rate limiting)
// =====================================================================

func TestOperatorInvitationTokens_CountRecentByCreatedBy(t *testing.T) {
	t.Parallel()

	db, repo := setupOperatorInvitationTokensTest(t)
	ctx := context.Background()

	t.Run("NoTokens_ReturnsZero", func(t *testing.T) {
		op := testpkg.CreateTestOperator(t, db)

		count, err := repo.CountRecentByCreatedBy(ctx, op.ID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("CountsAllTokensInWindow_PendingAndUsed", func(t *testing.T) {
		op := testpkg.CreateTestOperator(t, db)

		// Two pending tokens
		for i := 0; i < 2; i++ {
			token := newTestToken(fmt.Sprintf("pending-%d-%d@test.local", i, time.Now().UnixNano()), op.ID)
			require.NoError(t, repo.Create(ctx, token))
		}
		// One used token
		usedToken := newTestToken(fmt.Sprintf("used-%d@test.local", time.Now().UnixNano()), op.ID)
		require.NoError(t, repo.Create(ctx, usedToken))
		_, err := repo.MarkAsUsed(ctx, usedToken.ID)
		require.NoError(t, err)

		count, err := repo.CountRecentByCreatedBy(ctx, op.ID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 3, count, "should count pending and used tokens regardless of state")
	})

	t.Run("IgnoresTokensOutsideWindow", func(t *testing.T) {
		op := testpkg.CreateTestOperator(t, db)

		// Insert a token with created_at well in the past (2 hours ago).
		// Using raw SQL because repo.Create sets created_at to NOW() via the
		// base.Model hooks.
		tokenStr := uuid.Must(uuid.NewV4()).String()
		oldEmail := fmt.Sprintf("old-%d@test.local", time.Now().UnixNano())
		pastTime := time.Now().Add(-2 * time.Hour)
		_, err := db.ExecContext(ctx,
			`INSERT INTO platform.operator_invitation_tokens
			 (email, token, expires_at, created_by, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			oldEmail, tokenStr, time.Now().Add(48*time.Hour), op.ID, pastTime, pastTime,
		)
		require.NoError(t, err)

		// Insert a recent token
		recent := newTestToken(fmt.Sprintf("recent-%d@test.local", time.Now().UnixNano()), op.ID)
		require.NoError(t, repo.Create(ctx, recent))

		count, err := repo.CountRecentByCreatedBy(ctx, op.ID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 1, count, "should only count tokens created within the window")
	})

	t.Run("ScopesByCreatedBy", func(t *testing.T) {
		opA := testpkg.CreateTestOperator(t, db)
		opB := testpkg.CreateTestOperator(t, db)

		// Three tokens from opA, one from opB
		for i := 0; i < 3; i++ {
			require.NoError(t, repo.Create(ctx, newTestToken(fmt.Sprintf("a-%d-%d@test.local", i, time.Now().UnixNano()), opA.ID)))
		}
		require.NoError(t, repo.Create(ctx, newTestToken(fmt.Sprintf("b-%d@test.local", time.Now().UnixNano()), opB.ID)))

		countA, err := repo.CountRecentByCreatedBy(ctx, opA.ID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 3, countA)

		countB, err := repo.CountRecentByCreatedBy(ctx, opB.ID, time.Now().Add(-1*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 1, countB)
	})
}
