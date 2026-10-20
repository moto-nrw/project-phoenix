package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auditLedgerRetention struct {
	name             string
	function         string
	table            string
	allowedCutoffSQL string
	// seedSQL inserts one row of the ledger for the tenant (and the account,
	// when the ledger names one) dated by the SQL expression it is formatted
	// with.
	seedSQL     string
	usesAccount bool
	expiredSQL  string
	freshSQL    string
}

const lastYearStartSQL = `((date_trunc('year', CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin') - INTERVAL '1 year') AT TIME ZONE 'Europe/Berlin')`

var auditLedgerRetentions = []auditLedgerRetention{
	{
		name:             "auth events after 90 days",
		function:         "audit.delete_expired_auth_events",
		table:            "audit.auth_events",
		allowedCutoffSQL: "CURRENT_TIMESTAMP - INTERVAL '90 days'",
		seedSQL: `INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address, created_at)
			VALUES (?, ?, 'login', true, '127.0.0.1', %s) RETURNING id`,
		usesAccount: true,
		expiredSQL:  "CURRENT_TIMESTAMP - INTERVAL '91 days'",
		freshSQL:    "CURRENT_TIMESTAMP - INTERVAL '89 days'",
	},
	{
		name:             "data access log after the end of the following year",
		function:         "audit.delete_expired_data_access_log",
		table:            "audit.data_access_log",
		allowedCutoffSQL: lastYearStartSQL,
		seedSQL: `INSERT INTO audit.data_access_log (tenant_id, actor_account_id, actor_role, resource_type, range_start, range_end, accessed_at)
			VALUES (?, ?, 'staff', 'attendance', %[1]s, %[1]s, %[1]s) RETURNING id`,
		usesAccount: true,
		expiredSQL:  lastYearStartSQL + " - INTERVAL '1 day'",
		freshSQL:    lastYearStartSQL + " + INTERVAL '1 day'",
	},
	{
		name:             "deletion log after three years",
		function:         "audit.delete_expired_data_deletions",
		table:            "audit.data_deletions",
		allowedCutoffSQL: "CURRENT_TIMESTAMP - INTERVAL '3 years'",
		seedSQL: `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by, deleted_at)
			VALUES (?, 'auth_event_retention', 1, 'system', %s) RETURNING id`,
		expiredSQL: "CURRENT_TIMESTAMP - INTERVAL '3 years 1 day'",
		freshSQL:   "CURRENT_TIMESTAMP - INTERVAL '3 years' + INTERVAL '1 day'",
	},
}

func seedLedgerRow(t *testing.T, db *testpkg.DB, ledger auditLedgerRetention, at string, tenantID, accountID int64) int64 {
	t.Helper()
	args := []any{tenantID}
	if ledger.usesAccount {
		args = append(args, accountID)
	}
	var id int64
	require.NoError(t, db.NewRaw(fmt.Sprintf(ledger.seedSQL, at), args...).Scan(context.Background(), &id))
	return id
}

func TestAuditLedgerRetentionRejectsForeignSchoolsAndYoungCutoffs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	account := testpkg.CreateTestAccount(t, db, "ledger-retention@example.com").ID

	for _, ledger := range auditLedgerRetentions {
		t.Run(ledger.name, func(t *testing.T) {
			tenantA, _ := testpkg.CreateTestTenant(t, db)
			tenantB, _ := testpkg.CreateTestTenant(t, db)
			rowA := seedLedgerRow(t, db, ledger, ledger.expiredSQL, tenantA, account)
			rowB := seedLedgerRow(t, db, ledger, ledger.expiredSQL, tenantB, account)

			err := callAuditRetentionCapability(t, authDB, tenantA, ledger.function, tenantB, ledger.allowedCutoffSQL)
			require.ErrorContains(t, err, "SQLSTATE=42501", "a school never deletes another school's rows")
			err = callAuditRetentionCapability(t, authDB, tenantA, ledger.function, tenantA, "CURRENT_TIMESTAMP")
			require.ErrorContains(t, err, "SQLSTATE=42501", "nothing younger than the fixed window is deleted")
			assertAuditRowsExist(t, db, ledger.table, rowA, rowB)
		})
	}
}

func TestAuditLedgerRetentionDeletesOnlyExpiredRowsOfTheOwnSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	account := testpkg.CreateTestAccount(t, db, "ledger-retention-own@example.com").ID

	for _, ledger := range auditLedgerRetentions {
		t.Run(ledger.name, func(t *testing.T) {
			tenantA, _ := testpkg.CreateTestTenant(t, db)
			tenantB, _ := testpkg.CreateTestTenant(t, db)
			expiredA := seedLedgerRow(t, db, ledger, ledger.expiredSQL, tenantA, account)
			freshA := seedLedgerRow(t, db, ledger, ledger.freshSQL, tenantA, account)
			expiredB := seedLedgerRow(t, db, ledger, ledger.expiredSQL, tenantB, account)

			var deleted int64
			err := withPhoenixTenantTx(t, authDB, tenantA, func(ctx context.Context, tx testpkg.Tx) error {
				return tx.NewRaw(fmt.Sprintf("SELECT %s(?, %s)", ledger.function, ledger.allowedCutoffSQL), tenantA).Scan(ctx, &deleted)
			})
			require.NoError(t, err)
			assert.EqualValues(t, 1, deleted)
			assertAuditRowsMissing(t, db, ledger.table, expiredA)
			assertAuditRowsExist(t, db, ledger.table, freshA, expiredB)
		})
	}
}

func TestAuditLedgersStayClosedToDirectDeletes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	for _, table := range []string{"audit.auth_events", "audit.data_access_log", "audit.data_deletions"} {
		var canDelete bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, 'DELETE')`, table).Scan(context.Background(), &canDelete))
		assert.False(t, canDelete, "%s stays append-only for the tenant role", table)
	}
}
