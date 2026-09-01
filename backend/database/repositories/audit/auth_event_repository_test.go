package audit

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestAuthEventRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewAuthEventRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	// Create a test account
	account := testpkg.CreateTestAccount(t, db, "auth_event_test@example.com")

	t.Run("creates auth event with valid data", func(t *testing.T) {
		event := audit.NewAuthEvent(
			account.ID,
			audit.EventTypeLogin,
			true,
			"192.168.1.1",
		)

		err := repo.Create(ctx, event)
		require.NoError(t, err)
		assert.NotZero(t, event.ID)

	})

	t.Run("creates failed login event", func(t *testing.T) {
		event := audit.NewAuthEvent(
			account.ID,
			audit.EventTypeLogin,
			false,
			"10.0.0.1",
		)
		event.ErrorMessage = "Invalid credentials"
		event.UserAgent = "Mozilla/5.0"

		err := repo.Create(ctx, event)
		require.NoError(t, err)
		assert.NotZero(t, event.ID)
		assert.False(t, event.Success)

	})

	t.Run("creates token refresh event", func(t *testing.T) {
		event := audit.NewAuthEvent(
			account.ID,
			audit.EventTypeTokenRefresh,
			true,
			"172.16.0.1",
		)
		event.SetMetadata("token_family", "family-123")

		err := repo.Create(ctx, event)
		require.NoError(t, err)
		assert.NotZero(t, event.ID)

	})

	t.Run("creates password reset event", func(t *testing.T) {
		event := audit.NewAuthEvent(
			account.ID,
			audit.EventTypePasswordReset,
			true,
			"192.168.1.100",
		)

		err := repo.Create(ctx, event)
		require.NoError(t, err)
		assert.NotZero(t, event.ID)

	})
}

func TestAuthEventRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewAuthEventRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "find_event@example.com")

	t.Run("finds existing auth event", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, event.ID, found.ID)
		assert.Equal(t, account.ID, found.AccountID)
	})

	t.Run("returns error for non-existent event", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestAuthEventRepository_FindByAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewAuthEventRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	account1 := testpkg.CreateTestAccount(t, db, "account1@example.com")
	account2 := testpkg.CreateTestAccount(t, db, "account2@example.com")

	t.Run("finds events by account ID", func(t *testing.T) {
		event1 := audit.NewAuthEvent(account1.ID, audit.EventTypeLogin, true, "192.168.1.1")
		event2 := audit.NewAuthEvent(account1.ID, audit.EventTypeLogout, true, "192.168.1.1")
		event3 := audit.NewAuthEvent(account2.ID, audit.EventTypeLogin, true, "192.168.1.2")

		err := repo.Create(ctx, event1)
		require.NoError(t, err)
		err = repo.Create(ctx, event2)
		require.NoError(t, err)
		err = repo.Create(ctx, event3)
		require.NoError(t, err)

		events, err := repo.FindByAccountID(ctx, account1.ID, 10)
		require.NoError(t, err)
		assert.Len(t, events, 2)

		for _, e := range events {
			assert.Equal(t, account1.ID, e.AccountID)
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		event1 := audit.NewAuthEvent(account1.ID, audit.EventTypeLogin, true, "192.168.1.1")
		event2 := audit.NewAuthEvent(account1.ID, audit.EventTypeLogout, true, "192.168.1.1")
		event3 := audit.NewAuthEvent(account1.ID, audit.EventTypeTokenRefresh, true, "192.168.1.1")

		err := repo.Create(ctx, event1)
		require.NoError(t, err)
		err = repo.Create(ctx, event2)
		require.NoError(t, err)
		err = repo.Create(ctx, event3)
		require.NoError(t, err)

		events, err := repo.FindByAccountID(ctx, account1.ID, 2)
		require.NoError(t, err)
		assert.Len(t, events, 2)
	})
}

func TestAuthEventRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewAuthEventRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "list@example.com")

	t.Run("lists all events", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)

		events, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, events)
	})

	t.Run("lists with filters", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogout, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)

		filters := map[string]interface{}{
			"event_type": audit.EventTypeLogout,
			"success":    true,
		}
		events, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, e := range events {
			assert.Equal(t, audit.EventTypeLogout, e.EventType)
			assert.True(t, e.Success)
		}
	})
}

