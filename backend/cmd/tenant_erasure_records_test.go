package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	tenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/erasureledger"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// An erasure order recorded before migration 1.15.875 has neither the hint
// on the records nor the customer's choice (#4720, AVV Ziff. 8.4). The
// erasure procedure refuses it and names the missing step; once the operator
// recorded both, the same order erases the Träger.
func TestErasureCLIOrganizationRefusesAnOrderWithoutTheRecordsChoice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	school := testpkg.Tenant(t)
	var organizationID int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(ctx, &organizationID))
	operator := testpkg.CreateTestOperator(t, db)
	_, err := db.ExecContext(ctx, `UPDATE platform.organizations SET active = false WHERE id = ?`, organizationID)
	require.NoError(t, err)
	// A legacy order: due, without records_notice_shown_at and records_choice.
	_, err = db.ExecContext(ctx, `INSERT INTO platform.organization_erasure_orders (organization_id, order_reference, confirmed_on, erase_not_before, recorded_by)
		VALUES (?, 'ORDER-LEGACY', '2026-01-02', '2026-01-03', ?)`, organizationID, operator.ID)
	require.NoError(t, err)
	service, err := tenancyCompose.NewErasure(db, objects.NewLocal(t.TempDir()), erasureledger.New(newTestErasureLedger(t), securityruntime.Fingerprint))
	require.NoError(t, err)
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		require.NoError(t, db.NewRaw(query, args...).Scan(ctx, &n))
		return n
	}
	request := organizationtenancy.ErasureRequest{OrganizationID: organizationID, OperatorID: operator.ID, OrderReference: "ORDER-LEGACY",
		Confirmation: fmt.Sprintf("organization:%d", organizationID), PrerequisitesChecked: true, WritersStopped: true,
		LockTimeout: time.Second, StatementTimeout: time.Minute, DryRun: true}

	results, err := service.Run(ctx, request)
	require.NoError(t, err)
	require.Contains(t, string(results[len(results)-1]), `"status": "records_choice_missing"`, "the dry run names the missing step")
	request.DryRun = false
	_, err = service.Run(ctx, request)
	require.ErrorContains(t, err, "records_choice_missing", "an order without the hint and the choice is not executed")
	require.Equal(t, 1, count(`SELECT count(*) FROM platform.schools WHERE organization_id = ?`, organizationID), "no school is erased")
	require.Zero(t, count(`SELECT count(*) FROM platform.organization_erasure_orders WHERE organization_id = ? AND started_at IS NOT NULL`, organizationID),
		"the refused order is not started, so the operator can still withdraw it")

	// The SQL functions refuse it on their own, not only through the CLI.
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `SET ROLE phoenix_school_erasure`)
	require.NoError(t, err)
	_, beginErr := conn.ExecContext(ctx, `SELECT platform.begin_organization_erasure(?, ?, 'ORDER-LEGACY')`, organizationID, operator.ID)
	_, resetErr := conn.ExecContext(ctx, `RESET ROLE`)
	require.NoError(t, resetErr)
	require.NoError(t, conn.Close())
	require.ErrorContains(t, beginErr, "records")

	_, err = db.ExecContext(ctx, `UPDATE platform.organization_erasure_orders
		SET records_notice_shown_at = clock_timestamp(), records_choice = 'erasure_chosen' WHERE organization_id = ?`, organizationID)
	require.NoError(t, err)
	_, err = service.Run(ctx, request)
	require.NoError(t, err, "with the hint and the choice recorded the order is executed")
	require.Zero(t, count(`SELECT count(*) FROM platform.organizations WHERE id = ?`, organizationID), "the Träger is erased")
}

// An order that was already started before migration 1.15.875 has no choice
// either (#4720). It is not resumed until the operator records the hint and
// the customer's choice on it (it can no longer be withdrawn); then the same
// run resumes and erases the Träger.
func TestErasureCLIOrganizationDoesNotResumeAStartedOrderWithoutTheRecordsChoice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	school := testpkg.Tenant(t)
	var organizationID int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(ctx, &organizationID))
	operator := testpkg.CreateTestOperator(t, db)
	_, err := db.ExecContext(ctx, `UPDATE platform.organizations SET active = false WHERE id = ?`, organizationID)
	require.NoError(t, err)
	// A legacy order the erasure CLI started before the migration.
	_, err = db.ExecContext(ctx, `INSERT INTO platform.organization_erasure_orders (organization_id, order_reference, confirmed_on, erase_not_before, recorded_by, started_at)
		VALUES (?, 'ORDER-STARTED', '2026-01-02', '2026-01-03', ?, clock_timestamp())`, organizationID, operator.ID)
	require.NoError(t, err)
	service, err := tenancyCompose.NewErasure(db, objects.NewLocal(t.TempDir()), erasureledger.New(newTestErasureLedger(t), securityruntime.Fingerprint))
	require.NoError(t, err)
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		require.NoError(t, db.NewRaw(query, args...).Scan(ctx, &n))
		return n
	}
	request := organizationtenancy.ErasureRequest{OrganizationID: organizationID, OperatorID: operator.ID, OrderReference: "ORDER-STARTED",
		Confirmation: fmt.Sprintf("organization:%d", organizationID), PrerequisitesChecked: true, WritersStopped: true,
		LockTimeout: time.Second, StatementTimeout: time.Minute, DryRun: true}

	results, err := service.Run(ctx, request)
	require.NoError(t, err)
	require.Contains(t, string(results[len(results)-1]), `"status": "records_choice_missing"`, "the dry run names the missing step")
	request.DryRun = false
	_, err = service.Run(ctx, request)
	require.ErrorContains(t, err, "records_choice_missing", "a started order without the choice is not resumed")
	require.ErrorContains(t, err, "operator portal", "the refusal says what to do")
	require.Equal(t, 1, count(`SELECT count(*) FROM platform.schools WHERE organization_id = ?`, organizationID), "no school is erased")

	// The SQL functions refuse to resume it on their own.
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `SET ROLE phoenix_school_erasure`)
	require.NoError(t, err)
	_, beginErr := conn.ExecContext(ctx, `SELECT platform.begin_organization_erasure(?, ?, 'ORDER-STARTED')`, organizationID, operator.ID)
	_, resetErr := conn.ExecContext(ctx, `RESET ROLE`)
	require.NoError(t, resetErr)
	require.NoError(t, conn.Close())
	require.ErrorContains(t, beginErr, "records")

	_, err = db.ExecContext(ctx, `UPDATE platform.organization_erasure_orders
		SET records_notice_shown_at = clock_timestamp(), records_choice = 'erasure_chosen' WHERE organization_id = ?`, organizationID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE platform.organization_erasure_orders SET records_choice = 'handed_over', records_handed_over_on = '2026-01-02'
		WHERE organization_id = ?`, organizationID)
	require.ErrorContains(t, err, "records", "a recorded choice is evidence and stays")
	_, err = service.Run(ctx, request)
	require.NoError(t, err, "with the choice recorded the started order resumes")
	require.Zero(t, count(`SELECT count(*) FROM platform.organizations WHERE id = ?`, organizationID), "the Träger is erased")
}
