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
