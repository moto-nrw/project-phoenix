package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// The append-only ledgers of #3956 lose their expired rows only through the
// fixed, tenant-bound database functions of migration 1.15.447: the tenant
// role holds no DELETE on them. The Appender carries the calls because it
// already binds the Audit runtime the nightly cleanup runs in.

const (
	deleteExpiredAuthEventsSQL    = `SELECT audit.delete_expired_auth_events(?, ?)`
	deleteExpiredDataAccessLogSQL = `SELECT audit.delete_expired_data_access_log(?, ?)`
	deleteExpiredDataDeletionsSQL = `SELECT audit.delete_expired_data_deletions(?, ?)`
)

// DeleteExpiredAuthEvents removes the school's sign-in events before cutoff.
func (a *Appender) DeleteExpiredAuthEvents(ctx context.Context, cutoff time.Time) (int64, error) {
	const operation = "delete expired auth events"
	db, tenantID, err := a.retentionTenant(ctx, operation)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(deleteExpiredAuthEventsSQL, tenantID, cutoff).Scan(ctx, &deleted); err != nil {
		return 0, wrapDatabase(operation, err)
	}
	return deleted, nil
}

// DeleteExpiredDataAccessLog removes the school's access-log rows before
// cutoff.
func (a *Appender) DeleteExpiredDataAccessLog(ctx context.Context, cutoff time.Time) (int64, error) {
	const operation = "delete expired data access log"
	db, tenantID, err := a.retentionTenant(ctx, operation)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(deleteExpiredDataAccessLogSQL, tenantID, cutoff).Scan(ctx, &deleted); err != nil {
		return 0, wrapDatabase(operation, err)
	}
	return deleted, nil
}

// DeleteExpiredDataDeletions removes the school's deletion-log rows before
// cutoff.
func (a *Appender) DeleteExpiredDataDeletions(ctx context.Context, cutoff time.Time) (int64, error) {
	const operation = "delete expired deletion log"
	db, tenantID, err := a.retentionTenant(ctx, operation)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(deleteExpiredDataDeletionsSQL, tenantID, cutoff).Scan(ctx, &deleted); err != nil {
		return 0, wrapDatabase(operation, err)
	}
	return deleted, nil
}

// DeleteExpiredOperatorAuditLog removes the platform-wide operator-ledger
// entries before cutoff (#4047). The ledger has no school, so the call needs
// the administrative transaction of the hourly cleanup, not a tenant.
func (a *Appender) DeleteExpiredOperatorAuditLog(ctx context.Context, cutoff time.Time) (int64, error) {
	const operation = "delete expired operator audit log"
	db, _, err := database(ctx, a.runtime)
	if err != nil {
		return 0, wrapDatabase(operation, err)
	}
	result, err := db.NewDelete().
		TableExpr("platform.operator_audit_log").
		Where("created_at < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, wrapDatabase(operation, err)
	}
	deleted, err := result.RowsAffected()
	return deleted, wrapDatabase(operation, err)
}

// retentionTenant resolves the transaction and the school a retention call
// runs for; the database function refuses any other school.
func (a *Appender) retentionTenant(ctx context.Context, operation string) (bun.IDB, int64, error) {
	db, tenantID, err := database(ctx, a.runtime)
	if err != nil {
		return nil, 0, wrapDatabase(operation, err)
	}
	if tenantID <= 0 {
		return nil, 0, wrapDatabase(operation, fmt.Errorf("tenant context is required"))
	}
	return db, tenantID, nil
}
