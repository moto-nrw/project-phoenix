package compose_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Löschkonzept §5 asks the return to hand over the documents themselves,
// not only their rows: letters, fee notices, every contract version, paper
// contracts, signature images and proofs. The Träger package writes each
// stored document of its own tables as a file under DOKUMENTE/ with an
// index; the rows stay as they are (ADR 0187).

type documentIndexEntry struct {
	Path      string                     `json:"path"`
	Table     string                     `json:"table"`
	Column    string                     `json:"column"`
	RowID     string                     `json:"row_id"`
	RowKey    map[string]json.RawMessage `json:"row_key"`
	MediaType string                     `json:"media_type"`
	SHA256    string                     `json:"sha256"`
	Bytes     int64                      `json:"bytes"`
}

type documentFilesSummary struct {
	DocumentFiles *struct {
		Directory string `json:"directory"`
		Index     string `json:"index"`
		Columns   []struct {
			Table  string `json:"table"`
			Column string `json:"column"`
			Files  int    `json:"files"`
		} `json:"columns"`
	} `json:"documentFiles"`
}

// The document kinds the Löschkonzept names; the migrated schema must still
// hold each as a binary column the Träger package returns.
var namedDocumentKinds = []string{
	"parent_billing.care_letters.pdf", "parent_billing.document_letters.pdf",
	"care_contract.contract_documents.pdf", "care_contract.contract_signatures.paper_pdf",
	"audit.traeger_signatures.image_png", "enrollment.holiday_registration_proofs.content",
}

// migratedDocumentColumns answers every binary column the catalog exports
// from a table a Träger holds, read from the migrated schema, so a new
// document column is covered without editing this test.
// It runs before the fixture: that replaces the schema of its own database.
func migratedDocumentColumns(t *testing.T) map[string][]string {
	t.Helper()
	source, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	require.NoError(t, json.Unmarshal(source, &catalog))
	var rows []struct {
		TableName string
		Column    string
	}
	// pg_attribute, not information_schema: the test role need not hold a
	// privilege on every table to see its columns.
	require.NoError(t, testpkg.SetupTestDB(t).NewRaw(`SELECT n.nspname || '.' || c.relname AS table_name, a.attname AS column
		FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND a.attnum > 0 AND NOT a.attisdropped AND a.atttypid = 'bytea'::regtype
		ORDER BY 1, 2`).Scan(t.Context(), &rows))
	columns := map[string][]string{}
	for _, row := range rows {
		index := slices.IndexFunc(catalog.Tables, func(entry organizationtenancy.LifecycleTable) bool { return entry.Name == row.TableName })
		if index < 0 {
			continue
		}
		table := catalog.Tables[index]
		if _, excluded := table.ExcludeSecretColumns[row.Column]; excluded || table.Export != "include" || heldBySchools(catalog, table) {
			continue
		}
		columns[table.Name] = append(columns[table.Name], row.Column)
	}
	for _, kind := range namedDocumentKinds {
		dot := strings.LastIndex(kind, ".")
		require.Contains(t, columns[kind[:dot]], kind[dot+1:], "%s is a document column of the Träger package", kind)
	}
	return columns
}

// syntheticDocument is a document of one Träger: a PDF unless the column
// holds an image.
func syntheticDocument(table, column string, organization int64, version int) ([]byte, string, string) {
	label := fmt.Sprintf("synthetic-%s-%s-%d-v%d", table, column, organization, version)
	switch {
	case column == "image_png" || column == "logo":
		return append([]byte("\x89PNG\r\n\x1a\n"), label...), "image/png", ".png"
	case table == "enrollment.holiday_registration_proofs":
		return append([]byte("\xff\xd8\xff\xe0"), label...), "image/jpeg", ".jpg"
	default:
		return []byte("%PDF-1.7\n" + label), "application/pdf", ".pdf"
	}
}

