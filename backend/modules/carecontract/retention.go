package carecontract

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RetentionFact is one contract past its draft, as the retention of the
// Belege of a Träger needs it (#4720, ADR 0184): when it was concluded and
// whether and on which day it ended. A contract terminated, withdrawn or
// ended has Ended; EndedOn is its last day where the contract names one.
type RetentionFact struct {
	OrganizationID int64
	ContractID     int64
	ConcludedOn    *calendar.Date
	Ended          bool
	EndedOn        *calendar.Date
}

// ContractRetention lets the Elternabrechnung keep the retention record of
// each contract and delete a contract after the period the Träger
// instructed (ADR 0184). Both calls join the caller's administrative
// transaction; the Elternabrechnung decides when a contract is due.
type ContractRetention interface {
	// RetentionFacts lists the contracts past their draft of one Träger
	// or, for organizationID 0, of every Träger.
	RetentionFacts(ctx context.Context, organizationID int64) ([]RetentionFact, error)
	// DeleteEndedContracts deletes the given contracts of the Träger that
	// ended by today, with versions, parties, PDFs, mailings, signatures,
	// links, reminders, notices and their trail, and answers the IDs that
	// went. A draft or a contract in force on today is never deleted.
	DeleteEndedContracts(ctx context.Context, organizationID int64, contractIDs []int64, today calendar.Date) ([]int64, error)
}
