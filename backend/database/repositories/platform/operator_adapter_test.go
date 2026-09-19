package platform_test

import (
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The retained operator audit-log contract is an adapter over the Audit
// owner's platform-scoped ledger (#2720). The operator and refresh-session
// contracts it used to sit next to are gone (#3252): the retained platform
// flows reach operator rows through the directory port the service root
// binds, covered by the root's operator directory tests.

func TestOperatorAuditLogRepositoryAdapter(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewOperatorAuditLogPersistence(db)
	ctx := testpkg.Ctx(t)
	operator := testpkg.CreateTestOperator(t, db)

	resourceID := int64(123)
	entry := &platformModels.OperatorAuditLog{
		OperatorID: operator.ID, Action: platformModels.ActionCreate, ResourceType: platformModels.ResourceAnnouncement,
		ResourceID: &resourceID, RequestIP: net.ParseIP("192.168.1.1"),
	}
	require.NoError(t, entry.SetChanges(map[string]any{"title": "New Announcement", "active": true}))
	require.NoError(t, repo.Create(ctx, entry))
	assert.NotZero(t, entry.ID)
	assert.NotZero(t, entry.CreatedAt)

	entries, err := repo.FindByOperatorID(ctx, operator.ID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, entry.ID, entries[0].ID)
	changes, err := entries[0].GetChanges()
	require.NoError(t, err)
	assert.Equal(t, "New Announcement", changes["title"])
	assert.Equal(t, "192.168.1.1", entries[0].RequestIP.String())

	ranged, err := repo.FindByDateRange(ctx, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), 0)
	require.NoError(t, err)
	var seen bool
	for _, candidate := range ranged {
		seen = seen || candidate.ID == entry.ID
	}
	assert.True(t, seen)
}
