package enrollment_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// uniqueSchemaName builds a per-test schema name so concurrent tests
// don't collide on (tenant_id, name, version).
func uniqueSchemaName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// setupSchemaRepoTest spins up a tenant + a creator account so the
// form_schemas FK (created_by → auth.accounts) is satisfied. Returns
// the DB, the repo under test, the tenant id, and the creator id.
// Cleans up the creator account on test end; schema rows are wiped
// per-test by the caller via defer.
func setupSchemaRepoTest(t *testing.T) (*bun.DB, *capability.Module, int64, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "schemarepo")
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").
			Where("id = ?", account.ID).
			Exec(context.Background())
	})

	repo := enrollmentCompose.New()
	return db, repo, tenantID, account.ID
}

// runInTenantTx wraps a repo call in a tenant transaction so RLS +
// tenant_id assignment fire the same way as production. Returns the
// error from the closure unchanged.
func runInTenantTx(t *testing.T, db *bun.DB, tenantID int64, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithTenantTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}

// wipeSchemas removes every schema row created by the given account in
// the given tenant. Deferred from each test so the next test starts
// clean. Best-effort; tests don't fail on cleanup errors.
func wipeSchemas(db *bun.DB, tenantID, createdBy int64) {
	_, _ = db.NewDelete().
		TableExpr("enrollment.form_schemas").
		Where("tenant_id = ? AND created_by = ?", tenantID, createdBy).
		Exec(context.Background())
}

func validFields() []capability.FormField {
	return []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}
}

// --- Create + FindByID --------------------------------------------------

func TestFormSchemaRepository_Create_PersistsAndReturnsID(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	schema := &capability.FormSchema{
		Name:      uniqueSchemaName("create"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
	}
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, schema)
	})
	require.NoError(t, err)
	assert.NotZero(t, schema.ID, "RETURNING * must populate ID")
	assert.Equal(t, tenantID, schema.TenantID, "EnsureTenantID must stamp tenant from ctx")
	assert.False(t, schema.CreatedAt.IsZero(), "default current_timestamp must populate created_at")
}

func TestFormSchemaRepository_Create_RejectsInvalidSchema(t *testing.T) {
	t.Parallel()

	// Validate() runs before INSERT — missing name must surface as a
	// wrapped validation error, not a DB constraint error.
	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	schema := &capability.FormSchema{
		Version:   1,
		CreatedBy: creator,
		// Name intentionally empty.
	}
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, schema)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestFormSchemaRepository_FindByID_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	schema := &capability.FormSchema{
		Name:      uniqueSchemaName("find"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
	}
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, schema)
	})
	require.NoError(t, err)

	var got *capability.FormSchema
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Schema(ctx, schema.ID)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, schema.ID, got.ID)
	assert.Equal(t, schema.Name, got.Name)
	require.Len(t, got.Fields, 1)
	assert.Equal(t, "allergies", got.Fields[0].Key)
}

func TestFormSchemaRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _ := setupSchemaRepoTest(t)

	var got *capability.FormSchema
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Schema(ctx, 9_999_999)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "not found")
	assert.True(t, errors.Is(err, sql.ErrNoRows),
		"FindByID not-found must preserve sql.ErrNoRows for stale schema fallbacks")
}

// --- FindActive ---------------------------------------------------------

func TestFormSchemaRepository_FindActive_ReturnsActiveRow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	active := &capability.FormSchema{
		Name:      uniqueSchemaName("findactive"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, active)
	}))

	var got *capability.FormSchema
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.ActiveSchema(ctx)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, active.ID, got.ID)
}

func TestFormSchemaRepository_FindActive_NoRowsReturnsWrappedErrNoRows(t *testing.T) {
	t.Parallel()

	// Service layer relies on errors.Is(err, sql.ErrNoRows) to map to
	// its own ErrNoActiveSchema sentinel. The wrap must preserve that.
	db, repo, tenantID, _ := setupSchemaRepoTest(t)

	var got *capability.FormSchema
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.ActiveSchema(ctx)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, sql.ErrNoRows),
		"FindActive must wrap sql.ErrNoRows so the service can translate it")
}

// --- ListByTenant -------------------------------------------------------

