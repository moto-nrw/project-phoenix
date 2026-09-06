package enrollment_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

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
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	svc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Owner:  repoFactory.Enrollment(),
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
	})

	return db, svc, account.ID, tenantID
}

func agbPDFBlock(documentURL string) capability.FormLegalBlock {
	return capability.FormLegalBlock{
		Key:         enrollmentModels.ConsentKeyAGB,
		Kind:        capability.LegalBlockKindTerms,
		Title:       "AGB / Teilnahmebedingungen",
		Label:       "Ich akzeptiere die AGB.",
		Required:    true,
		Enabled:     true,
		SortOrder:   10,
		Source:      capability.LegalBlockSourceStandard,
		DisplayMode: capability.LegalBlockDisplayModePDF,
		DocumentURL: documentURL,
	}
}

func TestFormSchemaService_CreateSchema_CreatesActive(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	fields := []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldTextarea, SortOrder: 0},
	}
	schema, err := svc.CreateSchema(ctx, "Standardformular", fields, creatorID)
	require.NoError(t, err)
	assert.True(t, schema.IsActive)
	assert.Equal(t, 1, schema.Version)
	assert.Len(t, schema.Fields, 1)

	// GetActive should now return this schema.
	active, err := svc.GetActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, schema.ID, active.ID)
}

