package platform_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOperatorAuditLogRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorAuditLogRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "audit@example.com", "Audit Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	resourceID := int64(123)
	entry := &platformModels.OperatorAuditLog{
		OperatorID:   operator.ID,
		Action:       platformModels.ActionCreate,
		ResourceType: platformModels.ResourceAnnouncement,
		ResourceID:   &resourceID,
		RequestIP:    net.ParseIP("192.168.1.1"),
	}

	err := entry.SetChanges(map[string]any{
		"title":  "New Announcement",
		"active": true,
	})
	require.NoError(t, err)

	err = repo.Create(ctx, entry)
	require.NoError(t, err)
	assert.NotZero(t, entry.ID)
	assert.NotZero(t, entry.CreatedAt)

	// Cleanup
	defer cleanupTestAuditLog(t, db, entry.ID)

	// Verify changes were stored
	changes, err := entry.GetChanges()
	require.NoError(t, err)
	assert.Equal(t, "New Announcement", changes["title"])
	assert.Equal(t, true, changes["active"])
}

func TestOperatorAuditLogRepository_FindByOperatorID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorAuditLogRepository(db)
	ctx := context.Background()

	operator1 := createTestOperator(t, db, "op1@example.com", "Operator 1")
	defer cleanupTestOperator(t, db, operator1.ID)

	operator2 := createTestOperator(t, db, "op2@example.com", "Operator 2")
	defer cleanupTestOperator(t, db, operator2.ID)

	// Create audit logs for operator1
	entry1 := createTestAuditLog(t, db, operator1.ID, platformModels.ActionCreate, platformModels.ResourceAnnouncement)
	defer cleanupTestAuditLog(t, db, entry1.ID)

	entry2 := createTestAuditLog(t, db, operator1.ID, platformModels.ActionUpdate, platformModels.ResourceAnnouncement)
	defer cleanupTestAuditLog(t, db, entry2.ID)

	// Create audit log for operator2
	entry3 := createTestAuditLog(t, db, operator2.ID, platformModels.ActionDelete, platformModels.ResourceAnnouncement)
	defer cleanupTestAuditLog(t, db, entry3.ID)

	t.Run("FindAll", func(t *testing.T) {
		entries, err := repo.FindByOperatorID(ctx, operator1.ID, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2, "should find at least 2 entries")

		// Verify they belong to operator1
		for _, e := range entries {
			assert.Equal(t, operator1.ID, e.OperatorID)
		}
	})

	t.Run("WithLimit", func(t *testing.T) {
		entries, err := repo.FindByOperatorID(ctx, operator1.ID, 1)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(entries), 1, "should respect limit")
	})

	t.Run("OrderByCreatedAtDesc", func(t *testing.T) {
		entries, err := repo.FindByOperatorID(ctx, operator1.ID, 0)
		require.NoError(t, err)

		if len(entries) >= 2 {
			// Verify descending order
			for i := 0; i < len(entries)-1; i++ {
				assert.True(t,
					entries[i].CreatedAt.After(entries[i+1].CreatedAt) ||
						entries[i].CreatedAt.Equal(entries[i+1].CreatedAt),
					"entries should be ordered by created_at DESC")
			}
		}
	})
}

func TestOperatorAuditLogRepository_FindByResourceType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorAuditLogRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "audit@example.com", "Audit Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	// Create audit logs for different resource types
	announcementLog := createTestAuditLog(t, db, operator.ID, platformModels.ActionCreate, platformModels.ResourceAnnouncement)
	defer cleanupTestAuditLog(t, db, announcementLog.ID)

	suggestionLog := createTestAuditLog(t, db, operator.ID, platformModels.ActionCreate, platformModels.ResourceSuggestion)
	defer cleanupTestAuditLog(t, db, suggestionLog.ID)

	t.Run("FindAnnouncements", func(t *testing.T) {
		entries, err := repo.FindByResourceType(ctx, platformModels.ResourceAnnouncement, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, entries)

		// Verify all are announcement type
		for _, e := range entries {
			assert.Equal(t, platformModels.ResourceAnnouncement, e.ResourceType)
		}
	})

	t.Run("WithLimit", func(t *testing.T) {
		entries, err := repo.FindByResourceType(ctx, platformModels.ResourceAnnouncement, 1)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(entries), 1, "should respect limit")
	})
}

func TestOperatorAuditLogRepository_FindByDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platform.NewOperatorAuditLogRepository(db)
	ctx := context.Background()

	operator := createTestOperator(t, db, "audit@example.com", "Audit Operator")
	defer cleanupTestOperator(t, db, operator.ID)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	// Create audit log (will have current timestamp)
	entry := createTestAuditLog(t, db, operator.ID, platformModels.ActionCreate, platformModels.ResourceAnnouncement)
	defer cleanupTestAuditLog(t, db, entry.ID)

	t.Run("FindInRange", func(t *testing.T) {
		entries, err := repo.FindByDateRange(ctx, yesterday, tomorrow, 0)
		require.NoError(t, err)

		// Should contain our entry
		found := false
		for _, e := range entries {
			if e.ID == entry.ID {
				found = true
				assert.True(t, e.CreatedAt.After(yesterday) || e.CreatedAt.Equal(yesterday))
				assert.True(t, e.CreatedAt.Before(tomorrow) || e.CreatedAt.Equal(tomorrow))
			}
		}
		assert.True(t, found, "should find entry in date range")
	})

	t.Run("OutsideRange", func(t *testing.T) {
		farPast := now.Add(-48 * time.Hour)
		dayBeforeYesterday := now.Add(-36 * time.Hour)

		entries, err := repo.FindByDateRange(ctx, farPast, dayBeforeYesterday, 0)
		require.NoError(t, err)

		// Should not contain our recent entry
		for _, e := range entries {
			assert.NotEqual(t, entry.ID, e.ID, "should not find recent entry in past date range")
		}
	})

	t.Run("WithLimit", func(t *testing.T) {
		entries, err := repo.FindByDateRange(ctx, yesterday, tomorrow, 1)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(entries), 1, "should respect limit")
	})
}

// Helper functions

func createTestAuditLog(t *testing.T, db *bun.DB, operatorID int64, action, resourceType string) *platformModels.OperatorAuditLog {
	t.Helper()

	resourceID := int64(999)
	entry := &platformModels.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   &resourceID,
		RequestIP:    net.ParseIP("10.0.0.1"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewInsert().
		Model(entry).
		ModelTableExpr(`platform.operator_audit_log`).
		Exec(ctx)
	require.NoError(t, err)

	// Fetch the created entry to get the ID
	err = db.NewSelect().
		Model(entry).
		ModelTableExpr(`platform.operator_audit_log AS "log"`).
		Where(`"log".operator_id = ? AND "log".action = ? AND "log".resource_type = ?`,
			operatorID, action, resourceType).
		Order(`"log".created_at DESC`).
		Limit(1).
		Scan(ctx)
	require.NoError(t, err)

	return entry
}

func cleanupTestAuditLog(t *testing.T, db *bun.DB, logID int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		Model((*platformModels.OperatorAuditLog)(nil)).
		ModelTableExpr(`platform.operator_audit_log`).
		Where("id = ?", logID).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup audit log: %v", err)
	}
}
