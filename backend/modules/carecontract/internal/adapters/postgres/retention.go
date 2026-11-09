package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ContractRetention serves carecontract.ContractRetention in the
// administrative transaction of the hourly job (ADR 0184).
type ContractRetention struct{}

// NewContractRetention builds the retention adapter.
func NewContractRetention() *ContractRetention { return &ContractRetention{} }

func retentionDB(ctx context.Context) (bun.IDB, error) {
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("care contract postgres: the contract retention requires the administrative transaction")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("care contract postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("care contract postgres: unsupported transaction %T", transaction)
	}
	return tx, nil
}

// endedContractStatuses are the states of a contract that ended.
const endedContractStatuses = `('gekuendigt', 'widerrufen', 'beendet')`

// RetentionFacts lists the contracts past their draft. A terminated
// contract ends on its last day, a withdrawn one on the day the withdrawal
// was received; an ended contract without such a day has none.
func (ContractRetention) RetentionFacts(ctx context.Context, organizationID int64) ([]carecontract.RetentionFact, error) {
	db, err := retentionDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		OrganizationID int64          `bun:"organization_id"`
		ID             int64          `bun:"id"`
		ConcludedOn    *calendar.Date `bun:"concluded_on,type:date"`
		Ended          bool           `bun:"ended"`
		EndsOn         *calendar.Date `bun:"ends_on,type:date"`
	}
	if err := db.NewRaw(`SELECT c.organization_id, c.id, (c.concluded_at AT TIME ZONE 'Europe/Berlin')::date AS concluded_on,
			c.status IN `+endedContractStatuses+` AS ended,
			CASE c.status
				WHEN 'gekuendigt' THEN c.ends_on
				WHEN 'widerrufen' THEN (SELECT max(n.received_on) FROM care_contract.contract_notices n
					WHERE n.organization_id = c.organization_id AND n.contract_id = c.id AND n.kind = 'widerruf')
			END AS ends_on
		FROM care_contract.contracts c WHERE c.status <> 'entwurf' AND (?0 = 0 OR c.organization_id = ?0)
		ORDER BY c.organization_id, c.id`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: list contract retention facts: %w", err)
	}
	facts := make([]carecontract.RetentionFact, len(rows))
	for i, row := range rows {
		facts[i] = carecontract.RetentionFact{OrganizationID: row.OrganizationID, ContractID: row.ID,
			ConcludedOn: row.ConcludedOn, Ended: row.Ended, EndedOn: row.EndsOn}
	}
	return facts, nil
}

// DeleteEndedContracts deletes the ended contracts among contractIDs; the
// foreign keys cascade to everything of the contract. Its trail has no
// foreign key and goes in the same statement.
func (ContractRetention) DeleteEndedContracts(ctx context.Context, organizationID int64, contractIDs []int64, today calendar.Date) ([]int64, error) {
	if organizationID <= 0 || len(contractIDs) == 0 {
		return nil, nil
	}
	db, err := retentionDB(ctx)
	if err != nil {
		return nil, err
	}
	var deleted []int64
	err = db.NewRaw(`WITH removed AS (
			DELETE FROM care_contract.contracts c
			WHERE c.organization_id = ?0 AND c.id = ANY(?1) AND c.status IN `+endedContractStatuses+`
				AND (c.status <> 'gekuendigt' OR c.ends_on <= ?2::date)
			RETURNING c.id),
		trail AS (
			DELETE FROM care_contract.contract_events e USING removed
			WHERE e.organization_id = ?0 AND e.contract_id = removed.id)
		SELECT id FROM removed ORDER BY id`, organizationID, pgdialect.Array(contractIDs), today).Scan(ctx, &deleted)
	if err != nil {
		return nil, fmt.Errorf("care contract postgres: delete ended contracts: %w", err)
	}
	return deleted, nil
}
