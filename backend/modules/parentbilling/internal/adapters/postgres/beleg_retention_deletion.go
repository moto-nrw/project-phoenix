package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// DeleteDueBelege deletes the Belege of parent-billing whose instructed
// period ended on today, and the released runs left without one, through
// parent_billing.delete_due_belege (migration 1.15.828, ADR 0184). The
// function deletes a Beleg only when nothing its cascade would take is
// still kept, drops the records of what went and logs the counts.
func (s *Store) DeleteDueBelege(ctx context.Context, today calendar.Date) (int, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return 0, err
	}
	var rows []struct {
		BelegType string `bun:"beleg_type"`
		Deleted   int    `bun:"deleted"`
	}
	if err := db.NewRaw(`SELECT beleg_type, deleted FROM parent_billing.delete_due_belege(?::date)`, today).Scan(ctx, &rows); err != nil {
		return 0, fmt.Errorf("parent billing postgres: delete due belege: %w", err)
	}
	deleted := 0
	for _, row := range rows {
		deleted += row.Deleted
	}
	return deleted, nil
}

// belegLockTimeout bounds a lock wait of the retention, like the
// lock_timeout of parent_billing.delete_due_belege.
const belegLockTimeout = "100ms"

// DueContracts locks the contracts whose period ended on today against new
// mandates and debits and lists those to which none hangs and no
// suspension applies, with the event and the last day it may lie on under
// the current Weisung; their deletion would take mandates and debits by
// cascade. A contract another transaction holds is skipped. The deletion
// that follows in this transaction waits at most belegLockTimeout for a
// row of its cascade, so the job fails and retries in the next hour rather
// than holding up billing or deadlocking with it.
func (s *Store) DueContracts(ctx context.Context, today calendar.Date) ([]parentbilling.DueContract, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `SELECT set_config('lock_timeout', ?, true)`, belegLockTimeout); err != nil {
		return nil, fmt.Errorf("parent billing postgres: bound the contract deletion's lock wait: %w", err)
	}
	var rows []struct {
		OrganizationID int64         `bun:"organization_id"`
		ContractID     int64         `bun:"contract_id"`
		TriggerEvent   string        `bun:"trigger_event"`
		LastEventOn    calendar.Date `bun:"last_event_on,type:date"`
	}
	if err := db.NewRaw(`SELECT organization_id, contract_id, trigger_event, last_event_on FROM parent_billing.due_retained_contracts(?::date)`, today).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent billing postgres: list due contracts: %w", err)
	}
	contracts := make([]parentbilling.DueContract, len(rows))
	for i, row := range rows {
		contracts[i] = parentbilling.DueContract(row)
	}
	return contracts, nil
}

// LogContractDeletions drops the records of the contracts Care Contract
// deleted and logs their count.
func (s *Store) LogContractDeletions(ctx context.Context, organizationID int64, today calendar.Date, contractIDs []int64) error {
	db, err := adminDB(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`DELETE FROM parent_billing.beleg_retentions r
		WHERE r.organization_id = ? AND r.beleg_type = ? AND r.beleg_id = ANY(?)`,
		organizationID, parentbilling.BelegCareContract, pgdialect.Array(contractIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("parent billing postgres: drop records of deleted contracts: %w", err)
	}
	if len(contractIDs) == 0 {
		return nil
	}
	if _, err := db.NewRaw(`INSERT INTO parent_billing.beleg_deletions (organization_id, beleg_type, deleted_on, deleted_count)
		VALUES (?, ?, ?::date, ?)`, organizationID, parentbilling.BelegCareContract, today, len(contractIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("parent billing postgres: log deleted contracts: %w", err)
	}
	return nil
}

// DeleteExpiredBelegDeletions deletes the entries of the deletion log older
// than years, like the deletion log of a school.
func (s *Store) DeleteExpiredBelegDeletions(ctx context.Context, years int) (int, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return 0, err
	}
	result, err := db.NewRaw(`DELETE FROM parent_billing.beleg_deletions WHERE deleted_at < CURRENT_TIMESTAMP - make_interval(years => ?)`, years).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("parent billing postgres: delete expired beleg deletion log: %w", err)
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}
