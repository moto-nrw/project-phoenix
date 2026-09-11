package test

import (
	"context"
	"testing"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAuditAppenderRequiresAuthoritativeTransaction(t *testing.T) {
	t.Parallel()

	appender := auditRepo.NewAppender(func(context.Context) (bun.IDB, int64) { return nil, 1 })
	event := NewAuditAuthEvent(1, "192.0.2.1")
	err := appender.Append(context.Background(), event)
	require.ErrorContains(t, err, "transaction is required")
}

func TestAuditAppenderAttributesTenantAndDatabaseEnforcesAppendOnlyRLS(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	ownTenant := Tenant(t)
	otherTenant := NewTenantScope(t, db).TenantID
	account := CreateTestAccount(t, db, "audit-command-boundary@example.test")
	event := NewAuditAuthEvent(account.ID, "192.0.2.2")

	err := WithTenantTx(t, context.Background(), db, ownTenant, func(txCtx context.Context, tx bun.Tx) error {
		appender := auditRepo.NewAppender(func(context.Context) (bun.IDB, int64) { return tx, ownTenant })
		return appender.Append(txCtx, event)
	})
	require.NoError(t, err)
	eventTenant, eventID, eventAccount := AuditEventIdentity(event)
	assert.Equal(t, ownTenant, eventTenant)
	assert.Positive(t, eventID)
	assert.Equal(t, account.ID, eventAccount)

	var mismatchErr error
	require.NoError(t, WithTenantTx(t, context.Background(), db, ownTenant, func(txCtx context.Context, tx bun.Tx) error {
		mismatched := NewAuditAuthEvent(account.ID, "192.0.2.3")
		SetAuditEventTenant(mismatched, otherTenant)
		mismatchErr = auditRepo.NewAppender(func(context.Context) (bun.IDB, int64) { return tx, ownTenant }).Append(txCtx, mismatched)
		return nil
	}))
	require.ErrorContains(t, mismatchErr, "does not match transaction tenant")

	assertAuditTenantWriteDenied(t, db, ownTenant, "permission denied", `UPDATE audit.auth_events SET success = FALSE WHERE id = ?`, eventID)
	assertAuditTenantWriteDenied(t, db, ownTenant, "permission denied", `DELETE FROM audit.auth_events WHERE id = ?`, eventID)
	assertAuditTenantWriteDenied(t, db, ownTenant, "row-level security policy", `
		INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address)
		VALUES (?, ?, 'login', TRUE, '192.0.2.4')`, otherTenant, account.ID)
}

func TestAuditOwnedAppendViewsRejectMutation(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantID := Tenant(t)

	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `UPDATE audit.file_event_ledger SET detail = detail WHERE FALSE`)
	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `DELETE FROM audit.file_event_ledger WHERE FALSE`)
	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `UPDATE audit.guardian_financial_change_ledger SET note = note WHERE FALSE`)
	assertAuditTenantWriteDenied(t, db, tenantID, "permission denied", `DELETE FROM audit.guardian_financial_change_ledger WHERE FALSE`)
}

func assertAuditTenantWriteDenied(t *testing.T, db *bun.DB, tenantID int64, expected, query string, args ...any) {
	t.Helper()
	err := WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(query, args...).Exec(ctx)
		return err
	})
	require.Error(t, err)
	require.ErrorContains(t, err, expected)
}
