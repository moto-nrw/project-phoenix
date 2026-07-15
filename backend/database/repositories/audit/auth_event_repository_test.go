package audit_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestAuthEventRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AuthEvent
	ctx := testpkg.TenantContext(1)

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

		testpkg.CleanupTableRecords(t, db, "audit.auth_events", event.ID)
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

		testpkg.CleanupTableRecords(t, db, "audit.auth_events", event.ID)
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

		testpkg.CleanupTableRecords(t, db, "audit.auth_events", event.ID)
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

		testpkg.CleanupTableRecords(t, db, "audit.auth_events", event.ID)
	})
}

func TestAuthEventRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AuthEvent
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "find_event@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds existing auth event", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.auth_events", event.ID)

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

	repo := repositories.NewFactory(db).AuthEvent
	ctx := testpkg.TenantContext(1)

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
		defer testpkg.CleanupTableRecords(t, db, "audit.auth_events", event1.ID, event2.ID, event3.ID)

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
		defer testpkg.CleanupTableRecords(t, db, "audit.auth_events", event1.ID, event2.ID, event3.ID)

		events, err := repo.FindByAccountID(ctx, account1.ID, 2)
		require.NoError(t, err)
		assert.Len(t, events, 2)
	})
}

func TestAuthEventRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).AuthEvent
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "list@example.com")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("lists all events", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.auth_events", event.ID)

		events, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, events)
	})

	t.Run("lists with filters", func(t *testing.T) {
		event := audit.NewAuthEvent(account.ID, audit.EventTypeLogout, true, "192.168.1.1")
		err := repo.Create(ctx, event)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "audit.auth_events", event.ID)

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
