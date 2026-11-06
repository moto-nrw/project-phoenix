package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ContractApplication is an accepted application child of one school with
// the guardian who applied, as the Betreuungsvertrag of the Träger turns it
// into a contract (#4678, ADR 0153). It names no booking: Care Plan keeps
// those.
type ContractApplication struct {
	RequestChildID    int64
	StudentID         *int64
	ChildFirstName    string
	ChildLastName     string
	ChildBirthDate    *calendar.Date
	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	GuardianAccountID *int64
	PhaseID           int64
	PhaseName         string
	ServiceStartDate  *calendar.Date
	ActivateOn        *calendar.Date
}

// ContractApplications reads the accepted application children of the
// school whose tenant transaction the caller holds. A non-empty
// requestChildIDs keeps only those. It reads nothing else.
type ContractApplications interface {
	AcceptedContractApplications(ctx context.Context, requestChildIDs []int64) ([]ContractApplication, error)
	// ContractPhaseNames names the school's phases by ID.
	ContractPhaseNames(ctx context.Context, phaseIDs []int64) (map[int64]string, error)
}
