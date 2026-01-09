package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupAuthEventRepo(_ *testing.T, db *bun.DB) *repositories.Factory {
	return repositories.NewFactory(db)
}

// cleanupAuthEventRecords removes auth event records directly
func cleanupAuthEventRecords(t *testing.T, db *bun.DB, eventIDs ...int64) {
	t.Helper()
	if len(eventIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("audit.auth_events").
		Where("id IN (?)", bun.In(eventIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup auth events: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestAuthEventRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	// Create a test account
	account := testpkg.CreateTestAccount(t, db, "auth_event_test@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

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

		cleanupAuthEventRecords(t, db, event.ID)
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

		cleanupAuthEventRecords(t, db, event.ID)
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

		cleanupAuthEventRecords(t, db, event.ID)
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

		cleanupAuthEventRecords(t, db, event.ID)
	})
}

func TestAuthEventRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "find_event@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds existing auth event", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)
		defer cleanupAuthEventRecords(t, db, event.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	account1 := testpkg.CreateTestAccount(t, db, "account1@example.com")
	account2 := testpkg.CreateTestAccount(t, db, "account2@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account1.ID, account2.ID)

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
		defer cleanupAuthEventRecords(t, db, event1.ID, event2.ID, event3.ID)

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
		defer cleanupAuthEventRecords(t, db, event1.ID, event2.ID, event3.ID)

		events, err := repo.FindByAccountID(ctx, account1.ID, 2)
		require.NoError(t, err)
		assert.Len(t, events, 2)
	})
}

func TestAuthEventRepository_FindByEventType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "event_type@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds events by type", func(t *testing.T) {
		hourAgo := time.Now().Add(-time.Hour)

		loginEvent := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		logoutEvent := audit.NewAuthEvent(account.ID, audit.EventTypeLogout, true, "192.168.1.1")

		err := repo.Create(ctx, loginEvent)
		require.NoError(t, err)
		err = repo.Create(ctx, logoutEvent)
		require.NoError(t, err)
		defer cleanupAuthEventRecords(t, db, loginEvent.ID, logoutEvent.ID)

		events, err := repo.FindByEventType(ctx, audit.EventTypeLogin, hourAgo)
		require.NoError(t, err)

		for _, e := range events {
			assert.Equal(t, audit.EventTypeLogin, e.EventType)
		}

		var found bool
		for _, e := range events {
			if e.ID == loginEvent.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestAuthEventRepository_FindFailedAttempts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "failed_attempts@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds failed login attempts", func(t *testing.T) {
		hourAgo := time.Now().Add(-time.Hour)

		successEvent := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		failedEvent1 := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, false, "192.168.1.1")
		failedEvent1.ErrorMessage = "Invalid password"
		failedEvent2 := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, false, "192.168.1.2")
		failedEvent2.ErrorMessage = "Invalid password"

		err := repo.Create(ctx, successEvent)
		require.NoError(t, err)
		err = repo.Create(ctx, failedEvent1)
		require.NoError(t, err)
		err = repo.Create(ctx, failedEvent2)
		require.NoError(t, err)
		defer cleanupAuthEventRecords(t, db, successEvent.ID, failedEvent1.ID, failedEvent2.ID)

		events, err := repo.FindFailedAttempts(ctx, account.ID, hourAgo)
		require.NoError(t, err)
		assert.Len(t, events, 2)

		for _, e := range events {
			assert.False(t, e.Success)
			assert.Equal(t, account.ID, e.AccountID)
		}
	})
}

func TestAuthEventRepository_CountFailedAttempts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "count_failed@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("counts failed attempts", func(t *testing.T) {
		hourAgo := time.Now().Add(-time.Hour)

		failedEvent1 := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, false, "192.168.1.1")
		failedEvent2 := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, false, "192.168.1.2")
		failedEvent3 := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, false, "192.168.1.3")

		err := repo.Create(ctx, failedEvent1)
		require.NoError(t, err)
		err = repo.Create(ctx, failedEvent2)
		require.NoError(t, err)
		err = repo.Create(ctx, failedEvent3)
		require.NoError(t, err)
		defer cleanupAuthEventRecords(t, db, failedEvent1.ID, failedEvent2.ID, failedEvent3.ID)

		count, err := repo.CountFailedAttempts(ctx, account.ID, hourAgo)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})
}

func TestAuthEventRepository_CleanupOldEvents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "cleanup@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("cleans up old events", func(t *testing.T) {
		// Create a recent event (won't be cleaned up)
		recentEvent := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		err := repo.Create(ctx, recentEvent)
		require.NoError(t, err)
		defer cleanupAuthEventRecords(t, db, recentEvent.ID)

		// Clean up events older than 1 hour (recent event should remain)
		deleted, err := repo.CleanupOldEvents(ctx, time.Hour)
		require.NoError(t, err)
		// We can't guarantee any deletions, but at least verify no error
		assert.GreaterOrEqual(t, deleted, 0)

		// Verify recent event still exists
		found, err := repo.FindByID(ctx, recentEvent.ID)
		require.NoError(t, err)
		assert.Equal(t, recentEvent.ID, found.ID)
	})
}

func TestAuthEventRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repoFactory := setupAuthEventRepo(t, db)
	repo := repoFactory.AuthEvent
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "list@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("lists all events", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)
		defer cleanupAuthEventRecords(t, db, event.ID)

		events, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, events)
	})

	t.Run("lists with filters", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogout, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)
		defer cleanupAuthEventRecords(t, db, event.ID)

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