func TestFormSchemaService_UpdateSchema_KeepsAllVersionsActive(t *testing.T) {
	t.Parallel()

	// Multi-schema rework (commit 5e29a0dc8): publishing a new version
	// of the same logical schema no longer deactivates the previous
	// version. Phases pin schemas by id, so historical versions need to
	// stay valid until the row is hard-deleted. The (tenant_id, name,
	// version) unique index from migration 1.15.74 lets multiple
	// versions coexist with is_active=true.
	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	v1, err := svc.CreateSchema(ctx, "Standardformular", []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)
	assert.True(t, v1.IsActive)

	v2, err := svc.UpdateSchema(ctx, v1.ID, []capability.FormField{
		{Key: "diet", Label: "Ernährung", Type: capability.FormFieldText, SortOrder: 0},
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
	t.Parallel()

	_, svc, _, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.GetActive(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrNoActiveSchema),
		"GetActive on a tenant with no schema must return ErrNoActiveSchema")
}

func TestFormSchemaService_CreateSchema_RejectsCoreFieldKey(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.CreateSchema(ctx, "Standardformular", []capability.FormField{
		{Key: "guardian_email", Label: "Email", Type: capability.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.Error(t, err, "schema with a reserved core key must be rejected")
}

func TestFormSchemaService_CreateSchema_RejectsDuplicateKey(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.CreateSchema(ctx, "Standardformular", []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		{Key: "allergies", Label: "Doppelt", Type: capability.FormFieldText, SortOrder: 1},
	}, creatorID)
	require.Error(t, err, "duplicate keys must be rejected")
}

func TestFormSchemaService_CreateSchemaWithLegal_NormalizesTenantDocumentURL(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)
	documentURL := fmt.Sprintf("/api/public/enrollment-form-legal-documents/%d_terms.pdf", tenantID)

	schema, err := svc.CreateSchemaWithLegal(
		ctx,
		"PDF-Rechtstext",
		[]capability.FormField{
			{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		},
		creatorID,
		capability.CoreRequirements{},
		[]capability.FormLegalBlock{agbPDFBlock(documentURL)},
	)

	require.NoError(t, err)
	require.Len(t, schema.LegalBlocks, 1)
	assert.Equal(t,
		fmt.Sprintf("/uploads/enrollment-form-legal-documents/%d_terms.pdf", tenantID),
		schema.LegalBlocks[0].DocumentURL,
	)
}

func TestFormSchemaService_CreateSchemaWithLegal_AcceptsTenantSettingsDocumentURL(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)
	documentURL := fmt.Sprintf("/api/public/enrollment-legal-documents/%d_terms.pdf", tenantID)

	schema, err := svc.CreateSchemaWithLegal(
		ctx,
		"PDF aus Einstellungen",
		[]capability.FormField{
			{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		},
		creatorID,
		capability.CoreRequirements{},
		[]capability.FormLegalBlock{agbPDFBlock(documentURL)},
	)

	require.NoError(t, err)
	require.Len(t, schema.LegalBlocks, 1)
	assert.Equal(t,
		fmt.Sprintf("/uploads/enrollment-legal-documents/%d_terms.pdf", tenantID),
		schema.LegalBlocks[0].DocumentURL,
	)
}

func TestFormSchemaService_CreateSchemaWithLegal_RejectsForeignDocumentURL(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)
	documentURL := fmt.Sprintf("/uploads/enrollment-form-legal-documents/%d_terms.pdf", tenantID+1)

	_, err := svc.CreateSchemaWithLegal(
		ctx,
		"Fremde PDF",
		[]capability.FormField{
			{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		},
		creatorID,
		capability.CoreRequirements{},
		[]capability.FormLegalBlock{agbPDFBlock(documentURL)},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PDF document URL")
}

func TestFormSchemaService_CreateSchemaWithLegal_RejectsForeignSettingsDocumentURL(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)
	documentURL := fmt.Sprintf("/uploads/enrollment-legal-documents/%d_terms.pdf", tenantID+1)

	_, err := svc.CreateSchemaWithLegal(
		ctx,
		"Fremde Settings-PDF",
		[]capability.FormField{
			{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		},
		creatorID,
		capability.CoreRequirements{},
		[]capability.FormLegalBlock{agbPDFBlock(documentURL)},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PDF document URL")
}

func TestFormSchemaService_CreateSchemaWithLegal_ClearsDocumentURLInTextMode(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)
	block := agbPDFBlock(fmt.Sprintf("/uploads/enrollment-form-legal-documents/%d_terms.pdf", tenantID))
	block.DisplayMode = capability.LegalBlockDisplayModeText
	block.Text = "AGB als Text"

	schema, err := svc.CreateSchemaWithLegal(
		ctx,
		"Text-Rechtstext",
		[]capability.FormField{
			{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		},
		creatorID,
		capability.CoreRequirements{},
		[]capability.FormLegalBlock{block},
	)

	require.NoError(t, err)
	require.Len(t, schema.LegalBlocks, 1)
	assert.Equal(t, capability.LegalBlockDisplayModeText, schema.LegalBlocks[0].DisplayMode)
	assert.Empty(t, schema.LegalBlocks[0].DocumentURL)
	assert.Equal(t, "AGB als Text", schema.LegalBlocks[0].Text)
}

func TestFormSchemaService_ListVersions_ReturnsNewestFirst(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	fields := []capability.FormField{
		{Key: "field", Label: "Feld", Type: capability.FormFieldText, SortOrder: 0},
	}
	latest, err := svc.CreateSchema(ctx, "Standardformular", fields, creatorID)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		latest, err = svc.UpdateSchema(ctx, latest.ID, fields, creatorID)
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
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	field := []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
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
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	field := []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
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
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	field := []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
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
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	created, err := svc.CreateSchema(ctx, "Schuljahr", []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}, creatorID)
	require.NoError(t, err)

	_, err = svc.RenameSchema(ctx, created.ID, "   ")
	require.Error(t, err, "a blank name must be rejected")
}

func TestFormSchemaService_RenameSchema_RejectsNonPositiveID(t *testing.T) {
	t.Parallel()

	_, svc, _, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	_, err := svc.RenameSchema(ctx, 0, "Egal")
	require.ErrorIs(t, err, enrollmentService.ErrFormSchemaNotFound,
		"a non-positive id can never identify a schema")
}

func TestFormSchemaService_RenameSchema_MissingSchemaReturnsNotFound(t *testing.T) {
	t.Parallel()

	_, svc, _, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	// No row carries this id, so FindByID returns sql.ErrNoRows wrapped in a
	// DatabaseError; the service must map that to the typed not-found sentinel.
	_, err := svc.RenameSchema(ctx, 999999999, "Egal")
	require.ErrorIs(t, err, enrollmentService.ErrFormSchemaNotFound)
}

// newSchemaServiceWithOwner injects an owner read failure without replacing storage.
func newSchemaServiceWithOwner(repo enrollmentService.FormSchemaOwner) enrollmentService.FormSchemaService {
	return enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Owner:  repo,
		Logger: slog.Default(),
	})
}

type activeSchemaReadFailure struct {
	enrollmentService.FormSchemaOwner
	err error
}

func (r activeSchemaReadFailure) ActiveSchema(context.Context) (*capability.FormSchema, error) {
	return nil, r.err
}

func TestFormSchemaService_ActiveReadFailureStopsPublish(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	unavailable := errors.New("schema storage unavailable")
	svc := newSchemaServiceWithOwner(activeSchemaReadFailure{err: unavailable})
	_, err := svc.GetActive(ctx)
	require.ErrorIs(t, err, unavailable)
	require.NotErrorIs(t, err, enrollmentService.ErrNoActiveSchema)
	_, err = svc.PublishForm(ctx, enrollmentService.PublishFormInput{})
	require.ErrorIs(t, err, unavailable, "a read failure must not enter the default-schema creation path")
}

// TestFormSchemaService_RenameAndPublishConcurrently_NeverSplitsLineage drives
// a rename and a version-publish at the SAME id concurrently, each in its own
// tenant transaction, many times. The per-lineage advisory lock must serialize
// them: whichever runs first, the other sees the result, so every version row
// always ends up under one shared name. Without the lock the publish can read
// the pre-rename name and insert a new version under it while the rename moves
// the existing rows — splitting the lineage across two names.
func TestFormSchemaService_RenameAndPublishConcurrently_NeverSplitsLineage(t *testing.T) {
	t.Parallel()

	db, svc, creatorID, tenantID := setupSchemaTest(t)

	field := []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}

	// Seed a single-version lineage. lineageID is any version's id — it
	// survives every publish (versions are never deleted) and every rename
	// (rename keeps row ids), so both operations can reference it forever.
	var lineageID int64
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		created, err := svc.CreateSchema(ctx, "Konzert", field, creatorID)
		if err != nil {
			return err
		}
		lineageID = created.ID
		return nil
	}))

	for i := 0; i < 15; i++ {
		target := fmt.Sprintf("Konzert-%d", i)
		var wg sync.WaitGroup
		wg.Add(2)
		var renameErr, publishErr error
		go func() {
			defer wg.Done()
			renameErr = testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
				_, err := svc.RenameSchema(ctx, lineageID, target)
				return err
			})
		}()
		go func() {
			defer wg.Done()
			publishErr = testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
				_, err := svc.UpdateSchema(ctx, lineageID, field, creatorID)
				return err
			})
		}()
		wg.Wait()
		require.NoError(t, renameErr, "iteration %d: rename", i)
		require.NoError(t, publishErr, "iteration %d: publish", i)

		names := make(map[string]struct{})
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			versions, err := svc.ListVersions(ctx)
			if err != nil {
				return err
			}
			for _, v := range versions {
				names[v.Name] = struct{}{}
			}
			return nil
		}))
		require.Len(t, names, 1, "iteration %d: lineage split across names %v", i, names)
	}
}

