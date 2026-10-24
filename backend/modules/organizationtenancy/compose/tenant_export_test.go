package compose_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// This schema-shaped fixture gives EVERY catalog table two actual rows, even
// tables whose business fixtures are normally empty. A second migrated-schema
// test below checks that the executor also runs against the real migrations.
func exportFixture(t *testing.T) (*bun.DB, organizationtenancy.DataLifecycleCatalog, int64, int64) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	a, _ := testpkg.CreateTestTenant(t, db)
	b, _ := testpkg.CreateTestTenant(t, db)
	data, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	require.NoError(t, json.Unmarshal(data, &catalog))
	resetExportSchema(t, db, catalog)
	for _, table := range catalog.Tables {
		createExportFixtureTable(t, db, table)
	}
	for _, table := range catalog.Tables {
		for _, school := range []int64{a, b} {
			insertExportFixtureRow(t, db, table, school)
		}
	}
	return db, catalog, a, b
}

func resetExportSchema(t *testing.T, db *bun.DB, catalog organizationtenancy.DataLifecycleCatalog) {
	t.Helper()
	schemas := map[string]bool{}
	for _, table := range catalog.Tables {
		schemas[strings.Split(table.Name, ".")[0]] = true
	}
	for schema := range schemas {
		_, err := db.ExecContext(t.Context(), "DROP SCHEMA "+exportIdent(schema)+" CASCADE")
		require.NoError(t, err)
		_, err = db.ExecContext(t.Context(), "CREATE SCHEMA "+exportIdent(schema))
		require.NoError(t, err)
		_, err = db.ExecContext(t.Context(), "GRANT USAGE ON SCHEMA "+exportIdent(schema)+" TO phoenix_admin")
		require.NoError(t, err)
	}
}

func exportIdent(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func exportFixtureColumns(table organizationtenancy.LifecycleTable) map[string]string {
	columns := map[string]string{"id": "bigint PRIMARY KEY", "fixture_school": "bigint", "fixture_label": "text"}
	if table.Scope == "tenant" {
		columns["tenant_id"] = "bigint"
	}
	if table.ScopeColumn != "" && table.ScopeColumn != "id" {
		columns[table.ScopeColumn] = "bigint"
	}
	if table.Parent != nil {
		columns[table.Parent.Column] = "bigint"
	}
	for name := range table.ExcludeSecretColumns {
		columns[name] = "text"
	}
	if table.Name == "enrollment.requests" {
		columns["guardian_account_id"] = "bigint"
	}
	for _, name := range []string{"account_id", "user_id", "organization_id", "announcement_id"} {
		columns[name] = "bigint"
	}
	columns["email"], columns["avatar"], columns["target_school_ids"] = "text", "text", "bigint[]"
	if table.Name == "auth.accounts" {
		columns["username"] = "text"
	}
	if slices.Contains([]string{"documents.files", "documents.announcement_attachments", "users.student_documents", "users.staff_documents"}, table.Name) {
		columns["filename_stored"], columns["file_deleted_at"] = "text", "timestamptz"
	}
	return columns
}

func createExportFixtureTable(t *testing.T, db *bun.DB, table organizationtenancy.LifecycleTable) {
	t.Helper()
	definitions := []string{}
	for name, typ := range exportFixtureColumns(table) {
		definitions = append(definitions, exportIdent(name)+" "+typ)
	}
	slices.Sort(definitions)
	_, err := db.ExecContext(t.Context(), "CREATE TABLE "+table.Name+" ("+strings.Join(definitions, ",")+")")
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), "GRANT SELECT ON "+table.Name+" TO phoenix_admin")
	require.NoError(t, err)
}