// documentFixture adds the migrated document columns to the fixture tables
// and stores one document of each kind per Träger, and a second contract
// version for Träger A. It answers the expected files of Träger A by path.
func documentFixture(t *testing.T, db *bun.DB, documents map[string][]string, a, b int64) map[string][]byte {
	t.Helper()
	want := map[string][]byte{}
	for table, columns := range documents {
		for _, column := range columns {
			_, err := db.ExecContext(t.Context(), "ALTER TABLE "+table+" ADD COLUMN "+exportIdent(column)+" bytea")
			require.NoError(t, err)
		}
		if table == "enrollment.holiday_registration_proofs" {
			_, err := db.ExecContext(t.Context(), "ALTER TABLE "+table+" ADD COLUMN content_type text")
			require.NoError(t, err)
		}
		if table == "care_contract.contract_documents" {
			// Every version of a contract is returned, not only the latest.
			_, err := db.ExecContext(t.Context(), "INSERT INTO "+table+" (id, fixture_school, organization_id) VALUES (?, ?, ?)", a+1000, a, a)
			require.NoError(t, err)
		}
		for _, organization := range []int64{a, b} {
			for _, column := range columns {
				data, mediaType, ext := syntheticDocument(table, column, organization, 1)
				_, err := db.ExecContext(t.Context(), "UPDATE "+table+" SET "+exportIdent(column)+" = ? WHERE id = ?", data, organization)
				require.NoError(t, err)
				if table == "enrollment.holiday_registration_proofs" {
					_, err = db.ExecContext(t.Context(), "UPDATE "+table+" SET content_type = ? WHERE id = ?", mediaType, organization)
					require.NoError(t, err)
				}
				if organization == a {
					want["DOKUMENTE/"+table+"/"+strconv.FormatInt(a, 10)+"-"+column+ext] = data
				}
				if table == "care_contract.contract_documents" && organization == a {
					second, _, _ := syntheticDocument(table, column, a, 2)
					_, err = db.ExecContext(t.Context(), "UPDATE "+table+" SET "+exportIdent(column)+" = ? WHERE id = ?", second, a+1000)
					require.NoError(t, err)
					want["DOKUMENTE/"+table+"/"+strconv.FormatInt(a+1000, 10)+"-"+column+ext] = second
				}
			}
		}
	}
	return want
}

func readDocumentIndex(t *testing.T, out string) []documentIndexEntry {
	t.Helper()
	f, err := os.Open(filepath.Join(out, "DOKUMENTE", "index.jsonl"))
	require.NoError(t, err, "the documents have an index")
	defer func() { require.NoError(t, f.Close()) }()
	var entries []documentIndexEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry documentIndexEntry
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &entry))
		entries = append(entries, entry)
	}
	require.NoError(t, scanner.Err())
	return entries
}

func TestTenantExportOrganizationWritesEveryStoredDocumentAsAFile(t *testing.T) {
	t.Parallel()
	documents := migratedDocumentColumns(t)
	db, catalog, a, b := exportFixture(t)
	want := documentFixture(t, db, documents, a, b)
	audit := &exportAuditFake{}
	exporter := newTraegerExporter(t, db, &sepaFilesFake{}, audit)
	out := filepath.Join(t.TempDir(), "traeger")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{OrganizationID: a, OperatorID: 42, Output: out, Uploads: fixtureUploads(t, db, catalog, a, b)}))
	manifest := readExportManifest(t, out)
	assertExportArtifacts(t, out, manifest)

	entries := readDocumentIndex(t, out)
	got := map[string]bool{}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(entry.Path)))
		require.NoError(t, err, entry.Path)
		require.Equal(t, want[entry.Path], data, "%s: exactly the stored document of Träger A", entry.Path)
		require.Equal(t, testpkg.ContentFingerprint(data), entry.SHA256, entry.Path)
		require.Equal(t, int64(len(data)), entry.Bytes, entry.Path)
		require.Equal(t, "DOKUMENTE/"+entry.Table+"/"+entry.RowID+"-"+entry.Column+filepath.Ext(entry.Path), entry.Path)
		require.Equal(t, entry.RowID, string(entry.RowKey["id"]), entry.Path)
		require.NotEmpty(t, entry.MediaType, entry.Path)
		require.Contains(t, artifactPaths(manifest), entry.Path, "the manifest checksums every document")
		got[entry.Path] = true
	}
	require.Len(t, got, len(want), "every stored document of Träger A, each once, none of Träger B")
	for path := range want {
		require.True(t, got[path], path)
	}
	require.Contains(t, artifactPaths(manifest), "DOKUMENTE/index.jsonl")
	notice, err := os.ReadFile(filepath.Join(out, "DOKUMENTE", "LIESMICH.txt"))
	require.NoError(t, err)
	require.Contains(t, string(notice), "index.jsonl")

	// The rows keep their contract (#6329): the document stays in the row.
	rows, err := os.ReadFile(filepath.Join(out, "parent_billing.care_letters.jsonl"))
	require.NoError(t, err)
	var row map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rows, &row))
	require.Contains(t, string(row["pdf"]), `\\x`, "the row still carries the document")

	// Manifest and audit count the file set without contents.
	manifestData, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	require.NoError(t, err)
	var summary documentFilesSummary
	require.NoError(t, json.Unmarshal(manifestData, &summary))
	require.NotNil(t, summary.DocumentFiles, "the manifest marks the documents")
	require.Equal(t, "DOKUMENTE", summary.DocumentFiles.Directory)
	require.Equal(t, "DOKUMENTE/index.jsonl", summary.DocumentFiles.Index)
	files := 0
	for _, column := range summary.DocumentFiles.Columns {
		files += column.Files
	}
	require.Equal(t, len(want), files)
	require.Len(t, audit.records, 1)
	require.Equal(t, manifest.DocumentFiles, audit.records[0].DocumentFiles, "the audit counts the documents")
	encoded, err := json.Marshal(audit.records[0])
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "synthetic-", "the audit holds no document content")

	// Träger B's documents never appear.
	require.NoError(t, filepath.WalkDir(filepath.Join(out, "DOKUMENTE"), func(path string, entry os.DirEntry, err error) error {
		require.NoError(t, err)
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.NotContains(t, string(data), fmt.Sprintf("-%d-v", b), path)
		}
		return nil
	}))
}

