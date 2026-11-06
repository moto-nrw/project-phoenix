package carecontract

import (
	"context"
	"errors"
	"time"
)

// Conclusion of a contract (#4680, ADR 0088 sections 2, 3 and 6). A contract
// is concluded once: at the acceptance of the application (Weg b, this file's
// Conclusion) or with the parents' signature (Weg a, #4679). Both ways end in
// the same transition, so a concluded contract looks the same whichever way
// it took: status aktiv, its first version wirksam, the method and the
// instant of the conclusion, and a frozen PDF the parents received.

// The methods of a conclusion (ADR 0088 section 3). MethodWithApplication is
// Weg b: the binding submission of the application is the proof. The other
// two belong to the signature of #4679.
const (
	MethodWithApplication = "mit_anmeldung"
	MethodElectronic      = "einfach_elektronisch"
	MethodPaper           = "papier"
)

// The values of the setting care_contract.conclusion_mode, a Träger value a
// school may override (ADR 0086). ModeSendLater is Weg a and the default:
// the Träger-Büro makes a draft and sends it. ModeOnAcceptance is Weg b: the
// contract is concluded when the school accepts the application.
const (
	ModeSendLater    = "nachtraeglich"
	ModeOnAcceptance = "bei_annahme"
)

// ErrContractNotConcludable reports a conclusion of a contract that is
// concluded already or ended.
var ErrContractNotConcludable = errors.New("the contract cannot be concluded")

// ConclusionModes reads how contracts of the school come about, in the
// school's tenant transaction the caller holds: ModeSendLater or
// ModeOnAcceptance.
type ConclusionModes interface {
	ConclusionMode(ctx context.Context, schoolID int64) (string, error)
}

// ConclusionRun answers one run of the conclusion at acceptance over the
// schools of a Träger: the concluded contracts and, per accepted child that
// stays without a contract, why.
type ConclusionRun struct {
	Concluded []int64      `json:"concluded"`
	Skipped   []CreateSkip `json:"skipped"`
}

// Conclusion concludes the contracts of the schools that conclude at the
// acceptance (Weg b). For every accepted application child with booked care
// and no contract it creates the contract, freezes its PDF, sends it to the
// parents by mail and into the inbox, and concludes it: status aktiv, its
// version wirksam, method mit_anmeldung. A child whose modules lack
// conditions, or whose parents have no address, stays without a contract
// and is tried again on the next run. The run reads each school in its own
// tenant transaction and writes each contract in its own Träger
// transaction, so it must be called outside any transaction. The key of a
// contract (the application child) keeps a repeated run from concluding
// twice.
type Conclusion interface {
	ConcludeAccepted(ctx context.Context, organizationID int64) (ConclusionRun, error)
}

// Concluded tells how and when a contract was concluded.
type Concluded struct {
	Method string    `json:"method"`
	At     time.Time `json:"at"`
}

// SchoolConclusion is how the contracts of one school of the Träger come
// about: ModeSendLater or ModeOnAcceptance.
type SchoolConclusion struct {
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
	Mode       string `json:"mode"`
}