func TestFormSchemaRepository_ListByTenant_OrdersByVersionDesc(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	name := uniqueSchemaName("list")
	for i := 1; i <= 3; i++ {
		s := &capability.FormSchema{
			Name:      name,
			Version:   i,
			Fields:    validFields(),
			IsActive:  true,
			CreatedBy: creator,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertSchemaVersion(ctx, s)
		}))
	}

	var list []*capability.FormSchema
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.SchemaVersions(ctx)
		return lErr
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(list), 3)

	// Verify the 3 rows we inserted come back newest-first inside the
	// returned slice (other tests in CI may have left other rows for
	// this tenant — filter to ours).
	var ours []int
	for _, s := range list {
		if s.Name == name {
			ours = append(ours, s.Version)
		}
	}
	require.Equal(t, []int{3, 2, 1}, ours, "ListByTenant must order by version DESC")
}

func TestFormSchemaRepository_ListByTenant_EmptyIsNoError(t *testing.T) {
	t.Parallel()

	db, repo, _, _ := setupSchemaRepoTest(t)
	// Use a fresh tenant so we know no rows exist.
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	var list []*capability.FormSchema
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.SchemaVersions(ctx)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

// --- NextVersion / NextVersionForName ----------------------------------

func TestFormSchemaRepository_NextVersion_StartsAt1WhenEmpty(t *testing.T) {
	t.Parallel()

	// Tenant-scoped via RLS. Use a fresh tenant id so no other test
	// rows interfere with the MAX(version) read.
	db, repo, _, _ := setupSchemaRepoTest(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	var next int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var nErr error
		next, nErr = repo.NextSchemaVersion(ctx)
		return nErr
	})
	require.NoError(t, err)
	assert.Equal(t, 1, next, "no rows → next version is 1")
}

func TestFormSchemaRepository_NextVersionForName_BumpsWithinName(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	name := uniqueSchemaName("bump")
	otherName := uniqueSchemaName("other")
	for i := 1; i <= 2; i++ {
		s := &capability.FormSchema{
			Name:      name,
			Version:   i,
			Fields:    validFields(),
			IsActive:  true,
			CreatedBy: creator,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertSchemaVersion(ctx, s)
		}))
	}
	// A row for *another* name must not influence NextVersionForName(name).
	otherS := &capability.FormSchema{
		Name: otherName, Version: 1, Fields: validFields(), IsActive: true, CreatedBy: creator,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, otherS)
	}))

	var next int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var nErr error
		next, nErr = repo.NextSchemaVersionForName(ctx, name)
		return nErr
	})
	require.NoError(t, err)
	assert.Equal(t, 3, next, "NextVersionForName MUST scope to the same name")

	// Fresh name → 1.
	freshName := uniqueSchemaName("fresh")
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var nErr error
		next, nErr = repo.NextSchemaVersionForName(ctx, freshName)
		return nErr
	})
	require.NoError(t, err)
	assert.Equal(t, 1, next, "unseen name returns 1")
}

// --- DeactivatePrevious + UpdateActiveFlag -----------------------------

func TestFormSchemaRepository_DeactivatePrevious_FlipsEveryActiveRow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	for i := 1; i <= 2; i++ {
		s := &capability.FormSchema{
			Name:      uniqueSchemaName(fmt.Sprintf("deact-%d", i)),
			Version:   1,
			Fields:    validFields(),
			IsActive:  true,
			CreatedBy: creator,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertSchemaVersion(ctx, s)
		}))
	}

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.DeactivateSchemas(ctx)
	})
	require.NoError(t, err)

	var list []*capability.FormSchema
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.SchemaVersions(ctx)
		return lErr
	})
	require.NoError(t, err)
	for _, s := range list {
		if s.CreatedBy != creator {
			continue
		}
		assert.False(t, s.IsActive, "every active row for the tenant must be deactivated (row %d)", s.ID)
	}
}

func TestFormSchemaRepository_UpdateActiveFlag_TogglesSingleRow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	schema := &capability.FormSchema{
		Name:      uniqueSchemaName("flag"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, schema)
	}))

	// Deactivate.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.SetSchemaActive(ctx, schema.ID, false)
	}))

	var got *capability.FormSchema
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Schema(ctx, schema.ID)
		return fbErr
	}))
	assert.False(t, got.IsActive)

	// Re-activate.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.SetSchemaActive(ctx, schema.ID, true)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Schema(ctx, schema.ID)
		return fbErr
	}))
	assert.True(t, got.IsActive)
}