func insertExportFixtureRow(t *testing.T, db *bun.DB, table organizationtenancy.LifecycleTable, school int64) {
	t.Helper()
	row := map[string]any{"id": school, "fixture_school": school, "fixture_label": fmt.Sprintf("synthetic-school-%d", school),
		"account_id": school, "user_id": school, "organization_id": school, "announcement_id": school, "email": fmt.Sprintf("fixture-%d@example.invalid", school)}
	if table.Scope == "tenant" {
		row["tenant_id"] = school
	}
	if table.ScopeColumn != "" {
		row[table.ScopeColumn] = school
	}
	if table.Parent != nil {
		row[table.Parent.Column] = school
	}
	if table.Name == "enrollment.requests" {
		row["guardian_account_id"] = school
	}
	for name := range table.ExcludeSecretColumns {
		row[name] = "excluded-" + table.Name + "-" + name
	}
	_, err := db.NewInsert().TableExpr(table.Name).Model(&row).Exec(t.Context())
	require.NoError(t, err)
}

func fixtureUploads(t *testing.T, db *bun.DB, catalog organizationtenancy.DataLifecycleCatalog, a, b int64) string {
	t.Helper()
	root := t.TempDir()
	for _, kind := range catalog.Files {
		for _, school := range []int64{a, b} {
			key := kind.Kind + "/" + strconv.FormatInt(school, 10) + "/original.bin"
			if kind.Layout == "tenantPrefix" {
				key = kind.Kind + "/" + strconv.FormatInt(school, 10) + "_original.bin"
			}
			if kind.Layout == "account" {
				key = kind.Kind + "/global/" + strconv.FormatInt(school, 10) + ".bin"
				_, err := db.NewRaw("UPDATE auth.accounts SET avatar = ? WHERE id = ?", "/uploads/"+key, school).Exec(t.Context())
				require.NoError(t, err)
			}
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, key)), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(root, key), []byte(fmt.Sprintf("synthetic-original-%d", school)), 0o600))
		}
	}
	return root
}

func TestTenantExportEveryCatalogTableAndKindIsIsolated(t *testing.T) {
	t.Parallel()
	db, catalog, a, b := exportFixture(t)
	uploads := fixtureUploads(t, db, catalog, a, b)
	// A username is user content, never authority to copy another account's
	// global avatar. Only the selected account's actual avatar field is a reference.
	_, err := db.NewRaw("UPDATE auth.accounts SET username = ? WHERE id = ?", "/uploads/avatars/global/"+strconv.FormatInt(b, 10)+".bin", a).Exec(t.Context())
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: uploads}))
	manifest := readExportManifest(t, out)
	require.Len(t, manifest.Tables, len(catalog.Tables))
	assertExportTables(t, out, catalog, manifest, a, b)
	assertExportArtifacts(t, out, manifest)
	assertExportObjects(t, out, catalog, a, b)
	// Repeating into a fresh directory yields the same table/file bytes.
	second := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: second, Uploads: uploads}))
	require.Equal(t, manifest.Artifacts, readExportManifest(t, second).Artifacts)
	// Existing completed attempts are never overwritten or mixed with a retry.
	require.Error(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: uploads}))
	assertExportArtifacts(t, out, manifest)
}

func readExportManifest(t *testing.T, out string) organizationtenancy.ExportManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	require.NoError(t, err)
	marker, err := os.ReadFile(filepath.Join(out, "COMPLETE"))
	require.NoError(t, err)
	digest := testpkg.ContentFingerprint(data)
	require.Equal(t, digest+"\n", string(marker))
	var manifest organizationtenancy.ExportManifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	require.NotEmpty(t, manifest.ExportedAt)
	require.Equal(t, 1, manifest.CatalogVersion)
	return manifest
}

