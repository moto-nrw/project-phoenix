package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupFullSchemaTest layers PhaseRepo + RequestRepo onto the basic
// schema service so the DeleteSchema branches (which check for phase /
// request references) actually have something to look at.
func setupFullSchemaTest(t *testing.T) (*bun.DB, enrollmentService.FormSchemaService, int64, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, 1)
	repoFactory := repositories.NewFactory(db)
	svc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:        repoFactory.FormSchema,
		PhaseRepo:   repoFactory.Phase,
		RequestRepo: repoFactory.Request,
		Logger:      slog.Default(),
	})

	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Form", "Editor2")
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("enrollment.requests").
			Where("tenant_id = 1 AND guardian_email LIKE 'form-schema-extra-%@test'").
			Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("enrollment.phases").
			Where("tenant_id = 1 AND name LIKE 'form-schema-extra-%'").
			Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("enrollment.form_schemas").
			Where("created_by = ?", account.ID).
			Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("auth.accounts").
			Where("id = ?", account.ID).
			Exec(context.Background())
	})

	return db, svc, account.ID, repoFactory
}

func TestFormSchemaService_GetByID_RejectsNonPositive(t *testing.T) {
	_, svc, _, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	_, err := svc.GetByID(ctx, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive")

	_, err = svc.GetByID(ctx, -1)
	require.Error(t, err)
}

func TestFormSchemaService_GetByID_ReturnsRow(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	pub, err := svc.PublishVersion(ctx, []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	got, err := svc.GetByID(ctx, pub.ID)
	require.NoError(t, err)
	assert.Equal(t, pub.ID, got.ID)
	assert.Equal(t, pub.Name, got.Name)
	require.Len(t, got.Fields, 1)
	assert.Equal(t, "allergies", got.Fields[0].Key)
}

func TestFormSchemaService_CreateSchema_RequiresName(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	_, err := svc.CreateSchema(ctx, "", []enrollmentModels.FormField{
		{Key: "x", Label: "X", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestFormSchemaService_CreateSchema_FreshNameOK(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	name := uniqueSchemaName("Klassenanmeldung")
	out, err := svc.CreateSchema(ctx, name, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)
	assert.Equal(t, name, out.Name)
	assert.Equal(t, 1, out.Version, "fresh name starts at version 1")
}

func TestFormSchemaService_CreateSchema_RefusesExistingName(t *testing.T) {
	// CreateSchema is for *new* logical schemas. If the name already
	// has a v1 row, the admin should call UpdateSchema instead — the
	// service refuses to silently add a sibling.
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	name := uniqueSchemaName("Doppelname")
	_, err := svc.CreateSchema(ctx, name, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	_, err = svc.CreateSchema(ctx, name, []enrollmentModels.FormField{
		{Key: "field_b", Label: "Feld B", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestFormSchemaService_UpdateSchema_RejectsNonPositiveID(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	_, err := svc.UpdateSchema(ctx, 0, []enrollmentModels.FormField{
		{Key: "x", Label: "X", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestFormSchemaService_UpdateSchema_InheritsNameBumpsVersion(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	name := uniqueSchemaName("BumpTest")
	v1, err := svc.CreateSchema(ctx, name, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)
	require.Equal(t, 1, v1.Version)

	v2, err := svc.UpdateSchema(ctx, v1.ID, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A (überarbeitet)", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)
	assert.Equal(t, name, v2.Name, "new version inherits source name")
	assert.Equal(t, 2, v2.Version, "version is bumped by 1")
	assert.True(t, v2.IsActive)

	// v1 stays around — phases pinning it must keep working.
	v1ReFetched, err := svc.GetByID(ctx, v1.ID)
	require.NoError(t, err)
	assert.True(t, v1ReFetched.IsActive,
		"previous version stays active under multi-schema semantics")
}

func TestFormSchemaService_UpdateSchema_RejectsCoreFieldKey(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	name := uniqueSchemaName("CoreKeyGuard")
	v1, err := svc.CreateSchema(ctx, name, []enrollmentModels.FormField{
		{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	_, err = svc.UpdateSchema(ctx, v1.ID, []enrollmentModels.FormField{
		{Key: "guardian_email", Label: "Email", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.Error(t, err, "core field key must still be rejected on update")
}

func TestFormSchemaService_DeleteSchema_RejectsZero(t *testing.T) {
	_, svc, _, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	err := svc.DeleteSchema(ctx, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrFormSchemaNotFound))
}

func TestFormSchemaService_DeleteSchema_MissingIDReturnsNotFound(t *testing.T) {
	_, svc, _, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	err := svc.DeleteSchema(ctx, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrFormSchemaNotFound))
}

func TestFormSchemaService_DeleteSchema_HappyPathDropsAllVersions(t *testing.T) {
	db, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	name := uniqueSchemaName("DropMe")
	v1, err := svc.CreateSchema(ctx, name, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)
	v2, err := svc.UpdateSchema(ctx, v1.ID, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A (v2)", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteSchema(ctx, v2.ID))

	// Both versions gone.
	_, err = svc.GetByID(ctx, v1.ID)
	require.Error(t, err)
	_, err = svc.GetByID(ctx, v2.ID)
	require.Error(t, err)

	// Cross-check row count directly.
	count, err := db.NewSelect().
		TableExpr("enrollment.form_schemas").
		Where("name = ?", name).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count, "delete removes every row under the logical name")
}

func TestFormSchemaService_ValidateSubmission_MissingFieldErrors(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	schema, err := svc.CreateSchema(ctx, uniqueSchemaName("ValidateMissing"),
		[]enrollmentModels.FormField{
			{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0, Required: true},
		}, creatorID)
	require.NoError(t, err)

	err = svc.ValidateSubmission(ctx, schema.ID, enrollmentModels.SubmissionData{
		GuardianFields: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allergies")
	assert.Contains(t, err.Error(), "required")
}

func TestFormSchemaService_ValidateSubmission_EmptyStringTreatedAsMissing(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	schema, err := svc.CreateSchema(ctx, uniqueSchemaName("ValidateEmpty"),
		[]enrollmentModels.FormField{
			{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0, Required: true},
		}, creatorID)
	require.NoError(t, err)

	err = svc.ValidateSubmission(ctx, schema.ID, enrollmentModels.SubmissionData{
		GuardianFields: map[string]any{"allergies": ""},
	})
	require.Error(t, err, "empty string for required field must error")
}

func TestFormSchemaService_ValidateSubmission_PresentValueOK(t *testing.T) {
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	schema, err := svc.CreateSchema(ctx, uniqueSchemaName("ValidateOK"),
		[]enrollmentModels.FormField{
			{Key: "allergies", Label: "Allergien", Type: enrollmentModels.FormFieldText, SortOrder: 0, Required: true},
			{Key: "diet", Label: "Diät", Type: enrollmentModels.FormFieldText, SortOrder: 1, Required: false},
		}, creatorID)
	require.NoError(t, err)

	err = svc.ValidateSubmission(ctx, schema.ID, enrollmentModels.SubmissionData{
		GuardianFields: map[string]any{"allergies": "Nüsse"},
	})
	require.NoError(t, err)
}

func TestFormSchemaService_ValidateSubmission_SkipsPerChildFields(t *testing.T) {
	// PR 5 only validates guardian-level required fields. Per-child
	// required fields are deferred to PR 7's submit handler — verify
	// the service does not flag them as missing here.
	_, svc, creatorID, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	schema, err := svc.CreateSchema(ctx, uniqueSchemaName("ChildSkip"),
		[]enrollmentModels.FormField{
			{Key: "child_note", Label: "Hinweis", Type: enrollmentModels.FormFieldText, SortOrder: 0, Required: true, AppliesToCh: true},
		}, creatorID)
	require.NoError(t, err)

	err = svc.ValidateSubmission(ctx, schema.ID, enrollmentModels.SubmissionData{
		GuardianFields: map[string]any{},
	})
	require.NoError(t, err, "per-child required fields must be skipped at guardian-level validation")
}

func TestFormSchemaService_ValidateSubmission_MissingSchemaErrors(t *testing.T) {
	_, svc, _, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	err := svc.ValidateSubmission(ctx, 999_999_999, enrollmentModels.SubmissionData{
		GuardianFields: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema lookup")
}

// uniqueSchemaName builds a per-test schema name so parallel tests
// don't collide on the (tenant_id, name) uniqueness check baked into
// CreateSchema.
func uniqueSchemaName(prefix string) string {
	return strings.ReplaceAll(prefix, " ", "-") + "-" + time.Now().Format("150405.000000")
}
