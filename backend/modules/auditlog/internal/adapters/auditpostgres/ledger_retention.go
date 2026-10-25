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

// operatorAuditLogCutoffSQL is 1 January of last year in Berlin. It reads
// the new year a minute late so that the database clock remains the one
// source of time for ledger rows and their retention.
const operatorAuditLogCutoffSQL = `(
	(date_trunc('year', (statement_timestamp() AT TIME ZONE 'Europe/Berlin') - INTERVAL '1 minute') - INTERVAL '1 year')
	AT TIME ZONE 'Europe/Berlin'
)`

// DeleteExpiredOperatorAuditLog removes platform-wide operator-ledger
// entries before the database-clock cutoff (#4047), and with the same cutoff
// the entries of the Träger account ledger (#4650), the other ledger without
// a school. Neither has a school, so the call needs the administrative
// transaction of the hourly cleanup, not a tenant.
func (a *Appender) DeleteExpiredOperatorAuditLog(ctx context.Context) (int64, error) {
	const operation = "delete expired operator audit log"
	db, _, err := database(ctx, a.runtime)
	if err != nil {
		return 0, wrapDatabase(operation, err)
	}
	operator, err := db.NewDelete().
		TableExpr("platform.operator_audit_log").
		Where("created_at < " + operatorAuditLogCutoffSQL).
		Exec(ctx)
	if err != nil {
		return 0, wrapDatabase(operation, err)
	}
	traeger, err := db.NewDelete().
		TableExpr("audit.traeger_account_changes").
		Where("created_at < " + operatorAuditLogCutoffSQL).
		Exec(ctx)
	if err != nil {
		return 0, wrapDatabase(operation, err)
	}
	operatorRows, err := operator.RowsAffected()
	if err != nil {
		return 0, wrapDatabase(operation, err)
	}
	traegerRows, err := traeger.RowsAffected()
	return operatorRows + traegerRows, wrapDatabase(operation, err)
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

// deleteExpiredProtocolSQL deletes the rows of one protocol of #4044
// through the tenant-bound function of migration 1.15.454, which names each
// protocol's table and refuses a cutoff inside its fixed window.
const deleteExpiredProtocolSQL = `SELECT audit.delete_expired_protocol(?, ?, ?)`

// DeleteExpiredProtocol removes the school's rows of the protocol of kind
// before cutoff; the function refuses a kind it does not name.
func (a *Appender) DeleteExpiredProtocol(ctx context.Context, kind string, cutoff time.Time) (int64, error) {
	operation := "delete expired " + kind
	db, tenantID, err := a.retentionTenant(ctx, operation)
	if err != nil {
		return 0, err
	}
	var deleted int64
	if err := db.NewRaw(deleteExpiredProtocolSQL, tenantID, kind, cutoff).Scan(ctx, &deleted); err != nil {
		return 0, wrapDatabase(operation, err)
	}
	return deleted, nil
}
