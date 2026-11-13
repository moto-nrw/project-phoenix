package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
)

// ChangeContract reads the school, the child and the status of a contract
// of the Träger (#5979). The filter on organization_id holds without the
// Träger policy too; a contract of another Träger is not found.
func (Store) ChangeContract(ctx context.Context, organizationID, contractID int64) (ports.ChangeContract, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return ports.ChangeContract{}, err
	}
	return scanChangeContract(ctx, db.NewRaw(`SELECT id, school_id, request_child_id, student_id, status FROM care_contract.contracts
		WHERE organization_id = ? AND id = ?`, organizationID, contractID))
}

// LockChangeContract reads the contract like ChangeContract under the row
// lock withdrawal and termination take (#4682), so its status holds until
// the caller's Träger transaction ends.
func (Store) LockChangeContract(ctx context.Context, organizationID, contractID int64) (ports.ChangeContract, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return ports.ChangeContract{}, err
	}
	return scanChangeContract(ctx, db.NewRaw(`SELECT id, school_id, request_child_id, student_id, status FROM care_contract.contracts
		WHERE organization_id = ? AND id = ? FOR UPDATE`, organizationID, contractID))
}

func scanChangeContract(ctx context.Context, query *bun.RawQuery) (ports.ChangeContract, error) {
	var row struct {
		ID             int64  `bun:"id"`
		SchoolID       int64  `bun:"school_id"`
		RequestChildID int64  `bun:"request_child_id"`
		StudentID      *int64 `bun:"student_id"`
		Status         string `bun:"status"`
	}
	err := query.Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ChangeContract{}, carecontract.ErrContractNotFound
	}
	if err != nil {
		return ports.ChangeContract{}, fmt.Errorf("care contract postgres: read contract of a change: %w", err)
	}
	contract := ports.ChangeContract{ID: row.ID, SchoolID: row.SchoolID, Status: row.Status, RequestChildID: row.RequestChildID}
	if row.StudentID != nil {
		contract.StudentID = *row.StudentID
	}
	return contract, nil
}
