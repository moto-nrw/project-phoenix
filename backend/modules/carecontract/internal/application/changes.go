package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ChangeStore reads the contract of a change (#5979).
type ChangeStore = ports.ChangeStore

// The views of the access log for the parents' changes.
const (
	changeViewRead    = "contract_changes"
	changeViewApprove = "contract_change_approve"
	changeViewReject  = "contract_change_reject"
)

// ContractChanges serves carecontract.ContractChangeDecisions (#5979). It
// reads the contract in the Träger transaction and decides in the tenant
// transaction of the contract's school, after it checked that the school
// still belongs to the Träger. Care Plan locks the request and decides it
// exactly as for the school; the contract follows the booked care later
// with the amendment of the Worker, under the contract lock.
type ContractChanges struct {
	store ChangeStore
	ports carecontract.ChangePorts
}

var _ carecontract.ContractChangeDecisions = (*ContractChanges)(nil)

// NewContractChanges builds the office's decisions.
func NewContractChanges(store ChangeStore, changePorts carecontract.ChangePorts) *ContractChanges {
	if store == nil || changePorts.Schools == nil || changePorts.Requests == nil || changePorts.Access == nil || changePorts.Runtime == nil {
		panic("care contract: the store, the schools, the change requests, the access log and the runtime are required")
	}
	return &ContractChanges{store: store, ports: changePorts}
}

// ContractChanges reads the open change of the contract's child.
func (c *ContractChanges) ContractChanges(ctx context.Context, reviewer carecontract.Reviewer, contractID int64, effectiveFrom *calendar.Date) (carecontract.ContractChanges, error) {
	contract, err := c.contract(ctx, reviewer, contractID)
	if err != nil {
		return carecontract.ContractChanges{}, err
	}
	result := carecontract.ContractChanges{Decidable: contract.Status == carecontract.StatusActive, Pending: []carecontract.ChangeRequest{}}
	if contract.StudentID <= 0 {
		return result, nil
	}
	err = c.ports.Runtime.WithinSchool(ctx, contract.SchoolID, func(ctx context.Context) error {
		read, readErr := c.read(ctx, contract, result.Decidable, effectiveFrom)
		if readErr != nil {
			return readErr
		}
		result = read
		return c.log(ctx, reviewer, contract, changeViewRead, len(read.Pending))
	})
	if err != nil {
		return carecontract.ContractChanges{}, err
	}
	return result, nil
}

// DecideContractChange approves or rejects one request of the contract's
// child and answers the changes as they stand afterwards.
func (c *ContractChanges) DecideContractChange(ctx context.Context, reviewer carecontract.Reviewer, contractID, requestID int64, input carecontract.ChangeDecisionInput) (carecontract.ContractChanges, error) {
	if err := validateChangeDecision(requestID, input); err != nil {
		return carecontract.ContractChanges{}, err
	}
	contract, err := c.contract(ctx, reviewer, contractID)
	if err != nil {
		return carecontract.ContractChanges{}, err
	}
	if contract.StudentID <= 0 {
		return carecontract.ContractChanges{}, carecontract.ErrChangeNotFound
	}
	view := changeViewReject
	if input.Approve {
		view = changeViewApprove
	}
	input.Reason = strings.TrimSpace(input.Reason)
	var result carecontract.ContractChanges
	// The decision holds the contract's row lock, the lock withdrawal and
	// termination take (#4682): the contract cannot end between the status
	// check and the booked change. The school transaction runs beside the
	// Träger transaction, not inside it; Care Plan takes no contract lock, so
	// the order is always contract, then request.
	err = c.ports.Runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(traegerCtx context.Context) error {
		locked, lockErr := c.store.LockChangeContract(traegerCtx, reviewer.OrganizationID, contractID)
		if lockErr != nil {
			return lockErr
		}
		if locked.Status != carecontract.StatusActive {
			return carecontract.ErrContractNotActive
		}
		return c.ports.Runtime.WithinSchool(ctx, contract.SchoolID, func(ctx context.Context) error {
			impact, decideErr := c.ports.Requests.DecideChange(ctx, carecontract.ChangeDecision{
				ChangeDecisionInput: input, StudentID: contract.StudentID, RequestChildID: contract.RequestChildID, RequestID: requestID, AccountID: reviewer.AccountID,
			})
			if decideErr != nil {
				return decideErr
			}
			read, readErr := c.read(ctx, contract, true, nil)
			if readErr != nil {
				return readErr
			}
			result = read
			result.RosterImpact = impact
			return c.log(ctx, reviewer, contract, view, 1)
		})
	})
	if err != nil {
		return carecontract.ContractChanges{}, err
	}
	return result, nil
}

func validateChangeDecision(requestID int64, input carecontract.ChangeDecisionInput) error {
	if requestID <= 0 {
		return fmt.Errorf("%w: the request is required", carecontract.ErrChangeInvalid)
	}
	// An approval applies on a day the office confirms; a rejection applies
	// on none.
	if input.Approve && input.EffectiveFrom == nil {
		return fmt.Errorf("%w: an approval needs its effective day", carecontract.ErrChangeInvalid)
	}
	if !input.Approve && input.EffectiveFrom != nil {
		return fmt.Errorf("%w: a rejection has no effective day", carecontract.ErrChangeInvalid)
	}
	if !input.Approve && strings.TrimSpace(input.Reason) == "" {
		return carecontract.ErrChangeReasonRequired
	}
	return nil
}

// contract reads the contract in the Träger transaction and checks that
// its school still belongs to the Träger: a school that moved to another
// Träger is no longer the office's to decide for.
func (c *ContractChanges) contract(ctx context.Context, reviewer carecontract.Reviewer, contractID int64) (ports.ChangeContract, error) {
	schools, err := c.ports.Schools.TraegerSchools(ctx, reviewer.OrganizationID)
	if err != nil {
		return ports.ChangeContract{}, err
	}
	var contract ports.ChangeContract
	err = c.ports.Runtime.WithinTraeger(ctx, reviewer.OrganizationID, func(ctx context.Context) error {
		var readErr error
		contract, readErr = c.store.ChangeContract(ctx, reviewer.OrganizationID, contractID)
		return readErr
	})
	if err != nil {
		return ports.ChangeContract{}, err
	}
	if !slices.ContainsFunc(schools, func(school carecontract.School) bool { return school.ID == contract.SchoolID }) {
		return ports.ChangeContract{}, carecontract.ErrSchoolOutsideTraeger
	}
	return contract, nil
}

// read lists the child's open change in the school transaction.
func (c *ContractChanges) read(ctx context.Context, contract ports.ChangeContract, decidable bool, effectiveFrom *calendar.Date) (carecontract.ContractChanges, error) {
	pending, last, reasonRequired, err := c.ports.Requests.PendingChanges(ctx, contract.StudentID, contract.RequestChildID, effectiveFrom)
	if err != nil {
		return carecontract.ContractChanges{}, err
	}
	if pending == nil {
		pending = []carecontract.ChangeRequest{}
	}
	return carecontract.ContractChanges{Decidable: decidable, Pending: pending, LastDecision: last, ReasonRequired: reasonRequired}, nil
}

// log writes the office's access into the access log of the school, in
// the same school transaction as the read or the decision.
func (c *ContractChanges) log(ctx context.Context, reviewer carecontract.Reviewer, contract ports.ChangeContract, view string, rows int) error {
	return c.ports.Access.RecordAccess(ctx, carecontract.SchoolAccess{
		OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID, Data: carecontract.SchoolAccessChanges,
		View: view, Rows: rows, ContractIDs: []int64{contract.ID}, StudentIDs: []int64{contract.StudentID},
	})
}
