package carecontract

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Träger-Büro decides the parents' change of the booked care itself
// (#5979, #4681). The parents ask in the parents app for other days or
// modules; the school or the Träger-Büro decides. The decision is the
// school's own decision of Care Plan, run in the tenant transaction of the
// contract's school: it books the change, and the contract follows the
// booked care with a new version like after a decision of the school
// (AmendFromBookings). There is no second way to a version.

// The states of a module in a ChangeLine, as Care Plan names them.
const (
	ChangeModuleAdded     = "added"
	ChangeModuleRemoved   = "removed"
	ChangeModuleChanged   = "changed"
	ChangeModuleUnchanged = "unchanged"
)

// ChangeLine is one module of a change: what the child books now and what
// the parents ask for. Before or After is empty when the module is added or
// removed.
type ChangeLine struct {
	CareOfferingID int64    `json:"care_offering_id"`
	ModuleName     string   `json:"module_name"`
	State          string   `json:"state"`
	BeforeDays     []string `json:"before_days"`
	AfterDays      []string `json:"after_days"`
}

// ChangeRequest is a change of the booked care the parents asked for and
// nobody decided yet. Version pins the request a decision is taken on.
type ChangeRequest struct {
	ID            int64         `json:"id"`
	Version       string        `json:"version"`
	SubmittedAt   time.Time     `json:"submitted_at"`
	EffectiveFrom calendar.Date `json:"effective_from"`
	ParentNote    string        `json:"parent_note"`
	// EndsAllCare tells that the parents give up every module: that is a
	// termination of the contract (#4682), which the office records under
	// "Vertrag beenden"; the office can only reject the request here.
	EndsAllCare bool         `json:"ends_all_care"`
	Lines       []ChangeLine `json:"lines"`
}

// ChangeOutcome is the newest decided change of the child.
type ChangeOutcome struct {
	Approved      bool          `json:"approved"`
	DecidedAt     time.Time     `json:"decided_at"`
	EffectiveFrom calendar.Date `json:"effective_from"`
	Reason        string        `json:"reason"`
}

// ContractChanges is what the contract page shows: the open change of the
// child at the contract's school, if any, and the last decision.
type ContractChanges struct {
	// Decidable tells that the office may decide here: the contract is active.
	Decidable    bool            `json:"decidable"`
	Pending      []ChangeRequest `json:"pending"`
	LastDecision *ChangeOutcome  `json:"last_decision"`
	// ReasonRequired tells that the school asks for a reason on an approval
	// too; a rejection always needs one.
	ReasonRequired bool `json:"reason_required"`
}

// ChangeDecisionInput is the office's decision of one request. An approval
// takes effect on EffectiveFrom, which the office confirms; a rejection
// needs a reason.
type ChangeDecisionInput struct {
	Approve         bool           `json:"approve"`
	Reason          string         `json:"reason"`
	EffectiveFrom   *calendar.Date `json:"effective_from"`
	ExpectedVersion string         `json:"expected_version"`
}

// ContractChangeDecisions is the office's side of the parents' changes.
type ContractChangeDecisions interface {
	// ContractChanges reads the open change of the contract's child. It
	// writes the read into the access log of the school.
	ContractChanges(ctx context.Context, reviewer Reviewer, contractID int64) (ContractChanges, error)
	// DecideContractChange approves or rejects the request in the school's
	// tenant transaction and returns the changes as they stand afterwards.
	DecideContractChange(ctx context.Context, reviewer Reviewer, contractID, requestID int64, input ChangeDecisionInput) (ContractChanges, error)
}

// ChangeDecision is a decision as the school's port carries it: the child
// the contract names and the acting account of the Träger-Büro.
type ChangeDecision struct {
	ChangeDecisionInput
	StudentID int64
	RequestID int64
	AccountID int64
}

// SchoolChangeRequests reads and decides the parents' changes of one child
// in the tenant transaction of its school, which the caller holds (Care
// Plan). Pending lists the open requests and the last decision.
type SchoolChangeRequests interface {
	PendingChanges(ctx context.Context, studentID int64) ([]ChangeRequest, *ChangeOutcome, bool, error)
	DecideChange(ctx context.Context, decision ChangeDecision) error
}

// SchoolAccessChanges is a read of the parents' changes of a contract's
// child by the Träger-Büro (#5979).
const SchoolAccessChanges = "contract_changes"

// The failures of a decision the office can act on. The school's port
// translates Care Plan's failures into these.
var (
	// ErrContractNotActive reports a change on a contract that is not
	// active: only an active contract follows a change with a version.
	ErrContractNotActive = errors.New("the contract is not active")
	// ErrChangeNotFound reports a request that is not one of the child.
	ErrChangeNotFound = errors.New("change request not found")
	// ErrChangeDecided reports a request somebody decided already.
	ErrChangeDecided = errors.New("the change request is decided already")
	// ErrChangeStale reports a request that changed since the office read it.
	ErrChangeStale = errors.New("the change request changed")
	// ErrChangePast reports an approval of a day that has passed.
	ErrChangePast = errors.New("the change request lies in the past")
	// ErrChangeReasonRequired reports a decision without its reason.
	ErrChangeReasonRequired = errors.New("the decision needs a reason")
	// ErrChangeEndsCare reports an approval that ends every module: that is
	// a termination (#4682).
	ErrChangeEndsCare = errors.New("the change ends all care")
	// ErrChangeCapacityFull reports a module that has no free place.
	ErrChangeCapacityFull = errors.New("a requested module is full")
	// ErrChangeDateOutOfRange reports an effective day the school does not
	// allow.
	ErrChangeDateOutOfRange = errors.New("the effective day is out of range")
	// ErrChangeNotApplicable reports a change the school's rules refuse now
	// (no enrollment, a day conflict, the offerings switched off).
	ErrChangeNotApplicable = errors.New("the change cannot be applied")
	// ErrChangeInvalid reports a malformed decision.
	ErrChangeInvalid = errors.New("the decision is invalid")
)
