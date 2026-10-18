package audit

import (
	"context"
	"fmt"
	"slices"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// LogRetention records in audit.data_deletions that a nightly retention step
// deleted deleted rows of one data kind for the school of ctx (#3950). The
// entry names no person and holds no deleted content; kind is one of the
// data-kind deletion types of the Audit records.
func (c *Command) LogRetention(ctx context.Context, kind string, deleted, retentionDays int) error {
	if !auditModels.IsDataKindRetention(kind) {
		return fmt.Errorf("audit retention: unknown data kind %q", kind)
	}
	entry := auditModels.NewRetentionDataDeletion(kind, deleted)
	entry.SetMetadata("retention_days", retentionDays)
	return c.Append(ctx, entry)
}

// Fixed retention of the append-only audit ledgers for every school (#3956,
// ADR 0048). Migration 1.15.447 enforces the same windows in the database.
const (
	AuthEventRetentionDays    = 90
	DeletionLogRetentionYears = 3
)

// ledgerRetentionStore is the Audit append store's access to the
// tenant-bound retention functions of the ledgers.
type ledgerRetentionStore interface {
	DeleteExpiredAuthEvents(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteExpiredDataAccessLog(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteExpiredDataDeletions(ctx context.Context, cutoff time.Time) (int64, error)
	DeleteExpiredProtocol(ctx context.Context, kind string, cutoff time.Time) (int64, error)
}

func (c *Command) ledgerRetention() (ledgerRetentionStore, error) {
	store, ok := c.store.(ledgerRetentionStore)
	if !ok {
		return nil, fmt.Errorf("audit command: ledger retention store is required")
	}
	return store, nil
}

// DeleteExpiredAuthEvents removes the school's sign-in events older than 90
// days and reports how many went and the window it applied.
func (c *Command) DeleteExpiredAuthEvents(ctx context.Context) (int, int, error) {
	store, err := c.ledgerRetention()
	if err != nil {
		return 0, 0, err
	}
	cutoff := time.Now().AddDate(0, 0, -AuthEventRetentionDays).Add(-auditModels.RetentionClockMargin)
	deleted, err := store.DeleteExpiredAuthEvents(ctx, cutoff)
	return int(deleted), AuthEventRetentionDays, err
}

// DeleteExpiredDataAccessLog removes the school's access-log rows once the
// year after the access has ended: everything before 1 January of last year
// (Berlin). It reports how many rows went and the age of the cutoff in days.
func (c *Command) DeleteExpiredDataAccessLog(ctx context.Context) (int, int, error) {
	store, err := c.ledgerRetention()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()
	cutoff := followingYearCutoff(now)
	deleted, err := store.DeleteExpiredDataAccessLog(ctx, cutoff)
	return int(deleted), int(now.Sub(cutoff).Hours() / 24), err
}

// followingYearCutoff keeps everything of last year and this year: rows
// before 1 January of last year (Berlin) are past the end of the year after
// their event.
func followingYearCutoff(now time.Time) time.Time {
	// The year is read a margin earlier: right after New Year a database
	// clock still in the old year would refuse the new cutoff.
	year := now.In(calendar.Berlin).Add(-auditModels.RetentionClockMargin).Year()
	return time.Date(year-1, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
}

// operatorLedgerRetentionStore is the Audit append store's access to the
// platform-wide operator ledger.
type operatorLedgerRetentionStore interface {
	DeleteExpiredOperatorAuditLog(ctx context.Context) (int64, error)
}

// DeleteExpiredOperatorAuditLog removes the operator-ledger entries once the
// year after the entry has ended (#4047), the window of the access log. The
// ledger belongs to moto, not to a school: the hourly cleanup runs it in an
// administrative transaction and logs no school deletion.
func (c *Command) DeleteExpiredOperatorAuditLog(ctx context.Context) (int, error) {
	store, ok := c.store.(operatorLedgerRetentionStore)
	if !ok {
		return 0, fmt.Errorf("audit command: operator ledger retention store is required")
	}
	deleted, err := store.DeleteExpiredOperatorAuditLog(ctx)
	return int(deleted), err
}

// DeleteExpiredDeletionLog removes the school's deletion-log rows older than
// three years and reports how many went and the window in days.
func (c *Command) DeleteExpiredDeletionLog(ctx context.Context) (int, int, error) {
	store, err := c.ledgerRetention()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()
	cutoff := now.AddDate(-DeletionLogRetentionYears, 0, 0).Add(-auditModels.RetentionClockMargin)
	deleted, err := store.DeleteExpiredDataDeletions(ctx, cutoff)
	return int(deleted), int(now.Sub(cutoff).Hours() / 24), err
}

// ProtocolRetentionKinds are the protocols of #4044 the Audit Platform owns,
// each logged as its own data kind. Migration 1.15.454 enforces the same
// windows in the database.
var ProtocolRetentionKinds = []string{
	auditModels.DeletionTypeStudentDeletionLogRetention,
	auditModels.DeletionTypeEnrollmentDeletionLogRetention,
	auditModels.DeletionTypeEnrollmentRestorationLogRetention,
	auditModels.DeletionTypeTimeTrackingDeletionLogRetention,
	auditModels.DeletionTypeStaffMasterDataChangeRetention,
	auditModels.DeletionTypePersonnelNumberChangeRetention,
	auditModels.DeletionTypeGuardianFinancialChangeRetention,
	auditModels.DeletionTypeClassListChangeRetention,
	auditModels.DeletionTypeSubstitutionChangeRetention,
	auditModels.DeletionTypeFileEventRetention,
}

// followingYearProtocols keep their rows until the end of the year after the
// event, like the access log: the protocols of daily operation. Every other
// protocol proves a deletion or a change and keeps its rows three years,
// like the deletion log.
var followingYearProtocols = map[string]bool{
	auditModels.DeletionTypeClassListChangeRetention:    true,
	auditModels.DeletionTypeSubstitutionChangeRetention: true,
	auditModels.DeletionTypeFileEventRetention:          true,
}

// DeleteExpiredProtocol removes the school's rows of the protocol of kind
// past its fixed window and reports how many went and the age of the cutoff
// in days.
func (c *Command) DeleteExpiredProtocol(ctx context.Context, kind string) (int, int, error) {
	if !slices.Contains(ProtocolRetentionKinds, kind) {
		return 0, 0, fmt.Errorf("audit retention: unknown protocol %q", kind)
	}
	store, err := c.ledgerRetention()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()
	cutoff := now.AddDate(-DeletionLogRetentionYears, 0, 0).Add(-auditModels.RetentionClockMargin)
	if followingYearProtocols[kind] {
		cutoff = followingYearCutoff(now)
	}
	deleted, err := store.DeleteExpiredProtocol(ctx, kind, cutoff)
	return int(deleted), int(now.Sub(cutoff).Hours() / 24), err
}
