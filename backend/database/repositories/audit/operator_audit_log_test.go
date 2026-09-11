package audit

import (
	"context"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operator ledger is platform-scoped: it appends without a tenant in the
// runtime and reads back the platform row.
func TestOperatorAuditLogRepository_AppendsWithoutTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, db)
	repo := NewOperatorAuditLogRepository(NewRuntime(db, func(context.Context) int64 { return 0 }))
	ctx := testpkg.Ctx(t)

	entry := &auditModels.OperatorAuditEntry{OperatorID: operator.ID, Action: "login", ResourceType: "operator", ResourceID: &operator.ID}
	require.NoError(t, repo.Create(ctx, entry))
	assert.NotZero(t, entry.ID)
	assert.NotZero(t, entry.CreatedAt)

	entries, err := repo.FindByOperatorID(ctx, operator.ID, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, entry.ID, entries[0].ID)

	require.Error(t, repo.Create(ctx, nil))
}
