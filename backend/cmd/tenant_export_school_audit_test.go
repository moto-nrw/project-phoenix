package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The school package as the CLI binds it is in the operator audit like the
// Träger package (#6326): the attempt before the extraction, the outcome
// after it, on the school, without personal data.
func TestTenantExportSchoolIsInTheOperatorAudit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := tenant.WithUnitOfWork(context.Background(), database.UnitOfWork())
	school := testpkg.Tenant(t)
	const name = "Synthetisch-Auditkind"
	testpkg.CreateTestStudentForTenant(t, db, school, name, "Rueckgabe", "1a")
	operator := testpkg.CreateTestOperator(t, db)
	exporter, err := newTenantExporter(database, returnSealedFields(t), tenantExportProjectedFields(), false)
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(ctx, organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: operator.ID, Output: out, Uploads: t.TempDir()}))

	var entries []struct {
		Changes json.RawMessage `bun:"changes"`
	}
	require.NoError(t, db.NewRaw(`SELECT changes FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'export' AND resource_type = 'school' AND resource_id = ? ORDER BY id`, operator.ID, school).Scan(ctx, &entries))
	require.Len(t, entries, 2, "the attempt and the outcome")
	var started struct {
		Outcome  string `json:"outcome"`
		SchoolID int64  `json:"school_id"`
	}
	require.NoError(t, json.Unmarshal(entries[0].Changes, &started))
	require.Equal(t, "started", started.Outcome)
	require.Equal(t, school, started.SchoolID)
	var record organizationtenancy.TenantExportRecord
	require.NoError(t, json.Unmarshal(entries[1].Changes, &record))
	marker, err := os.ReadFile(filepath.Join(out, "COMPLETE"))
	require.NoError(t, err)
	require.Equal(t, organizationtenancy.TenantExportCompleted, record.Outcome)
	require.Equal(t, school, record.SchoolID)
	require.Zero(t, record.OrganizationID)
	require.Equal(t, strings.TrimSpace(string(marker)), record.ManifestSHA256)
	require.Positive(t, record.Tables)
	for _, entry := range entries {
		require.NotContains(t, string(entry.Changes), name, "the audit holds no personal data")
		require.NotContains(t, string(entry.Changes), out, "nor the local path")
	}
}

// Like the Träger package, the school package needs an existing, active
// operator before anything is read: an unknown or deactivated one gets no
// file and no row (#6326).
func TestTenantExportSchoolRefusesAnUnknownOrInactiveOperator(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := tenant.WithUnitOfWork(context.Background(), database.UnitOfWork())
	school := testpkg.Tenant(t)
	inactive := testpkg.CreateTestOperator(t, db)
	_, err := db.ExecContext(ctx, `UPDATE platform.operators SET active = false, deactivated_at = now() WHERE id = ?`, inactive.ID)
	require.NoError(t, err)
	exporter, err := newTenantExporter(database, returnSealedFields(t), tenantExportProjectedFields(), false)
	require.NoError(t, err)
	for name, operator := range map[string]int64{"inactive": inactive.ID, "unknown": 1 << 60} {
		out := filepath.Join(t.TempDir(), "export")
		require.Error(t, exporter.Export(ctx, organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: operator, Output: out, Uploads: t.TempDir()}), name)
		require.NoDirExists(t, out, name)
		var rows int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
			WHERE action = 'export' AND resource_type = 'school' AND resource_id = ?`, school).Scan(ctx, &rows))
		require.Zero(t, rows, name)
	}
}

// The command refuses a school export without --operator before it opens
// the database (#6326).
func TestTenantExportSchoolRequiresAnOperator(t *testing.T) {
	t.Parallel()
	err := runTenantExport(nil, organizationtenancy.TenantExportOptions{SchoolID: testpkg.UniqueTestTenantID(t), Output: "out", Uploads: "uploads"})
	require.ErrorContains(t, err, "--operator is required")
}