func TestFormSchemaRepository_UpdateActiveFlag_MissingIDErrors(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _ := setupSchemaRepoTest(t)

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.SetSchemaActive(ctx, 9_999_999, true)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- DeleteByName -------------------------------------------------------

func TestFormSchemaRepository_DeleteByName_RemovesEveryVersion(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	name := uniqueSchemaName("delall")
	for i := 1; i <= 3; i++ {
		s := &capability.FormSchema{
			Name:      name,
			Version:   i,
			Fields:    validFields(),
			IsActive:  true,
			CreatedBy: creator,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertSchemaVersion(ctx, s)
		}))
	}
	// Keep a row under a *different* name to confirm it survives.
	survivor := &capability.FormSchema{
		Name:      uniqueSchemaName("keep"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, survivor)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.DeleteSchemaLineage(ctx, name)
	}))

	// Every version under `name` is gone.
	count, err := db.NewSelect().
		TableExpr("enrollment.form_schemas").
		Where("tenant_id = ? AND name = ?", tenantID, name).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count, "DeleteByName must drop every version of the name")

	// Survivor is intact.
	count, err = db.NewSelect().
		TableExpr("enrollment.form_schemas").
		Where("tenant_id = ? AND name = ?", tenantID, survivor.Name).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "different-name rows must survive")
}

func TestFormSchemaRepository_DeleteByName_UnknownNameErrors(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _ := setupSchemaRepoTest(t)

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.DeleteSchemaLineage(ctx, "no-such-schema-name-"+t.Name())
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Legal document references -----------------------------------------

func TestFormSchemaRepository_HasLegalDocumentReference(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	defer wipeSchemas(db, otherTenantID, creator)

	storedURL := "/uploads/enrollment-legal-documents/1_terms.pdf"
	publicURL := "/api/public/enrollment-legal-documents/1_terms.pdf"
	documentStoredURL := "/uploads/enrollment-legal-documents/1_document-url.pdf"
	documentPublicURL := "/api/public/enrollment-legal-documents/1_document-url.pdf"
	otherStoredURL := "/uploads/enrollment-legal-documents/2_terms.pdf"
	otherPublicURL := "/api/public/enrollment-legal-documents/2_terms.pdf"

	storedSchema := &capability.FormSchema{
		Name:      uniqueSchemaName("legal-stored"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
		LegalBlocks: []capability.FormLegalBlock{{
			Key:       enrollmentModels.ConsentKeyAGB,
			Kind:      capability.LegalBlockKindTerms,
			Title:     "AGB",
			Label:     "AGB akzeptieren",
			Text:      "Bitte lesen: " + storedURL,
			Required:  true,
			Enabled:   true,
			SortOrder: 10,
			Source:    capability.LegalBlockSourceStandard,
		}},
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, storedSchema)
	}))

	publicSchema := &capability.FormSchema{
		Name:      uniqueSchemaName("legal-public"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
		LegalBlocks: []capability.FormLegalBlock{{
			Key:       enrollmentModels.ConsentKeyAGB,
			Kind:      capability.LegalBlockKindTerms,
			Title:     "AGB",
			Label:     "AGB akzeptieren",
			Text:      "Bitte lesen: " + otherPublicURL,
			Required:  true,
			Enabled:   true,
			SortOrder: 10,
			Source:    capability.LegalBlockSourceStandard,
		}},
	}
	require.NoError(t, runInTenantTx(t, db, otherTenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, publicSchema)
	}))

	documentURLSchema := &capability.FormSchema{
		Name:      uniqueSchemaName("legal-document-url"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
		LegalBlocks: []capability.FormLegalBlock{{
			Key:         enrollmentModels.ConsentKeyAGB,
			Kind:        capability.LegalBlockKindTerms,
			Title:       "AGB",
			Label:       "AGB akzeptieren",
			Required:    true,
			Enabled:     true,
			SortOrder:   10,
			Source:      capability.LegalBlockSourceStandard,
			DisplayMode: capability.LegalBlockDisplayModePDF,
			DocumentURL: documentPublicURL,
		}},
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, documentURLSchema)
	}))

	var referenced bool
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var refErr error
		referenced, refErr = repo.SchemaReferencesLegalDocument(ctx, storedURL, publicURL)
		return refErr
	})
	require.NoError(t, err)
	assert.True(t, referenced, "stored upload URL in legal_blocks must count as a reference")

	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var refErr error
		referenced, refErr = repo.SchemaReferencesLegalDocument(ctx, documentStoredURL, documentPublicURL)
		return refErr
	})
	require.NoError(t, err)
	assert.True(t, referenced, "document_url in legal_blocks must count as a reference")

	err = runInTenantTx(t, db, otherTenantID, func(ctx context.Context) error {
		var refErr error
		referenced, refErr = repo.SchemaReferencesLegalDocument(ctx, otherStoredURL, otherPublicURL)
		return refErr
	})
	require.NoError(t, err)
	assert.True(t, referenced, "public URL in legal_blocks must count as a reference")

	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var refErr error
		referenced, refErr = repo.SchemaReferencesLegalDocument(ctx, otherStoredURL, otherPublicURL)
		return refErr
	})
	require.NoError(t, err)
	assert.False(t, referenced, "references from another tenant must not be visible")
}

