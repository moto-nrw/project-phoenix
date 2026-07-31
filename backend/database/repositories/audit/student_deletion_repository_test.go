package audit_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentDeletionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repo := repositories.NewFactory(db).StudentDeletionAudit

	assert.ErrorContains(t, repo.Create(context.Background(), nil), "audit event is required")

	event := &auditModels.StudentDeletion{StudentID: 99, ActorAccountID: 42, Reason: "test_data"}
	ctx := testpkg.TenantContext(1)
	expectedTenantID := tenant.FromContext(ctx)
	require.NoError(t, repo.Create(ctx, event))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "audit.student_deletions", event.ID)
	})
	assert.Equal(t, expectedTenantID, event.TenantID)
	assert.Positive(t, event.ID)
}
