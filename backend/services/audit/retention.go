package audit

import (
	"context"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// LogRetention records in audit.data_deletions that a nightly retention step
// deleted deleted rows of one data kind for the school of ctx (#3950). The
// entry names no person and holds no deleted content; kind is one of the
// data-kind deletion types of models/audit.
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
	cutoff := time.Now().AddDate(0, 0, -AuthEventRetentionDays).Add(-retentionClockMargin)
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
	now := time.Now().In(calendar.Berlin)
	// The year is read a margin earlier: right after New Year a database
	// clock still in the old year would refuse the new cutoff.
	year := now.Add(-retentionClockMargin).Year()
	cutoff := time.Date(year-1, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
	deleted, err := store.DeleteExpiredDataAccessLog(ctx, cutoff)
	return int(deleted), int(now.Sub(cutoff).Hours() / 24), err
}

// DeleteExpiredDeletionLog removes the school's deletion-log rows older than
// three years and reports how many went and the window in days.
func (c *Command) DeleteExpiredDeletionLog(ctx context.Context) (int, int, error) {
	store, err := c.ledgerRetention()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now()
	cutoff := now.AddDate(-DeletionLogRetentionYears, 0, 0).Add(-retentionClockMargin)
	deleted, err := store.DeleteExpiredDataDeletions(ctx, cutoff)
	return int(deleted), int(now.Sub(cutoff).Hours() / 24), err
}
