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