// --- RenameByName -------------------------------------------------------

func TestFormSchemaRepository_RenameByName_RenamesEveryVersion(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)

	oldName := uniqueSchemaName("renall")
	for i := 1; i <= 3; i++ {
		s := &capability.FormSchema{
			Name:      oldName,
			Version:   i,
			Fields:    validFields(),
			IsActive:  i == 3,
			CreatedBy: creator,
		}
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertSchemaVersion(ctx, s)
		}))
	}
	// A row under a different name must keep its name untouched.
	survivor := &capability.FormSchema{
		Name:      uniqueSchemaName("keep"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, survivor)
	}))

	newName := uniqueSchemaName("renamed")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.RenameSchemaLineage(ctx, oldName, newName)
	}))

	// All three versions now carry the new name; old name is gone.
	count, err := db.NewSelect().
		TableExpr("enrollment.form_schemas").
		Where("tenant_id = ? AND name = ?", tenantID, newName).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, count, "RenameByName must rename every version of the lineage")

	count, err = db.NewSelect().
		TableExpr("enrollment.form_schemas").
		Where("tenant_id = ? AND name = ?", tenantID, oldName).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count, "no row may keep the old name")

	// Survivor under a different name is unaffected.
	count, err = db.NewSelect().
		TableExpr("enrollment.form_schemas").
		Where("tenant_id = ? AND name = ?", tenantID, survivor.Name).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "rows under other names must not be touched")
}

func TestFormSchemaRepository_RenameByName_UnknownNameErrors(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _ := setupSchemaRepoTest(t)

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.RenameSchemaLineage(ctx, "no-such-schema-name-"+t.Name(), "whatever")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Tenant isolation ---------------------------------------------------

func TestFormSchemaRepository_RLS_TenantIsolation(t *testing.T) {
	t.Parallel()

	// A row created under tenant A must not be visible to tenant B.
	// This guards against accidental ModelTableExpr or query construction
	// that bypasses the RLS predicate.
	db, repo, _, creator := setupSchemaRepoTest(t)

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	t.Cleanup(func() {
		_, _ = db.NewDelete().
			TableExpr("enrollment.form_schemas").
			Where("tenant_id IN (?, ?) AND created_by = ?", tenantA, tenantB, creator).
			Exec(context.Background())
	})

	schemaA := &capability.FormSchema{
		Name:      uniqueSchemaName("rlsA"),
		Version:   1,
		Fields:    validFields(),
		IsActive:  true,
		CreatedBy: creator,
	}
	require.NoError(t, runInTenantTx(t, db, tenantA, func(ctx context.Context) error {
		return repo.InsertSchemaVersion(ctx, schemaA)
	}))

	// Looking up the row from tenant B's session must fail (RLS hides
	// the row entirely → "not found" via sql.ErrNoRows).
	var got *capability.FormSchema
	err := runInTenantTx(t, db, tenantB, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.Schema(ctx, schemaA.ID)
		return fbErr
	})
	require.Error(t, err, "cross-tenant FindByID must NOT see the other tenant's row")
	assert.Nil(t, got)
}
