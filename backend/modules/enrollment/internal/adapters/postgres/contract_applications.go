package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ContractApplicationStore reads the accepted application children for the
// Betreuungsvertrag (#4678) in the caller's tenant transaction.
type ContractApplicationStore struct{ fingerprint func([]byte) string }

var _ enrollment.ContractApplications = ContractApplicationStore{}

// NewContractApplicationStore builds the store.
func NewContractApplicationStore(fingerprint func([]byte) string) ContractApplicationStore {
	if fingerprint == nil {
		panic("enrollment: contract evidence fingerprint is required")
	}
	return ContractApplicationStore{fingerprint: fingerprint}
}

const acceptedContractApplicationsSQL = `SELECT child.id AS request_child_id,
	COALESCE(child.created_student_id, child.matched_student_id) AS student_id,
	child.first_name AS child_first_name, child.last_name AS child_last_name,
	child.date_of_birth AS child_birth_date,
	request.guardian_first_name, request.guardian_last_name, request.guardian_email, request.guardian_account_id,
	request.legal_blocks_snapshot, request.submission_source, child.reviewed_at AS accepted_at,
 phase.id AS phase_id, phase.name AS phase_name, phase.service_start_date, child.activate_on
	FROM enrollment.request_children AS child
	JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id
	JOIN enrollment.phases AS phase ON phase.id = request.phase_id AND phase.tenant_id = request.tenant_id
	WHERE child.tenant_id = ? AND child.status = 'approved' AND request.withdrawn_at IS NULL
		AND (cardinality(?::bigint[]) = 0 OR child.id = ANY(?::bigint[]))
	ORDER BY child.last_name, child.first_name, child.id`

// AcceptedContractApplications lists the accepted children of the school.
func (s ContractApplicationStore) AcceptedContractApplications(ctx context.Context, requestChildIDs []int64) ([]enrollment.ContractApplication, error) {
	tx, tenantID, err := tenant.CurrentTenantTransaction[bun.Tx](ctx)
	if err != nil {
		return nil, fmt.Errorf("enrollment postgres: contract applications need the school transaction: %w", err)
	}
	if requestChildIDs == nil {
		requestChildIDs = []int64{}
	}
	var rows []contractApplicationRow
	if err := tx.NewRaw(acceptedContractApplicationsSQL, tenantID, pgdialect.Array(requestChildIDs),
		pgdialect.Array(requestChildIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: contract applications: %w", err)
	}
	result := make([]enrollment.ContractApplication, len(rows))
	for i, row := range rows {
		result[i] = row.application(s.fingerprint)
	}
	return result, nil
}

type contractApplicationRow struct {
	AcceptedAt          *time.Time      `bun:"accepted_at"`
	LegalBlocksSnapshot json.RawMessage `bun:"legal_blocks_snapshot"`
	SubmissionSource    string          `bun:"submission_source"`
	RequestChildID      int64           `bun:"request_child_id"`
	StudentID           *int64          `bun:"student_id"`
	ChildFirstName      string          `bun:"child_first_name"`
	ChildLastName       string          `bun:"child_last_name"`
	ChildBirthDate      *calendar.Date  `bun:"child_birth_date,type:date"`
	GuardianFirstName   string          `bun:"guardian_first_name"`
	GuardianLastName    string          `bun:"guardian_last_name"`
	GuardianEmail       string          `bun:"guardian_email"`
	GuardianAccountID   *int64          `bun:"guardian_account_id"`
	PhaseID             int64           `bun:"phase_id"`
	PhaseName           string          `bun:"phase_name"`
	ServiceStartDate    *calendar.Date  `bun:"service_start_date,type:date"`
	ActivateOn          *calendar.Date  `bun:"activate_on,type:date"`
}

// ContractPhases reads the school's phase names and inclusive service bounds.
func (ContractApplicationStore) ContractPhases(ctx context.Context, phaseIDs []int64) (map[int64]enrollment.ContractPhase, error) {
	names := map[int64]enrollment.ContractPhase{}
	if len(phaseIDs) == 0 {
		return names, nil
	}
	tx, tenantID, err := tenant.CurrentTenantTransaction[bun.Tx](ctx)
	if err != nil {
		return nil, fmt.Errorf("enrollment postgres: phase names need the school transaction: %w", err)
	}
	var rows []struct {
		ID           int64          `bun:"id"`
		Name         string         `bun:"name"`
		ServiceStart *calendar.Date `bun:"service_start_date,type:date"`
		ServiceEnd   *calendar.Date `bun:"service_end_date,type:date"`
	}
	if err := tx.NewRaw(`SELECT id, name, service_start_date, service_end_date FROM enrollment.phases WHERE tenant_id = ? AND id = ANY(?)`,
		tenantID, pgdialect.Array(phaseIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: phase names: %w", err)
	}
	for _, row := range rows {
		names[row.ID] = enrollment.ContractPhase{Name: row.Name, ServiceStart: row.ServiceStart, ServiceEnd: row.ServiceEnd}
	}
	return names, nil
}

func (r contractApplicationRow) application(fingerprint func([]byte) string) enrollment.ContractApplication {
	return enrollment.ContractApplication{
		AcceptedAt: r.AcceptedAt, Acceptances: enrollment.ContractAcceptances(r.LegalBlocksSnapshot, r.SubmissionSource, r.AcceptedAt, fingerprint),
		RequestChildID: r.RequestChildID, StudentID: r.StudentID,
		ChildFirstName: r.ChildFirstName, ChildLastName: r.ChildLastName, ChildBirthDate: r.ChildBirthDate,
		GuardianFirstName: r.GuardianFirstName, GuardianLastName: r.GuardianLastName, GuardianEmail: r.GuardianEmail,
		GuardianAccountID: r.GuardianAccountID, PhaseID: r.PhaseID, PhaseName: r.PhaseName,
		ServiceStartDate: r.ServiceStartDate, ActivateOn: r.ActivateOn,
	}
}