func assertExportTables(t *testing.T, out string, catalog organizationtenancy.DataLifecycleCatalog, manifest organizationtenancy.ExportManifest, a, b int64) {
	t.Helper()
	modelData, err := os.ReadFile(filepath.Join(out, "model.json"))
	require.NoError(t, err)
	var model []organizationtenancy.ExportModelTable
	require.NoError(t, json.Unmarshal(modelData, &model))
	for _, table := range model {
		definition := catalog.Tables[slices.IndexFunc(catalog.Tables, func(entry organizationtenancy.LifecycleTable) bool { return entry.Name == table.Name })]
		for _, column := range table.Columns {
			require.NotContains(t, definition.ExcludeSecretColumns, column.Name, table.Name)
		}
	}
	for _, table := range catalog.Tables {
		file := filepath.Join(out, table.Name+".jsonl")
		if table.Export != "include" {
			require.NoFileExists(t, file)
			index := slices.IndexFunc(manifest.Tables, func(entry organizationtenancy.ExportTable) bool { return entry.Name == table.Name })
			require.NotEqual(t, -1, index, table.Name)
			entry := manifest.Tables[index]
			require.Equal(t, table.Export, entry.Classification, table.Name)
			require.NotEmpty(t, entry.Reason, table.Name)
			require.Equal(t, table.ExportReason, entry.Reason, table.Name)
			require.Zero(t, entry.Rows, table.Name)
			require.Equal(t, -1, slices.IndexFunc(model, func(entry organizationtenancy.ExportModelTable) bool { return entry.Name == table.Name }), table.Name)
			continue
		}
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		var row map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &row), table.Name+": exactly one row of A required")
		require.Equal(t, strconv.FormatInt(a, 10), string(row["fixture_school"]), table.Name)
		require.NotContains(t, string(data), fmt.Sprintf("synthetic-school-%d", b), table.Name)
		for name := range table.ExcludeSecretColumns {
			require.NotContains(t, row, name, table.Name)
			require.NotContains(t, string(data), "excluded-"+table.Name+"-"+name, table.Name)
		}
	}
}

func assertExportArtifacts(t *testing.T, out string, manifest organizationtenancy.ExportManifest) {
	t.Helper()
	for _, artifact := range manifest.Artifacts {
		data, err := os.ReadFile(filepath.Join(out, artifact.Path))
		require.NoError(t, err)
		digest := testpkg.ContentFingerprint(data)
		require.Equal(t, artifact.SHA256, digest, artifact.Path)
		require.Equal(t, artifact.Bytes, int64(len(data)), artifact.Path)
		info, err := os.Stat(filepath.Join(out, artifact.Path))
		require.NoError(t, err)
		require.Zero(t, info.Mode().Perm()&0o077)
	}
}

func assertExportObjects(t *testing.T, out string, catalog organizationtenancy.DataLifecycleCatalog, a, b int64) {
	t.Helper()
	f, err := os.Open(filepath.Join(out, "objects.jsonl"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var object map[string]string
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &object))
		seen[strings.Split(object["key"], "/")[0]] = true
		data, err := os.ReadFile(filepath.Join(out, object["path"]))
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("synthetic-original-%d", a), string(data))
		require.NotContains(t, string(data), fmt.Sprintf("synthetic-original-%d", b))
	}
	require.NoError(t, scanner.Err())
	require.Len(t, seen, len(catalog.Files))
}

func TestTenantExportMigratedSchema(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	a := testpkg.Tenant(t)
	b, _ := testpkg.CreateTestTenant(t, db)
	testpkg.CreateTestStudentForTenant(t, db, a, "Synthetic A", "Fixture", "A")
	testpkg.CreateTestStudentForTenant(t, db, b, "Synthetic B", "Fixture", "B")
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(context.Background(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: t.TempDir()}))
	manifest := readExportManifest(t, out)
	assertExportArtifacts(t, out, manifest)
	data, err := os.ReadFile(filepath.Join(out, "users.persons.jsonl"))
	require.NoError(t, err)
	require.Contains(t, string(data), "Synthetic A")
	require.NotContains(t, string(data), "Synthetic B")
	modelData, err := os.ReadFile(filepath.Join(out, "model.json"))
	require.NoError(t, err)
	var model []organizationtenancy.ExportModelTable
	require.NoError(t, json.Unmarshal(modelData, &model))
	person := model[slices.IndexFunc(model, func(table organizationtenancy.ExportModelTable) bool { return table.Name == "users.persons" })]
	require.Contains(t, person.Relationships, organizationtenancy.ExportRelationship{Column: "account_id", TargetTable: "auth.accounts", TargetColumn: "id"})
}