// TestFormSchemaService_RenameThenFailedPublish_RollsBackRename proves the
// combined "rename + edit" save is atomic at the database level: the rename
// and the version-publish ride in ONE tenant transaction owned by the public
// service method, so a publish failure rolls the rename
// back. Without the shared transaction the rename would commit on its own and
// leave a "renamed but content unchanged" lineage — the partial-save bug this
// guards against. The publish is made to fail deterministically with a
// reserved core-field key (same rejection as
// TestFormSchemaService_CreateSchema_RejectsCoreFieldKey).
func TestFormSchemaService_RenameThenFailedPublish_RollsBackRename(t *testing.T) {
	t.Parallel()

	db, svc, creatorID, tenantID := setupSchemaTest(t)

	field := []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}

	var lineageID int64
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		created, err := svc.CreateSchema(ctx, "Sommerfest", field, creatorID)
		if err != nil {
			return err
		}
		lineageID = created.ID
		return nil
	}))

	// No caller-supplied transaction: the public method must roll its own
	// rename back when the subsequent publication rejects a reserved core key.
	newName := "Herbstfest"
	ctx := testpkg.WithTenantRuntime(t, testpkg.TenantContext(tenantID), db)
	failed, txErr := svc.PublishFormVersion(ctx, enrollmentService.PublishFormVersionInput{
		ID: lineageID, Name: &newName, ActorID: creatorID,
		Fields: []capability.FormField{
			{Key: "guardian_email", Label: "Email", Type: capability.FormFieldText, SortOrder: 0},
		},
	})
	require.Error(t, txErr, "the reserved-key publish must fail and abort the transaction")
	require.ErrorContains(t, txErr, "guardian_email")
	require.Nil(t, failed)

	// Neither change committed: the name is still the original and no new
	// version row was inserted.
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		versions, err := svc.ListVersions(ctx)
		if err != nil {
			return err
		}
		require.Len(t, versions, 1, "a rolled-back publish must not leave a new version")
		assert.Equal(t, "Sommerfest", versions[0].Name,
			"the rename must roll back when the publish in the same transaction fails")
		return nil
	}))
	retried, err := svc.PublishFormVersion(ctx, enrollmentService.PublishFormVersionInput{
		ID: lineageID, Name: &newName, ActorID: creatorID, Fields: field,
	})
	require.NoError(t, err)
	require.Equal(t, 2, retried.Version, "rollback must not consume a version")
	require.Equal(t, newName, retried.Name)
	versions, err := svc.ListVersions(ctx)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	for _, version := range versions {
		require.Equal(t, newName, version.Name, "successful retry renames the entire lineage")
	}
}

