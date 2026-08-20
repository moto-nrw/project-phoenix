package platform_test

import (
	"net"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorAuditLogRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := platform.NewOperatorAuditLogRepository(db)
	ctx := testpkg.Ctx(t)

	operator := createTestOperator(t, db, "audit@example.com", "Audit Operator")

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

	// Verify changes were stored
	changes, err := entry.GetChanges()
	require.NoError(t, err)
	assert.Equal(t, "New Announcement", changes["title"])
	assert.Equal(t, true, changes["active"])
}
