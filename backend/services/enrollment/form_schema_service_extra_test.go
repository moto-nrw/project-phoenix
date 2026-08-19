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

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	pub, err := svc.CreateSchema(ctx, "Testformular GetByID", []enrollmentModels.FormField{
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

func TestFormSchemaService_UpdateSchema_RepointsPhasesButKeepsRequestSchemaPin(t *testing.T) {
	_, svc, creatorID, repoFactory := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	name := uniqueSchemaName("RepointTest")
	v1, err := svc.CreateSchema(ctx, name, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A", Type: enrollmentModels.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	phaseName := uniqueSchemaName("form-schema-extra-repoint")
	phase := &enrollmentModels.Phase{
		Name:              phaseName,
		Kind:              enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:  timezone.NewDate(2026, 9, 1),
		ServiceEndDate:    timezone.NewDate(2027, 7, 31),
		FormSchemaID:      &v1.ID,
		IsActive:          true,
		CareOverflowMode:  enrollmentModels.PhaseCareOverflowWaitlist,
		EnrollmentOpenAt:  nil,
		EnrollmentCloseAt: nil,
	}
	require.NoError(t, repoFactory.Phase.Create(ctx, phase))

	request := &enrollmentModels.Request{
		SchemaID:          &v1.ID,
		PhaseID:           phase.ID,
		GuardianFirstName: "Form",
		GuardianLastName:  "Schema",
		GuardianEmail:     "form-schema-extra-repoint@test",
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		StatusToken:       "form-schema-extra-repoint-" + time.Now().Format("150405.000000"),
		SubmittedAt:       time.Now().UTC(),
	}
	require.NoError(t, repoFactory.Request.Create(ctx, request))

	v2, err := svc.UpdateSchema(ctx, v1.ID, []enrollmentModels.FormField{
		{Key: "field_a", Label: "Feld A (v2)", Type: enrollmentModels.FormFieldText, SortOrder: 0, Required: true},
	}, creatorID)
	require.NoError(t, err)
	require.NotEqual(t, v1.ID, v2.ID)

	gotPhase, err := repoFactory.Phase.FindByID(ctx, phase.ID)
	require.NoError(t, err)
	require.NotNil(t, gotPhase.FormSchemaID)
	assert.Equal(t, v2.ID, *gotPhase.FormSchemaID,
		"phase must follow the newly published version of the same logical schema")

	gotRequest, err := repoFactory.Request.FindByID(ctx, request.ID)
	require.NoError(t, err)
	require.NotNil(t, gotRequest.SchemaID)
	assert.Equal(t, v1.ID, *gotRequest.SchemaID,
		"already-submitted requests must keep their original schema_id pin")
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

func TestFormSchemaService_GetByID_MissingIDReturnsNotFound(t *testing.T) {
	_, svc, _, _ := setupFullSchemaTest(t)
	ctx := testpkg.TenantContext(1)

	_, err := svc.GetByID(ctx, 999_999_999)
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

// uniqueSchemaName builds a per-test schema name so parallel tests
// don't collide on the (tenant_id, name) uniqueness check baked into
// CreateSchema.
func uniqueSchemaName(prefix string) string {
	return strings.ReplaceAll(prefix, " ", "-") + "-" + time.Now().Format("150405.000000")
}
