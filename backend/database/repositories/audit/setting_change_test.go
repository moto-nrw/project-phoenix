package audit_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	repoAudit "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// createTestChange creates a valid setting change for testing
func createTestChange(key string, scopeType string, scopeID *int64, changeType string) *audit.SettingChange {
	oldValue, _ := json.Marshal(map[string]any{"value": false})
	newValue, _ := json.Marshal(map[string]any{"value": true})
	return &audit.SettingChange{
		SettingKey: key,
		ScopeType:  scopeType,
		ScopeID:    scopeID,
		ChangeType: changeType,
		OldValue:   oldValue,
		NewValue:   newValue,
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		Reason:     "test change",
	}
}

// cleanupChange removes a test change from the database
func cleanupChange(tb testing.TB, db *bun.DB, id int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = db.NewDelete().
		TableExpr("audit.setting_changes").
		Where("id = ?", id).
		Exec(ctx)
}

func TestSettingChangeRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("creates change successfully", func(t *testing.T) {
		change := createTestChange("test.change.create."+t.Name(), "system", nil, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		assert.NotZero(t, change.ID)
	})

	t.Run("creates change with og scope", func(t *testing.T) {
		ogID := int64(123)
		change := createTestChange("test.change.create.og."+t.Name(), "og", &ogID, "update")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		assert.NotZero(t, change.ID)
	})

	t.Run("fails with invalid change", func(t *testing.T) {
		change := &audit.SettingChange{
			// Missing required fields
		}
		err := repo.Create(ctx, change)
		require.Error(t, err)
	})
}

func TestSettingChangeRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("finds existing change", func(t *testing.T) {
		change := createTestChange("test.change.findbyid."+t.Name(), "system", nil, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		found, err := repo.FindByID(ctx, change.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, change.ID, found.ID)
		assert.Equal(t, change.SettingKey, found.SettingKey)
	})

	t.Run("returns nil for non-existent ID", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSettingChangeRepository_FindByScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("finds changes by system scope", func(t *testing.T) {
		change1 := createTestChange("test.change.scope1."+t.Name(), "system", nil, "create")
		err := repo.Create(ctx, change1)
		require.NoError(t, err)
		defer cleanupChange(t, db, change1.ID)

		change2 := createTestChange("test.change.scope2."+t.Name(), "system", nil, "update")
		err = repo.Create(ctx, change2)
		require.NoError(t, err)
		defer cleanupChange(t, db, change2.ID)

		found, err := repo.FindByScope(ctx, "system", nil, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 2)
	})

	t.Run("finds changes by og scope", func(t *testing.T) {
		ogID := int64(456)
		change := createTestChange("test.change.scope.og."+t.Name(), "og", &ogID, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		found, err := repo.FindByScope(ctx, "og", &ogID, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		change1 := createTestChange("test.change.limit1."+t.Name(), "system", nil, "create")
		err := repo.Create(ctx, change1)
		require.NoError(t, err)
		defer cleanupChange(t, db, change1.ID)

		change2 := createTestChange("test.change.limit2."+t.Name(), "system", nil, "update")
		err = repo.Create(ctx, change2)
		require.NoError(t, err)
		defer cleanupChange(t, db, change2.ID)

		found, err := repo.FindByScope(ctx, "system", nil, 1)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(found), 1)
	})
}

func TestSettingChangeRepository_FindByKey(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("finds changes by key", func(t *testing.T) {
		uniqueKey := "test.change.findbykey." + t.Name()

		change1 := createTestChange(uniqueKey, "system", nil, "create")
		err := repo.Create(ctx, change1)
		require.NoError(t, err)
		defer cleanupChange(t, db, change1.ID)

		change2 := createTestChange(uniqueKey, "system", nil, "update")
		err = repo.Create(ctx, change2)
		require.NoError(t, err)
		defer cleanupChange(t, db, change2.ID)

		found, err := repo.FindByKey(ctx, uniqueKey, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 2)
	})
}

func TestSettingChangeRepository_FindByKeyAndScope(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("finds changes by key and scope", func(t *testing.T) {
		uniqueKey := "test.change.keyandscope." + t.Name()
		ogID := int64(789)

		change := createTestChange(uniqueKey, "og", &ogID, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		found, err := repo.FindByKeyAndScope(ctx, uniqueKey, "og", &ogID, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})

	t.Run("returns empty for non-matching scope", func(t *testing.T) {
		uniqueKey := "test.change.keyandscope.nomatch." + t.Name()
		ogID := int64(999)

		change := createTestChange(uniqueKey, "og", &ogID, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		differentOgID := int64(888)
		found, err := repo.FindByKeyAndScope(ctx, uniqueKey, "og", &differentOgID, 10)
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

func TestSettingChangeRepository_FindByAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("finds changes by account", func(t *testing.T) {
		// Create account for testing
		account := testpkg.CreateTestAccount(t, db, "changetest")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		change := createTestChange("test.change.account."+t.Name(), "system", nil, "create")
		change.AccountID = &account.ID
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		found, err := repo.FindByAccount(ctx, account.ID, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})
}

func TestSettingChangeRepository_FindRecent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("finds recent changes", func(t *testing.T) {
		change := createTestChange("test.change.recent."+t.Name(), "system", nil, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		// Find changes from last hour
		since := time.Now().Add(-1 * time.Hour)
		found, err := repo.FindRecent(ctx, since, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})
}

func TestSettingChangeRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("lists with filters", func(t *testing.T) {
		uniqueKey := "test.change.list." + t.Name()

		change := createTestChange(uniqueKey, "system", nil, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		filters := map[string]interface{}{
			"setting_key": uniqueKey,
			"limit":       10,
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})

	t.Run("lists with change_type filter", func(t *testing.T) {
		change := createTestChange("test.change.list.type."+t.Name(), "system", nil, "delete")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		filters := map[string]interface{}{
			"change_type": "delete",
			"limit":       100,
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})

	t.Run("lists with time range filters", func(t *testing.T) {
		change := createTestChange("test.change.list.time."+t.Name(), "system", nil, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)
		defer cleanupChange(t, db, change.ID)

		filters := map[string]interface{}{
			"since": time.Now().Add(-1 * time.Hour),
			"until": time.Now().Add(1 * time.Hour),
			"limit": 100,
		}
		found, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(found), 1)
	})
}

func TestSettingChangeRepository_CleanupOldChanges(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoAudit.NewSettingChangeRepository(db)
	ctx := context.Background()

	t.Run("cleans up old changes", func(t *testing.T) {
		// Create a change
		change := createTestChange("test.change.cleanup."+t.Name(), "system", nil, "create")
		err := repo.Create(ctx, change)
		require.NoError(t, err)

		// Manually update created_at to be old
		_, err = db.NewUpdate().
			TableExpr("audit.setting_changes").
			Set("created_at = ?", time.Now().Add(-30*24*time.Hour)).
			Where("id = ?", change.ID).
			Exec(ctx)
		require.NoError(t, err)

		// Cleanup changes older than 7 days
		deletedCount, err := repo.CleanupOldChanges(ctx, 7*24*time.Hour)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deletedCount, 1)

		// Verify deleted
		found, err := repo.FindByID(ctx, change.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}