// --- PublishForm / PublishFormVersion (POST + PUT /schema orchestration) ---

func publishFormFields() []capability.FormField {
	return []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}
}

func TestFormSchemaService_PublishForm_WithNameCreatesNamedSchema(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	schema, err := svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Name:    "Klassenanmeldung",
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Klassenanmeldung", schema.Name,
		"a non-empty name creates a new named schema")
	assert.Equal(t, 1, schema.Version)
	assert.True(t, schema.IsActive)
}

func TestFormSchemaService_PublishForm_NoNameNoActiveCreatesStandardformular(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	schema, err := svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Standardformular", schema.Name,
		"no name + no active schema falls back to the default Standardformular lineage")
	assert.Equal(t, 1, schema.Version)
}

func TestFormSchemaService_PublishForm_NoNameWithActiveUpdatesActive(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	first, err := svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Name:    "Standardformular",
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)
	require.Equal(t, 1, first.Version)

	second, err := svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Fields: []capability.FormField{
			{Key: "diet", Label: "Diät", Type: capability.FormFieldText, SortOrder: 0},
		},
		ActorID: creatorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Standardformular", second.Name,
		"no name + an active schema exists updates that schema's lineage")
	assert.Equal(t, 2, second.Version)
}

func TestFormSchemaService_PublishFormVersion_WithNameRenamesAndPublishes(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	first, err := svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Name:    "Ferienprogramm alt",
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)

	newName := "Ferienprogramm neu"
	version, err := svc.PublishFormVersion(ctx, enrollmentService.PublishFormVersionInput{
		ID:      first.ID,
		Name:    &newName,
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Ferienprogramm neu", version.Name,
		"a name in the PUT body renames the lineage AND publishes the new version")
	assert.Equal(t, 2, version.Version)

	versions, err := svc.ListVersions(ctx)
	require.NoError(t, err)
	for _, v := range versions {
		assert.Equal(t, "Ferienprogramm neu", v.Name,
			"the whole lineage shares the renamed name")
	}
}

func TestFormSchemaService_PublishFormVersion_BlankNameSkipsRename(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	first, err := svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Name:    "Klassenanmeldung",
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)

	blank := "   "
	version, err := svc.PublishFormVersion(ctx, enrollmentService.PublishFormVersionInput{
		ID:      first.ID,
		Name:    &blank,
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Klassenanmeldung", version.Name,
		"a blank name must not rename the lineage")
	assert.Equal(t, 2, version.Version, "the publish still runs")
}

func TestFormSchemaService_PublishFormVersion_RenameCollisionWrapsError(t *testing.T) {
	t.Parallel()

	_, svc, creatorID, tenantID := setupSchemaTest(t)
	ctx := testpkg.TenantContext(tenantID)

	target, err := svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Name:    "Anmeldung A",
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)
	_, err = svc.PublishForm(ctx, enrollmentService.PublishFormInput{
		Name:    "Anmeldung B",
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.NoError(t, err)

	collidingName := "Anmeldung B"
	_, err = svc.PublishFormVersion(ctx, enrollmentService.PublishFormVersionInput{
		ID:      target.ID,
		Name:    &collidingName,
		Fields:  publishFormFields(),
		ActorID: creatorID,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, enrollmentService.ErrFormSchemaNameExists,
		"a rename onto an existing lineage name surfaces the collision sentinel")
	var renameErr enrollmentService.RenameStepError
	assert.ErrorAs(t, err, &renameErr,
		"a rename-step failure is wrapped so the handler can map it distinctly")

	versions, err := svc.ListVersions(ctx)
	require.NoError(t, err)
	aVersions := 0
	for _, v := range versions {
		if v.Name == "Anmeldung A" {
			aVersions++
		}
	}
	assert.Equal(t, 1, aVersions, "a failed rename must abort before the publish")
}