// Deliberately NOT parallel: unscoped sweep — ListPendingAccountWideWipes
// queries across all tenants and the assertion pins the exact result count,
// so the pending event a parallel neighbour creates lands in it (CI found
// this; the neighbour claims its event again, but not before this test
// looks).
func TestAuthEventRepository_PendingAccountWideWipes(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := NewAuthEventRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "pending_wipe@example.com")

	event := audit.NewAuthEvent(account.ID, audit.EventTypeTokenRevoked, true, "0.0.0.0")
	event.SetMetadata("reason", "password_reset")
	event.SetMetadata("pending_account_wide_wipe", true)
	require.NoError(t, repo.Create(ctx, event))

	pending, err := repo.ListPendingAccountWideWipes(ctx, event.CreatedAt.Add(-time.Minute))
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, account.ID, pending[0].AccountID)
	assert.Equal(t, "password_reset", pending[0].Reason)
	assert.False(t, pending[0].CreatedAt.IsZero())

	completed := audit.NewAuthEvent(account.ID, audit.EventTypeAccountWideWipeCompleted, true, "0.0.0.0")
	completed.SetMetadata("pending_event_id", event.ID)
	require.NoError(t, repo.Create(ctx, completed))
	pending, err = repo.ListPendingAccountWideWipes(ctx, event.CreatedAt.Add(-time.Minute))
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// Deliberately NOT parallel: unscoped sweep — ListPendingAccountWideWipes
// queries across all tenants, so a parallel test's revocation event lands in
// the result and the "recent only" assertion sees it.
func TestAuthEventRepository_ListPendingAccountWideWipesIncludesOlderThanSevenDays(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := NewAuthEventRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "old_pending_wipe@example.com")

	createdAt := time.Now().Add(-8 * 24 * time.Hour)
	_, err := db.NewRaw(`
		INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address, metadata, created_at)
		VALUES (?, ?, 'token_revoked', true, '0.0.0.0', jsonb_build_object('reason', 'password_reset', 'pending_account_wide_wipe', true), ?)
	`, testpkg.Tenant(t), account.ID, createdAt).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewDelete().TableExpr("audit.auth_events").Where("account_id = ?", account.ID).Exec(ctx)
	}()

	recentOnly, err := repo.ListPendingAccountWideWipes(ctx, time.Now().Add(-7*24*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, recentOnly)

	all, err := repo.ListPendingAccountWideWipes(ctx, time.Time{})
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, account.ID, all[0].AccountID)
	assert.Equal(t, "password_reset", all[0].Reason)
}

func TestAuthEventRepository_ClaimPendingAccountWideWipes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := NewAuthEventRepository(NewRuntime(db, auditTestTenantID))
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "claim_wipe@example.com")

	event := audit.NewAuthEvent(account.ID, audit.EventTypeTokenRevoked, true, "0.0.0.0")
	event.SetMetadata("reason", "account_deactivated")
	event.SetMetadata("pending_account_wide_wipe", true)
	require.NoError(t, repo.Create(ctx, event))

	claimed, err := repo.ClaimPendingAccountWideWipes(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, "account_deactivated", claimed[0].Reason)
	completed := audit.NewAuthEvent(account.ID, audit.EventTypeAccountWideWipeCompleted, true, "0.0.0.0")
	completed.SetMetadata("pending_event_id", claimed[0].EventID)
	require.NoError(t, repo.Create(ctx, completed))

	again, err := repo.ClaimPendingAccountWideWipes(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, again)
}
