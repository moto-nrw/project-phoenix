package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupSchemaTest(t *testing.T) (*bun.DB, enrollmentService.FormSchemaService, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, 1)
	repoFactory := repositories.NewFactory(db)
	svc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repoFactory.FormSchema,
		Logger: slog.Default(),
	})

	// Need a real auth.accounts row to satisfy the created_by FK.
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Form", "Editor")
	t.Cleanup(func() {
		// Clean up any schemas + the test account in the right order.
		_, _ = db.NewDelete().
			TableExpr("enrollment.form_schemas").
			Where("created_by = ?", account.ID).
			Exec(context.Background())
		_, _ = db.NewDelete().
			TableExpr("auth.accounts").
			Where("id = ?", account.ID).
			Exec(context.Background())
		_ = db.Close()
	})

	return db, svc, account.ID
}

func TestFormSchemaService_PublishVersion_CreatesActive(t *testing.T) {
	_, svc, creatorID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	fields := []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldTextarea, SortOrder: 0},
	}
	schema, err := svc.PublishVersion(ctx, fields, creatorID)
	require.NoError(t, err)
	assert.True(t, schema.IsActive)
	assert.Equal(t, 1, schema.Version)
	assert.Len(t, schema.Fields, 1)

	// GetActive should now return this schema.
	active, err := svc.GetActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, schema.ID, active.ID)
}

func TestFormSchemaService_PublishVersion_KeepsAllVersionsActive(t *testing.T) {
	// Multi-schema rework (commit 5e29a0dc8): publishing a new version
	// of the same logical schema no longer deactivates the previous
	// version. Phases pin schemas by id, so historical versions need to
	// stay valid until the row is hard-deleted. The (tenant_id, name,
	// version) unique index from migration 1.15.74 lets multiple
	// versions coexist with is_active=true.
	_, svc, creatorID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	v1, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)
	assert.True(t, v1.IsActive)

	v2, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "diet", Label: "Ernährung", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)
	assert.True(t, v2.IsActive)
	assert.Equal(t, 2, v2.Version)

	// v1 STAYS active (was deactivated under the old single-schema flow).
	v1Refetched, err := svc.GetByID(ctx, v1.ID)
	require.NoError(t, err)
	assert.True(t, v1Refetched.IsActive,
		"previous version must remain active under multi-schema semantics")
}

func TestFormSchemaService_GetActive_NoRowsErrSentinel(t *testing.T) {
	_, svc, _ := setupSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	_, err := svc.GetActive(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrNoActiveSchema),
		"GetActive on a tenant with no schema must return ErrNoActiveSchema")
}

func TestFormSchemaService_PublishVersion_RejectsCoreFieldKey(t *testing.T) {
	_, svc, creatorID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	_, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "guardian_email", Label: "Email", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.Error(t, err, "schema with a reserved core key must be rejected")
}

func TestFormSchemaService_PublishVersion_RejectsDuplicateKey(t *testing.T) {
	_, svc, creatorID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	_, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
		{Key: "allergies", Label: "Doppelt", Type: enrollmentModels.FormFieldText, SortOrder: 1},
	}, creatorID)
	require.Error(t, err, "duplicate keys must be rejected")
}

func TestFormSchemaService_ListVersions_ReturnsNewestFirst(t *testing.T) {
	_, svc, creatorID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	for i := 0; i < 3; i++ {
		_, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
			{Key: "field", Label: "Feld", Type: enrollmentModels.FormFieldText, SortOrder: 0},
		}, creatorID)
		require.NoError(t, err)
	}

	list, err := svc.ListVersions(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, 3, list[0].Version)
	assert.Equal(t, 2, list[1].Version)
	assert.Equal(t, 1, list[2].Version)
	// All three stay active — the multi-schema rework dropped the
	// "only one active" invariant. Phases pin by id, so older
	// versions need to remain valid for already-bound phases.
	assert.True(t, list[0].IsActive)
	assert.True(t, list[1].IsActive)
	assert.True(t, list[2].IsActive)
}
