package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BookedCare reads the booked services of the contracts at the school whose
// tenant transaction the caller holds (#4683, ADR 0156). The school policy of
// migration 1.15.785 admits only the school's contracts past the draft; the
// statement names the school and the status once more.
type BookedCare struct{}

var _ carecontract.BookedCare = BookedCare{}

func NewBookedCare() BookedCare { return BookedCare{} }

// schoolDB returns the tenant transaction of the reading school. A Träger
// or administrative transaction is refused: this read belongs to the school.
func schoolDB(ctx context.Context) (bun.IDB, int64, error) {
	school, err := tenant.TenantFromContext(ctx)
	if err != nil || tenant.IsAdminTx(ctx) || tenant.TraegerTransactionFromContext(ctx) != 0 {
		return nil, 0, errors.New("care contract postgres: the tenant transaction of the school is required")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("care contract postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("care contract postgres: unsupported transaction %T", transaction)
	}
	return tx, school.Int64(), nil
}

type bookedVersionRow struct {
	ContractID    int64          `bun:"contract_id"`
	StudentID     int64          `bun:"student_id"`
	VersionNumber int            `bun:"version_number"`
	EffectiveFrom calendar.Date  `bun:"effective_from,type:date"`
	EndsOn        *calendar.Date `bun:"ends_on,type:date"`
	Services      string         `bun:"services"`
}

// bookedCareSQL reads every version of the children's contracts that
// overlaps the window and is neither rejected nor replaced, the newest
// effective version of a child first. A replaced version holds on no day
// (#4681), so it never enters the Soll, whatever the order of the versions.
// A terminated contract ends its versions on its last day, and a withdrawn
// one books no day at all (#4682), as VersionsInForce answers them.
const bookedCareSQL = `SELECT * FROM (
	SELECT c.id AS contract_id, c.student_id, v.version_number, v.effective_from,
		LEAST(v.ends_on, c.ends_on) AS ends_on, ` + servicesSQL + ` AS services
	FROM care_contract.contracts AS c
	JOIN care_contract.contract_versions AS v ON v.contract_id = c.id AND v.organization_id = c.organization_id
	WHERE c.school_id = ? AND c.student_id = ANY(?) AND c.status NOT IN (?, '` + carecontract.StatusWithdrawn + `')
		AND v.status NOT IN (?, ?) AND v.effective_from <= ?
		AND (c.ends_on IS NULL OR v.effective_from <= c.ends_on)) AS b
	WHERE b.ends_on IS NULL OR b.ends_on >= ?
	ORDER BY b.student_id, b.effective_from DESC, b.version_number DESC, b.contract_id DESC`

// BookedCare lists the booked care of the children by child. A child with
// several contracts at the school gets the versions of all of them, the
// newest first, so the newest version that covers a day decides it.
func (BookedCare) BookedCare(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[int64]carecontract.BookedContract, error) {
	result := map[int64]carecontract.BookedContract{}
	if len(studentIDs) == 0 {
		return result, nil
	}
	db, school, err := schoolDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []bookedVersionRow
	if err := db.NewRaw(bookedCareSQL, school, pgdialect.Array(studentIDs), carecontract.StatusDraft,
		carecontract.VersionRejected, carecontract.VersionReplaced, to, from).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care contract postgres: read booked care: %w", err)
	}
	for _, row := range rows {
		services := []carecontract.Service{}
		if err := json.Unmarshal([]byte(row.Services), &services); err != nil {
			return nil, fmt.Errorf("care contract postgres: read booked services: %w", err)
		}
		contract, ok := result[row.StudentID]
		if !ok {
			contract = carecontract.BookedContract{ContractID: row.ContractID, StudentID: row.StudentID}
		}
		contract.Versions = append(contract.Versions, carecontract.BookedVersion{
			Number: row.VersionNumber, EffectiveFrom: row.EffectiveFrom, EndsOn: row.EndsOn, Services: services,
		})
		result[row.StudentID] = contract
	}
	return result, nil
}
