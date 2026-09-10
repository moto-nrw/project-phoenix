package cmd

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateBackfillRequestChildStorageCmd_Metadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "backfill", migrateBackfillCmd.Use)
	assert.Equal(t, "request-child-storage", migrateBackfillRequestChildStorageCmd.Use)
	assert.NotNil(t, migrateBackfillRequestChildStorageCmd.RunE)
	assert.Contains(t, migrateCmd.Commands(), migrateBackfillCmd)
	for _, flag := range []string{"batch-size", "tenant", "max-passes", "lock-timeout", "verify-only", "restart"} {
		assert.NotNil(t, migrateBackfillRequestChildStorageCmd.Flags().Lookup(flag), flag)
	}
}

func TestMigrateBackfillRequestChildStorageFlags(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "request-child-storage"}
	registerRequestChildStorageFlags(cmd)
	require.NoError(t, cmd.Flags().Parse([]string{"--batch-size=7", "--tenant=3,4", "--max-passes=2", "--lock-timeout=250ms", "--verify-only", "--restart"}))
	options, err := requestChildStorageOptionsFromFlags(cmd)
	require.NoError(t, err)
	assert.Equal(t, migrations.RequestChildStorageBackfillOptions{
		BatchSize: 7, TenantIDs: []int64{3, 4}, MaxPasses: 2, LockTimeout: 250 * time.Millisecond, VerifyOnly: true, Restart: true,
	}, options)
}

func TestMigrateBackfillRequestChildStorageReportsPerTenantEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenant := testpkg.Tenant(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "CLI backfill")
	_, err := db.NewRaw(`INSERT INTO enrollment.request_child_offerings
		(tenant_id, request_child_id, care_offering_id, selected_days, notes, valid_from, valid_until)
		VALUES (?, ?, ?, '["mon"]', 'cli', '2026-08-01', '2027-08-01')`, tenant, childID, offering.ID).Exec(t.Context())
	require.NoError(t, err)

	var output bytes.Buffer
	options := migrations.RequestChildStorageBackfillOptions{TenantIDs: []int64{tenant}}
	require.ErrorContains(t, runRequestChildStorageBackfill(t.Context(), db, options, &output), "incomplete for tenants")
	assert.Regexp(t, `tenant +complete +hwm +scanned`, output.String())
	assert.Regexp(t, strconv.FormatInt(tenant, 10)+` +false`, output.String())
	assert.Contains(t, output.String(), "unresolved_origins=1")
	assert.Contains(t, output.String(), "missing-authoritative-submission")

	// Drift makes verify-only fail with a non-zero exit and an evidence row.
	_, err = db.NewRaw(`UPDATE enrollment.request_child_offerings SET notes = 'changed' WHERE tenant_id = ?`, tenant).Exec(t.Context())
	require.NoError(t, err)
	output.Reset()
	options.VerifyOnly = true
	err = runRequestChildStorageBackfill(t.Context(), db, options, &output)
	require.ErrorContains(t, err, "incomplete for tenants")
	assert.Regexp(t, strconv.FormatInt(tenant, 10)+` +false`, output.String())
}
