package compose_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Legal-reference cases need real form/request rows, not a rebuilt copy of
// every catalog table. Each top-level test owns its tenant in the package DB.
func legalDocumentExportFixture(t *testing.T, db *bun.DB) (school, form, request int64) {
	t.Helper()
	school = testpkg.Tenant(t)
	author := testpkg.CreateTestAccount(t, db, "legal-document-export")
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.form_schemas
		(tenant_id, version, name, created_by) VALUES (?, 1, 'Synthetic form', ?) RETURNING id`, school, author.ID).Scan(t.Context(), &form))
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	applicant := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID,
		fmt.Sprintf("legal-%d@example.invalid", school), fmt.Sprintf("legal-export-%d", school))
	return school, form, applicant.ID
}

func TestTenantExportRequiresFormLegalDocumentOriginal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school, form, _ := legalDocumentExportFixture(t, db)
	filename := strconv.FormatInt(school, 10) + "_legal.pdf"
	blocks, err := json.Marshal([]map[string]string{{"document_url": "/api/public/enrollment-form-legal-documents/" + filename}})
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE enrollment.form_schemas SET legal_blocks = ?::jsonb WHERE id = ?", string(blocks), form).Exec(t.Context())
	require.NoError(t, err)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}, openedSealedFields{}, projectedFields{}, &exportAuditFake{})
	require.NoError(t, err)
	uploads := t.TempDir()
	// A missing namespace must not silently turn into a completed export.
	assertFailedExport(t, exporter, school, uploads)
	dir := filepath.Join(uploads, "enrollment-form-legal-documents")
	require.NoError(t, os.Mkdir(dir, 0o700))
	// The namespace alone cannot satisfy a reference to an absent PDF.
	assertFailedExport(t, exporter, school, uploads)
	original := []byte("synthetic legal document original")
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), original, 0o600))
	out := filepath.Join(t.TempDir(), "complete")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: 42, Output: out, Uploads: uploads}))
	manifest := readExportManifest(t, out)
	assertExportArtifacts(t, out, manifest)
	data, err := os.ReadFile(filepath.Join(out, "objects.jsonl"))
	require.NoError(t, err)
	var object map[string]string
	require.NoError(t, json.Unmarshal(data, &object))
	require.Equal(t, "enrollment-form-legal-documents/"+filename, object["key"])
	data, err = os.ReadFile(filepath.Join(out, object["path"]))
	require.NoError(t, err)
	require.Equal(t, original, data)
}

func TestTenantExportRequiresEmbeddedFormAttachments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school, form, request := legalDocumentExportFixture(t, db)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}, openedSealedFields{}, projectedFields{}, &exportAuditFake{})
	require.NoError(t, err)
	cases := []struct {
		name, table, column, payload string
		id                           int64
	}{
		{"field attachments", "enrollment.form_schemas", "fields", `[{"attachments":[{"document_url":%q}]}]`, form},
		{"legal attachments", "enrollment.form_schemas", "legal_blocks", `[{"attachments":[{"document_url":%q}]}]`, form},
		{"submission snapshot", "enrollment.requests", "legal_blocks_snapshot", `[{"blocks":[{"attachments":[{"document_url":%q}]}]}]`, request},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, query := range []string{"UPDATE enrollment.form_schemas SET fields = '[]', legal_blocks = '[]' WHERE tenant_id = ?", "UPDATE enrollment.requests SET legal_blocks_snapshot = '[]' WHERE tenant_id = ?"} {
				_, err := db.ExecContext(t.Context(), query, school)
				require.NoError(t, err)
			}
			filename := strconv.FormatInt(school, 10) + fmt.Sprintf("_attachment_%d.pdf", i)
			payload := fmt.Sprintf(tc.payload, "/uploads/enrollment-form-legal-documents/"+filename)
			_, err := db.NewRaw("UPDATE "+tc.table+" SET "+tc.column+" = ?::jsonb WHERE id = ?", payload, tc.id).Exec(t.Context())
			require.NoError(t, err)
			uploads := t.TempDir()
			assertFailedExport(t, exporter, school, uploads)
			dir := filepath.Join(uploads, "enrollment-form-legal-documents")
			require.NoError(t, os.Mkdir(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte("synthetic attachment original"), 0o600))
			out := filepath.Join(t.TempDir(), "complete")
			require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: 42, Output: out, Uploads: uploads}))
			assertExportArtifacts(t, out, readExportManifest(t, out))
		})
	}
}

func TestTenantExportRejectsForeignLegalDocumentReferences(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school, form, _ := legalDocumentExportFixture(t, db)
	foreign, _ := testpkg.CreateTestTenant(t, db)
	filename := strconv.FormatInt(foreign, 10) + "_foreign.pdf"
	uploads := t.TempDir()
	dir := filepath.Join(uploads, "enrollment-form-legal-documents")
	require.NoError(t, os.Mkdir(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte("synthetic foreign original"), 0o600))
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}, openedSealedFields{}, projectedFields{}, &exportAuditFake{})
	require.NoError(t, err)
	for _, prefix := range []string{"/uploads/", "/api/public/"} {
		blocks, err := json.Marshal([]map[string]string{{"document_url": prefix + "enrollment-form-legal-documents/" + filename}})
		require.NoError(t, err)
		_, err = db.NewRaw("UPDATE enrollment.form_schemas SET legal_blocks = ?::jsonb WHERE id = ?", string(blocks), form).Exec(t.Context())
		require.NoError(t, err)
		assertFailedExport(t, exporter, school, uploads)
	}
	// Looking like a URL in arbitrary text never grants access to a global avatar.
	blocks, err := json.Marshal([]map[string]string{{"text": "/uploads/avatars/global/" + filename}})
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE enrollment.form_schemas SET legal_blocks = ?::jsonb WHERE id = ?", string(blocks), form).Exec(t.Context())
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "complete")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: 42, Output: out, Uploads: uploads}))
	readExportManifest(t, out)
	data, err := os.ReadFile(filepath.Join(out, "objects.jsonl"))
	require.NoError(t, err)
	require.Empty(t, data)
}
