package cmd

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The return of a school carries the health field of a child opened with
// the key ring of the installation, never its ciphertext (#3838, #6328).
func TestTenantExportOpensHealthWithTheRootBoundKeyRings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Synthetic", "Health", "1a")
	health := "Synthetische Erdnussallergie"
	testpkg.SetTestStudentHealth(t, db, student.ID, &health)

	sealed, err := tenantExportSealedFields()
	require.NoError(t, err)
	exporter, err := organizationCompose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{
		Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint,
	}, sealed)
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: school, Output: out, Uploads: t.TempDir()}))
	require.FileExists(t, filepath.Join(out, "COMPLETE"))

	file, err := os.Open(filepath.Join(out, "users.student_care_profiles.jsonl"))
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	var opened []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &row))
		require.NotContains(t, row, "health_data", "the ciphertext never leaves")
		var value *string
		require.NoError(t, json.Unmarshal(row["health_info"], &value))
		if value != nil {
			opened = append(opened, *value)
		}
	}
	require.NoError(t, scanner.Err())
	require.Equal(t, []string{health}, opened)
}
