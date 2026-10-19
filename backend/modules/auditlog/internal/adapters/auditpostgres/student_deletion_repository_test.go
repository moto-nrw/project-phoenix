package audit

import (
	"context"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentDeletionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewStudentDeletionRepository(NewRuntime(db, auditTestTenantID))

	assert.ErrorContains(t, repo.Create(context.Background(), nil), "audit event is required")

	actor := int64(42)
	event := &auditModels.StudentDeletion{StudentID: 99, ActorAccountID: &actor, Reason: "test_data"}
	ctx := testpkg.Ctx(t)
	expectedTenantID := auditModels.TenantIDFromContext(ctx)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, event))
	assert.Equal(t, expectedTenantID, event.TenantID)
	assert.Positive(t, event.ID)
}
