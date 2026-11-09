//go:build testhooks

package compose_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// This extends the existing schema-shaped exporter fixture, not the migrated
// schema/real-native export proof. Run the same test on original and candidate.
// No candidate branch, new query budget or product hook is added to this test.
func TestTenantExportMetadataFailureBoundary(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"MissingOwnTable", "ZeroColumnOwnTable"} {
		t.Run(mode, func(t *testing.T) {
			testpkg.OwnTenant(t)
			// exportFixture registers its isolated clone after OwnTenant, creates
			// all catalog fixture tables and commits rows for both real school IDs.
			db, catalog, school, otherSchool := exportFixture(t)
			uploads := fixtureUploads(t, db, catalog, school, otherSchool)
			exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{
				Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint,
			}, openedSealedFields{})
			require.NoError(t, err)
			const target = "platform.organizations"
			var prefix []string
			found := false
			for _, table := range catalog.Tables {
				if table.Name == target {
					require.Equal(t, "include", table.Export)
					found = true
					break
				}
				if table.Export == "include" {
					prefix = append(prefix, table.Name+".jsonl")
				}
			}
			require.True(t, found)
			require.NotEmpty(t, prefix, "the failure must have a non-vacuous completed artifact prefix")

			complete := filepath.Join(t.TempDir(), "positive")
			positiveErr := exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: school, Output: complete, Uploads: uploads})
			positivePrefix, positivePrefixErr := metadataFailureReadPrefix(complete, prefix)
			t.Logf("metadata-failure positive_error=%v positive_prefix_read_error=%v prefix_files=%d positive_prefix=%v", positiveErr, positivePrefixErr, len(positivePrefix), metadataFailureDigests(positivePrefix))
			require.NoError(t, positiveErr)
			require.NoError(t, positivePrefixErr)
			manifest := readExportManifest(t, complete)
			assertExportArtifacts(t, complete, manifest)

			// Only this owner's relation in the disposable synthetic clone is
			// changed. Plain DROP deliberately does not bypass unexpected FKs.
			_, err = db.ExecContext(t.Context(), "DROP TABLE platform.organizations")
			require.NoError(t, err)
			if mode == "ZeroColumnOwnTable" {
				_, err = db.ExecContext(t.Context(), "CREATE TABLE platform.organizations ()")
				require.NoError(t, err)
				var columns int
				err = db.NewRaw(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'platform' AND table_name = 'organizations'`).Scan(t.Context(), &columns)
				require.NoError(t, err)
				require.Zero(t, columns)
				var canRead bool
				err = db.NewRaw(`SELECT has_table_privilege('phoenix_tenant_export', 'platform.organizations', 'SELECT')`).Scan(t.Context(), &canRead)
				require.NoError(t, err)
				require.True(t, canRead, "zero columns, not a missing reader privilege, is the arrangement")
			}

			partial := filepath.Join(t.TempDir(), "partial")
			counter := testpkg.CaptureQueriesForContext(t, db)
			operationErr := exporter.Export(counter.Context(t.Context()), organizationtenancy.TenantExportOptions{SchoolID: school, Output: partial, Uploads: uploads})
			counter.Stop()
			actualPrefix, partialReadErr := metadataFailureReadDirectory(partial)
			queries := counter.Queries()
			t.Logf("metadata-failure mode=%s operation_error=%v events=%d driver_write_rows=%d partial_read_error=%v actual_files=%d actual_prefix=%v", mode, operationErr, len(queries), counter.WriteRows(), partialReadErr, len(actualPrefix), metadataFailureDigests(actualPrefix))
			for i, query := range queries {
				t.Logf("metadata-failure SQL[%d]=%s", i, query)
			}
			// Establish the exact partial-output contract before asserting the
			// expected error. No generated field or file content is normalized.
			require.NoError(t, partialReadErr)
			require.Equal(t, positivePrefix, actualPrefix, "only predecessor table artifacts, with their exact same-clone positive bytes, may remain")
			t.Logf("metadata-failure prefix_byte_equal=true files=%d", len(actualPrefix))
			require.EqualError(t, operationErr, "tenant export: table platform.organizations failed")
			require.NotEmpty(t, queries)
			require.Empty(t, counter.Operation("INSERT"))
			require.Empty(t, counter.Operation("UPDATE"))
			require.Empty(t, counter.Operation("DELETE"))
			require.NoFileExists(t, filepath.Join(partial, target+".jsonl"))
			require.NoFileExists(t, filepath.Join(partial, "model.json"))
			require.NoFileExists(t, filepath.Join(partial, "manifest.json"))
			require.NoFileExists(t, filepath.Join(partial, "COMPLETE"))
		})
	}
}

func metadataFailureReadPrefix(root string, names []string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return out, err
		}
		out[name] = data
	}
	return out, nil
}

func metadataFailureReadDirectory(root string) (map[string][]byte, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("unexpected non-regular partial artifact %s", entry.Name())
		}
		names = append(names, entry.Name())
	}
	return metadataFailureReadPrefix(root, names)
}

func metadataFailureDigests(files map[string][]byte) []string {
	out := make([]string, 0, len(files))
	for name, data := range files {
		out = append(out, fmt.Sprintf("%s bytes=%d sha256=%s", name, len(data), testpkg.ContentFingerprint(data)))
	}
	slices.Sort(out)
	return out
}