// A document column the package cannot write fails the package: no
// COMPLETE, a failure record with its class, no partial files.
func TestTenantExportOrganizationFailsWhenADocumentCannotBeWritten(t *testing.T) {
	t.Parallel()
	documents := migratedDocumentColumns(t)
	db, catalog, a, b := exportFixture(t)
	documentFixture(t, db, documents, a, b)
	// Without a primary key the document cannot be named by its row.
	_, err := db.ExecContext(t.Context(), "ALTER TABLE parent_billing.care_letters DROP CONSTRAINT care_letters_pkey")
	require.NoError(t, err)
	audit := &exportAuditFake{}
	exporter := newTraegerExporter(t, db, &sepaFilesFake{}, audit)
	out := filepath.Join(t.TempDir(), "traeger")
	require.Error(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{OrganizationID: a, OperatorID: 42, Output: out, Uploads: fixtureUploads(t, db, catalog, a, b)}))
	require.NoDirExists(t, out, "the partial files are removed")
	require.Len(t, audit.records, 1)
	require.Equal(t, organizationtenancy.TenantExportFailed, audit.records[0].Outcome)
	require.Equal(t, "document_files", audit.records[0].ErrorClass)
	require.True(t, audit.records[0].PartialFilesRemoved)
}

// On the migrated schema the Träger logo, keyed by organization_id, is a
// file of exactly its Träger.
func TestTenantExportOrganizationMigratedSchemaWritesTheLogo(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	organizations := map[int64]int64{}
	for range 2 {
		school, _ := testpkg.CreateTestTenant(t, db)
		var organization int64
		require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(t.Context(), &organization))
		logo, _, _ := syntheticDocument("platform.traeger_profiles", "logo", organization, 1)
		_, err := db.ExecContext(t.Context(), `INSERT INTO platform.traeger_profiles
			(organization_id, logo, logo_content_type, logo_width, logo_height, logo_updated_at)
			VALUES (?, ?, 'image/png', 1, 1, clock_timestamp())`, organization, logo)
		require.NoError(t, err)
		organizations[school] = organization
	}
	var a, b int64
	for _, organization := range organizations {
		if a == 0 {
			a = organization
		} else {
			b = organization
		}
	}
	require.NotEqual(t, a, b)
	exporter := newTraegerExporter(t, db, &sepaFilesFake{}, &exportAuditFake{})
	out := filepath.Join(t.TempDir(), "traeger")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{OrganizationID: a, OperatorID: 42, Output: out, Uploads: t.TempDir()}))
	assertExportArtifacts(t, out, readExportManifest(t, out))
	entries := readDocumentIndex(t, out)
	require.Len(t, entries, 1, "exactly the logo of Träger A")
	want := "DOKUMENTE/platform.traeger_profiles/" + strconv.FormatInt(a, 10) + "-logo.png"
	require.Equal(t, want, entries[0].Path)
	require.Equal(t, "image/png", entries[0].MediaType)
	require.Equal(t, strconv.FormatInt(a, 10), string(entries[0].RowKey["organization_id"]))
	data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(want)))
	require.NoError(t, err)
	logo, _, _ := syntheticDocument("platform.traeger_profiles", "logo", a, 1)
	require.Equal(t, logo, data)
}
