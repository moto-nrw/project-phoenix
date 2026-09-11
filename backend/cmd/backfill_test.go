package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/migrations"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackfillCommandMetadata(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "backfill", backfillCmd.Use)
	assert.Nil(t, backfillCmd.RunE, "backfill only groups the named backfills")
	assert.Equal(t, "staff-owner", backfillStaffOwnerCmd.Use)
	assert.NotNil(t, backfillStaffOwnerCmd.RunE)
	assert.NotNil(t, backfillStaffOwnerCmd.Flags().Lookup(flagBackfillBatchSize))
	assert.NotNil(t, backfillStaffOwnerCmd.Flags().Lookup(flagBackfillMaxPasses))
	assert.Equal(t, "status", backfillStaffOwnerStatusCmd.Use)
	assert.Equal(t, "reset", backfillStaffOwnerResetCmd.Use)
	assert.Contains(t, backfillStaffOwnerResetCmd.Long, "users.staff is never modified")
	require.True(t, backfillCmd.HasSubCommands())
	for _, sub := range []*cobra.Command{backfillStaffOwnerStatusCmd, backfillStaffOwnerResetCmd} {
		assert.Equal(t, backfillStaffOwnerCmd, sub.Parent())
	}
	assert.Equal(t, RootCmd, backfillCmd.Parent())
}

func TestBackfillRootFailsFastWithoutDatabaseDependency(t *testing.T) {
	t.Parallel()
	err := (backfillRoot{}).run(t.Context(), func(context.Context, *testpkg.DB) error { return nil })
	require.ErrorContains(t, err, "database opener is required")
	err = backfillRoot{openDatabase: func() (*testpkg.DB, func(), error) { return nil, nil, nil }}.run(t.Context(), func(context.Context, *testpkg.DB) error { return nil })
	require.ErrorContains(t, err, "database opener returned nil")
}

// The backfill visits every school in the database, so the test owns an
// isolated clone instead of the package's shared one.
func TestBackfillStaffOwnerCommands(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Backfill", "Command")
	released := 0
	root := backfillRoot{openDatabase: func() (*testpkg.DB, func(), error) {
		return db, func() { released++ }, nil
	}}
	newCommand := func() (*cobra.Command, *bytes.Buffer) {
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetContext(ctx)
		cmd.SetOut(&out)
		return cmd, &out
	}

	cmd, out := newCommand()
	require.NoError(t, root.staffOwnerRun(cmd, migrations.StaffOwnerBackfillOptions{BatchSize: 1}))
	require.Equal(t, 1, released, "the database is released after every command")
	require.Contains(t, out.String(), "staff owner backfill stable")
	report := decodeStaffOwnerReport(t, out.String())
	require.True(t, report.Stable())
	var found bool
	for _, tenant := range report.Tenants {
		if tenant.TenantID == tenantID {
			found = true
			require.EqualValues(t, 1, tenant.RowsCopied)
			require.EqualValues(t, 1, tenant.SourceCount)
		}
	}
	require.True(t, found)

	cmd, out = newCommand()
	require.NoError(t, root.staffOwnerStatus(cmd))
	require.Equal(t, 2, released)
	require.Equal(t, len(report.Tenants), len(decodeStaffOwnerReport(t, out.String()).Tenants))

	// A cross-tenant work-time model keeps the tenant unstable; the command
	// prints the evidence and still fails so scripts cannot proceed to Cutover.
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	var foreignModel int64
	require.NoError(t, db.NewRaw(`INSERT INTO config.work_time_models (tenant_id, name, rotation_anchor_date) VALUES (?, 'Foreign', '2026-09-01') RETURNING id`, otherTenant).Scan(ctx, &foreignModel))
	_, err := db.ExecContext(ctx, `UPDATE users.staff SET work_time_model_id = ? WHERE id = ?`, foreignModel, staff.ID)
	require.NoError(t, err)
	cmd, out = newCommand()
	err = root.staffOwnerRun(cmd, migrations.StaffOwnerBackfillOptions{MaxPasses: 1})
	require.ErrorContains(t, err, "not stable")
	require.Contains(t, out.String(), `"rows_rejected": 1`)
	cmd, _ = newCommand()
	require.ErrorContains(t, root.staffOwnerStatus(cmd), "not stable")

	cmd, out = newCommand()
	require.NoError(t, root.staffOwnerReset(cmd))
	require.Contains(t, out.String(), "users.staff untouched")
	var memberships int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships`).Scan(ctx, &memberships))
	require.Zero(t, memberships)
	cmd, out = newCommand()
	require.ErrorContains(t, root.staffOwnerStatus(cmd), "not stable", "unvisited schools block Cutover")
	report = decodeStaffOwnerReport(t, out.String())
	require.Empty(t, report.Tenants)
	require.Contains(t, report.MissingTenants, tenantID)
	var sourceRows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff WHERE id = ?`, staff.ID).Scan(ctx, &sourceRows))
	require.Equal(t, 1, sourceRows)
}

func decodeStaffOwnerReport(t *testing.T, output string) migrations.StaffOwnerBackfillReport {
	t.Helper()
	// The JSON document ends at the first line that is not part of it.
	end := strings.Index(output, "\n}\n")
	require.GreaterOrEqual(t, end, 0, "report output must be a JSON object: %s", output)
	var report migrations.StaffOwnerBackfillReport
	require.NoError(t, json.Unmarshal([]byte(output[:end+2]), &report))
	return report
}

func TestBackfillStaffOwnerReportsPartialFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	tenantA := testpkg.Tenant(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	failedTenant, healthyTenant := min(tenantA, tenantB), max(tenantA, tenantB)
	testpkg.CreateTestStaffForTenant(t, db, failedTenant, "Backfill", "Failed")
	testpkg.CreateTestStaffForTenant(t, db, healthyTenant, "Backfill", "Healthy")
	_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE FUNCTION public.reject_backfill_fixture() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture batch failure' USING ERRCODE = '22023'; END $$;
		CREATE TRIGGER reject_backfill_fixture BEFORE INSERT OR UPDATE ON users.staff_school_memberships
		FOR EACH ROW WHEN (NEW.tenant_id = %d) EXECUTE FUNCTION public.reject_backfill_fixture();`, failedTenant))
	require.NoError(t, err)
	root := backfillRoot{openDatabase: func() (*testpkg.DB, func(), error) { return db, nil, nil }}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.SetOut(&out)
	err = root.staffOwnerRun(cmd, migrations.StaffOwnerBackfillOptions{})
	require.ErrorContains(t, err, "22023")
	report := decodeStaffOwnerReport(t, out.String())
	require.Equal(t, []int64{failedTenant}, report.Unstable())
	var healthy bool
	for _, cp := range report.Tenants {
		if cp.TenantID == healthyTenant {
			healthy = cp.Stable && cp.RowsCopied == 1
		}
	}
	require.True(t, healthy, "the later school's successful checkpoint must be rendered")
	outputErr := errors.New("fixture output failure")
	cmd.SetOut(backfillFailingWriter{err: outputErr})
	err = root.staffOwnerRun(cmd, migrations.StaffOwnerBackfillOptions{})
	require.ErrorContains(t, err, "22023")
	require.ErrorIs(t, err, outputErr, "preserve both operation and output errors")
}

type backfillFailingWriter struct{ err error }

func (w backfillFailingWriter) Write([]byte) (int, error) { return 0, w.err }
