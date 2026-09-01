package audit

import (
	"context"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentDeletionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewStudentDeletionRepository(NewRuntime(db, auditTestTenantID))

	assert.ErrorContains(t, repo.Create(context.Background(), nil), "audit event is required")

	event := &auditModels.StudentDeletion{StudentID: 99, ActorAccountID: 42, Reason: "test_data"}
	ctx := testpkg.Ctx(t)
	expectedTenantID := auditModels.TenantIDFromContext(ctx)
	require.NoError(t, repo.Create(ctx, event))
	assert.Equal(t, expectedTenantID, event.TenantID)
	assert.Positive(t, event.ID)
}
