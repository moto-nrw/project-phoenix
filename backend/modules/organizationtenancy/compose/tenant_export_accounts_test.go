package compose_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestTenantExportIncludesApplicantAccountWithoutMembership(t *testing.T) {
	t.Parallel()
	db, _, a, b := exportFixture(t)
	for _, table := range []string{"auth.account_tenants", "users.persons", "users.guardian_profiles", "users.profiles"} {
		_, err := db.NewRaw("UPDATE "+table+" SET account_id = NULL WHERE id = ?", a).Exec(t.Context())
		require.NoError(t, err)
	}
	// The applicant's only school-A link is the pending enrollment request.
	// Another school's membership of the same account must not be returned.
	_, err := db.NewRaw("UPDATE auth.account_tenants SET account_id = ? WHERE id = ?", a, b).Exec(t.Context())
	require.NoError(t, err)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: t.TempDir()}))
	data, err := os.ReadFile(filepath.Join(out, "auth.accounts.jsonl"))
	require.NoError(t, err)
	require.Contains(t, string(data), "synthetic-school-"+strconv.FormatInt(a, 10))
	require.NotContains(t, string(data), "synthetic-school-"+strconv.FormatInt(b, 10))
	memberships, err := os.ReadFile(filepath.Join(out, "auth.account_tenants.jsonl"))
	require.NoError(t, err)
	require.NotContains(t, string(memberships), "synthetic-school-"+strconv.FormatInt(b, 10))
}
