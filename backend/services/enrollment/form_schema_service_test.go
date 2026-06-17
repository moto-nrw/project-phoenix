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

func setupSchemaTest(t *testing.T) (*bun.DB, enrollmentService.FormSchemaService, int64, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
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

	return db, svc, account.ID, tenantID
}

func TestFormSchemaService_PublishVersion_CreatesActive(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

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
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

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
	_, svc, _, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.GetActive(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrNoActiveSchema),
		"GetActive on a tenant with no schema must return ErrNoActiveSchema")
}

func TestFormSchemaService_PublishVersion_RejectsCoreFieldKey(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "guardian_email", Label: "Email", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.Error(t, err, "schema with a reserved core key must be rejected")
}

func TestFormSchemaService_PublishVersion_RejectsDuplicateKey(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
		{Key: "allergies", Label: "Doppelt", Type: enrollmentModels.FormFieldText, SortOrder: 1},
	}, creatorID)
	require.Error(t, err, "duplicate keys must be rejected")
}

func TestFormSchemaService_ListVersions_ReturnsNewestFirst(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

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

func TestFormSchemaService_RenameSchema_RenamesWholeLineage(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	field := []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}
	created, err := svc.CreateSchema(ctx, "Ferienbetreuung", field, creatorID)
	require.NoError(t, err)
	// Add a second version so we can assert the rename hits every row.
	v2, err := svc.UpdateSchema(ctx, created.ID, field, creatorID)
	require.NoError(t, err)
	require.Equal(t, 2, v2.Version)

	renamed, err := svc.RenameSchema(ctx, created.ID, "  Ferienprogramm  ")
	require.NoError(t, err)
	assert.Equal(t, "Ferienprogramm", renamed.Name, "service must trim the name")

	list, err := svc.ListVersions(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	for _, s := range list {
		assert.Equal(t, "Ferienprogramm", s.Name,
			"every version of the lineage must carry the new name")
	}
}

func TestFormSchemaService_RenameSchema_RejectsCollisionWithOtherSchema(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	field := []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}
	a, err := svc.CreateSchema(ctx, "Schuljahr", field, creatorID)
	require.NoError(t, err)
	_, err = svc.CreateSchema(ctx, "Ferien", field, creatorID)
	require.NoError(t, err)

	_, err = svc.RenameSchema(ctx, a.ID, "Ferien")
	require.ErrorIs(t, err, enrollmentService.ErrFormSchemaNameExists,
		"renaming onto an existing schema name must be refused, not split the lineage")
}

func TestFormSchemaService_RenameSchema_SameNameIsNoOp(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	field := []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}
	created, err := svc.CreateSchema(ctx, "Schuljahr", field, creatorID)
	require.NoError(t, err)

	// Renaming to the unchanged name must not trip the collision check
	// against the schema's own rows.
	renamed, err := svc.RenameSchema(ctx, created.ID, "Schuljahr")
	require.NoError(t, err)
	assert.Equal(t, "Schuljahr", renamed.Name)
}

func TestFormSchemaService_RenameSchema_RejectsEmptyName(t *testing.T) {
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	created, err := svc.CreateSchema(ctx, "Schuljahr", []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	_, err = svc.RenameSchema(ctx, created.ID, "   ")
	require.Error(t, err, "a blank name must be rejected")
}

func TestFormSchemaService_RenameSchema_RejectsNonPositiveID(t *testing.T) {
	_, svc, _, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.RenameSchema(ctx, 0, "Egal")
	require.ErrorIs(t, err, enrollmentService.ErrFormSchemaNotFound,
		"a non-positive id can never identify a schema")
}

func TestFormSchemaService_RenameSchema_MissingSchemaReturnsNotFound(t *testing.T) {
	_, svc, _, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	// No row carries this id, so FindByID returns sql.ErrNoRows wrapped in a
	// DatabaseError; the service must map that to the typed not-found sentinel.
	_, err := svc.RenameSchema(ctx, 999999999, "Egal")
	require.ErrorIs(t, err, enrollmentService.ErrFormSchemaNotFound)
}

// newSchemaServiceWithRepo builds a service around an arbitrary repo so a
// test can inject failures into a single repository method. Mirrors the
// minimal config setupSchemaTest uses (RenameSchema only touches s.repo).
func newSchemaServiceWithRepo(repo enrollmentModels.FormSchemaRepository) enrollmentService.FormSchemaService {
	return enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   repo,
		Logger: slog.Default(),
	})
}

// existsCheckFailsRepo delegates to a real repo (so FindByID returns the
// source row) but fails the name-existence probe, exercising RenameSchema's
// "check existing name" error branch.
type existsCheckFailsRepo struct {
	enrollmentModels.FormSchemaRepository
}

func (existsCheckFailsRepo) ExistsByName(context.Context, string) (bool, error) {
	return false, errors.New("exists probe boom")
}

// renameExecFailsRepo reports no collision but fails the actual rename with a
// plain (non-23505) error, exercising the generic rename error branch. The
// 23505 race fallback needs a real concurrent unique-violation and is left to
// the repository-level tests.
type renameExecFailsRepo struct {
	enrollmentModels.FormSchemaRepository
}

func (renameExecFailsRepo) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}

func (renameExecFailsRepo) RenameByName(context.Context, string, string) error {
	return errors.New("rename exec boom")
}

func TestFormSchemaService_RenameSchema_LoadErrorIsServerFault(t *testing.T) {
	_, _, _, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	// failingSchemaRepo (decision_export_integration_test.go) makes FindByID
	// fail with a non-ErrNoRows error — a transient read fault, not a 404.
	svc := newSchemaServiceWithRepo(failingSchemaRepo{})
	_, err := svc.RenameSchema(ctx, 42, "Egal")
	require.Error(t, err)
	require.NotErrorIs(t, err, enrollmentService.ErrFormSchemaNotFound,
		"a read fault must not be flattened into not-found")
	assert.Contains(t, err.Error(), "load source schema")
}

func TestFormSchemaService_RenameSchema_ExistsCheckErrorPropagates(t *testing.T) {
	db, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	created, err := svc.CreateSchema(ctx, "Schuljahr", []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	wrapped := newSchemaServiceWithRepo(existsCheckFailsRepo{repositories.NewFactory(db).FormSchema})
	_, err = wrapped.RenameSchema(ctx, created.ID, "Ferien")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check existing name")
}

func TestFormSchemaService_RenameSchema_RenameExecErrorPropagates(t *testing.T) {
	db, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	created, err := svc.CreateSchema(ctx, "Schuljahr", []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	wrapped := newSchemaServiceWithRepo(renameExecFailsRepo{repositories.NewFactory(db).FormSchema})
	_, err = wrapped.RenameSchema(ctx, created.ID, "Ferien")
	require.Error(t, err)
	require.NotErrorIs(t, err, enrollmentService.ErrFormSchemaNameExists,
		"a plain rename failure is not a name collision")
	assert.Contains(t, err.Error(), "rename schema")
}
