package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// contractNoticeTargetSQL reads the school and the child of a contract that
// still lies in the inbox of the address (#5941): sent to it, assigned to a
// child and in a state the parents may still act on. A draft, an ended,
// terminated or withdrawn contract and a contract of another Träger answer
// nothing. The Träger is named again beside the Träger policy.
const contractNoticeTargetSQL = `SELECT c.school_id, c.student_id FROM care_contract.contracts AS c
	WHERE c.organization_id = ? AND c.id = ? AND c.student_id IS NOT NULL
		AND c.status IN ('` + carecontract.StatusSent + `', '` + carecontract.StatusActive + `')
		AND EXISTS (SELECT 1 FROM care_contract.contract_mailings AS m
			WHERE m.organization_id = c.organization_id AND m.contract_id = c.id AND lower(m.recipient_email) = lower(?))`

// ContractNoticeTarget reads the school and the child of a contract in the
// inbox of email, in the caller's Träger transaction; it records no read. A
// missing contract or address finds nothing.
func (Store) ContractNoticeTarget(ctx context.Context, organizationID, contractID int64, email string) (carecontract.ContractNoticeTarget, bool, error) {
	address := strings.TrimSpace(email)
	if contractID <= 0 || address == "" {
		return carecontract.ContractNoticeTarget{}, false, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return carecontract.ContractNoticeTarget{}, false, err
	}
	var rows []struct {
		SchoolID  int64 `bun:"school_id"`
		StudentID int64 `bun:"student_id"`
	}
	if err := db.NewRaw(contractNoticeTargetSQL, organizationID, contractID, address).Scan(ctx, &rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return carecontract.ContractNoticeTarget{}, false, fmt.Errorf("care contract postgres: read contract notice target: %w", err)
	}
	if len(rows) == 0 {
		return carecontract.ContractNoticeTarget{}, false, nil
	}
	target := carecontract.ContractNoticeTarget{SchoolID: rows[0].SchoolID, StudentID: rows[0].StudentID}
	return target, true, nil
}
