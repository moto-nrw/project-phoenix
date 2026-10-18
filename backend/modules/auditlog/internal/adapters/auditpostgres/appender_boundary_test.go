package audit

import (
	"context"
	"testing"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAuditAppenderRequiresAuthoritativeTransaction(t *testing.T) {
	t.Parallel()

	appender := NewAppender(func(context.Context) (bun.IDB, int64) { return nil, 1 })
	event := audit.NewAuthEvent(1, audit.EventTypeLogin, true, "192.0.2.1")
	err := appender.Append(context.Background(), event)
	require.ErrorContains(t, err, "transaction is required")
}

func TestAuditAppenderAttributesTenantAndDatabaseEnforcesAppendOnlyRLS(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ownTenant := testpkg.Tenant(t)
	otherTenant := testpkg.NewTenantScope(t, db).TenantID
	account := testpkg.CreateTestAccount(t, db, "audit-command-boundary@example.test")
	event := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.0.2.2")

	err := testpkg.WithTenantTx(t, context.Background(), db, ownTenant, func(txCtx context.Context, tx bun.Tx) error {
		appender := NewAppender(func(context.Context) (bun.IDB, int64) { return tx, ownTenant })
		return appender.Append(txCtx, event)
	})
	require.NoError(t, err)
	assert.Equal(t, ownTenant, event.TenantID)
	assert.Positive(t, event.ID)
	assert.Equal(t, account.ID, event.AccountID)

	var mismatchErr error
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, ownTenant, func(txCtx context.Context, tx bun.Tx) error {
		mismatched := audit.NewAuthEvent(account.ID, audit.EventTypeLogin, true, "192.0.2.3")
		mismatched.SetTenantID(otherTenant)
		mismatchErr = NewAppender(func(context.Context) (bun.IDB, int64) { return tx, ownTenant }).Append(txCtx, mismatched)
		return nil
	}))
	require.ErrorContains(t, mismatchErr, "does not match transaction tenant")

	assertAuditTenantWriteDenied(t, db, ownTenant, "permission denied", `UPDATE audit.auth_events SET success = FALSE WHERE id = ?`, event.ID)
	assertAuditTenantWriteDenied(t, db, ownTenant, "permission denied", `DELETE FROM audit.auth_events WHERE id = ?`, event.ID)
	assertAuditTenantWriteDenied(t, db, ownTenant, "row-level security policy", `
		INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address)
		VALUES (?, ?, 'login', TRUE, '192.0.2.4')`, otherTenant, account.ID)
}

func TestAuditOwnedAppendViewsRejectMutation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)

	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `UPDATE audit.file_event_ledger SET detail = detail WHERE FALSE`)
	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `DELETE FROM audit.file_event_ledger WHERE FALSE`)
	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `UPDATE audit.guardian_financial_change_ledger SET note = note WHERE FALSE`)
	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `DELETE FROM audit.guardian_financial_change_ledger WHERE FALSE`)
}

func assertAuditTenantWriteDenied(t *testing.T, db *bun.DB, tenantID int64, expected, query string, args ...any) {
	t.Helper()
	err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(query, args...).Exec(ctx)
		return err
	})
	require.Error(t, err)
	require.ErrorContains(t, err, expected)
}
