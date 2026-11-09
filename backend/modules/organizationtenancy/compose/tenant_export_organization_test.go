package compose_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// heldBySchools reports a table whose rows belong to a school: tenant
// scoped, or a child of such a table.
func heldBySchools(catalog organizationtenancy.DataLifecycleCatalog, table organizationtenancy.LifecycleTable) bool {
	for table.Scope == "tenantChild" {
		table = catalog.Tables[slices.IndexFunc(catalog.Tables, func(entry organizationtenancy.LifecycleTable) bool { return entry.Name == table.Parent.Table })]
	}
	return table.Scope == "tenant" || table.Scope == "tenantReference"
}

// A Träger package returns the Träger's own tables, its schools' master rows
// and the accounts of its Träger office, of this Träger only; what a school
// holds stays for the package of that school (#3838).
func TestTenantExportOrganizationPackageHoldsOnlyItsTraeger(t *testing.T) {
	t.Parallel()
	db, catalog, a, b := exportFixture(t)
	uploads := fixtureUploads(t, db, catalog, a, b)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}, openedSealedFields{}, projectedFields{})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "traeger")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{OrganizationID: a, Output: out, Uploads: uploads}))
	manifest := readExportManifest(t, out)
	require.Equal(t, "organization", manifest.Package)
	require.Len(t, manifest.Tables, len(catalog.Tables))
	assertExportArtifacts(t, out, manifest)

	exported := 0
	for _, table := range catalog.Tables {
		entry := manifest.Tables[slices.IndexFunc(manifest.Tables, func(entry organizationtenancy.ExportTable) bool { return entry.Name == table.Name })]
		file := filepath.Join(out, table.Name+".jsonl")
		switch {
		case table.Export != "include":
			require.Equal(t, table.Export, entry.Classification, table.Name)
			require.NoFileExists(t, file, table.Name)
		case heldBySchools(catalog, table):
			require.Equal(t, "schoolPackage", entry.Classification, table.Name)
			require.Zero(t, entry.Rows, table.Name)
			require.NoFileExists(t, file, table.Name)
		default:
			exported++
			require.Equal(t, "include", entry.Classification, table.Name)
			data, err := os.ReadFile(file)
			require.NoError(t, err, table.Name)
			var row map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &row), table.Name+": exactly one row of Träger A required")
			require.Equal(t, strconv.FormatInt(a, 10), string(row["fixture_school"]), table.Name)
			require.NotContains(t, string(data), fmt.Sprintf("synthetic-school-%d", b), table.Name)
			for name := range table.ExcludeSecretColumns {
				require.NotContains(t, string(data), "excluded-"+table.Name+"-"+name, table.Name)
			}
		}
	}
	require.Positive(t, exported)

	// Only the avatar of the Träger office's account, no school file.
	f, err := os.Open(filepath.Join(out, "objects.jsonl"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	var keys []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var object map[string]string
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &object))
		keys = append(keys, object["key"])
	}
	require.NoError(t, scanner.Err())
	require.Equal(t, []string{"avatars/global/" + strconv.FormatInt(a, 10) + ".bin"}, keys)
}

func TestTenantExportRejectsAmbiguousSubjects(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}, openedSealedFields{}, projectedFields{})
	require.NoError(t, err)
	school := testpkg.Tenant(t)
	for name, options := range map[string]organizationtenancy.TenantExportOptions{
		"neither":            {},
		"both":               {SchoolID: school, OrganizationID: school},
		"person of a Träger": {OrganizationID: school, Person: "kind:1"},
		"unknown Träger":     {OrganizationID: testpkg.UniqueTestTenantID(t)},
		"negative Träger":    {OrganizationID: -1},
	} {
		options.Output, options.Uploads = filepath.Join(t.TempDir(), "export"), t.TempDir()
		require.Error(t, exporter.Export(t.Context(), options), name)
		require.NoFileExists(t, filepath.Join(options.Output, "COMPLETE"), name)
	}
}
