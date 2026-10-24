package identityaccess

import (
	"context"
	"errors"
)

// ErrOperatorSelfDeactivation reports an operator who tried to deactivate
// their own account.
var ErrOperatorSelfDeactivation = errors.New("an operator cannot deactivate their own account")

// OperatorStatusChange is one deactivation (Active false) or reactivation
// (Active true) of an operator account by another operator. IPAddress is
// recorded in the operator ledger.
type OperatorStatusChange struct {
	OperatorID int64
	ActorID    int64
	IPAddress  string
	Active     bool
}

// OperatorStatus is the lifecycle of operator accounts of team members who
// left (#4047), for the operator management screens and the hourly cleanup.
//
// A deactivation locks the account and ends its sessions at once; an
// operator cannot deactivate their own account. A reactivation lets the
// operator sign in again. Both record an entry in the operator ledger and
// refuse with ErrOperatorNotFound or ErrOperatorSelfDeactivation.
// DeleteDeactivatedOperators removes the accounts deactivated before
// 1 January of last year (Berlin): by then the ledger, which keeps an entry
// until the end of the following year, has lost every entry of the account.
type OperatorStatus interface {
	ChangeOperatorStatus(ctx context.Context, change OperatorStatusChange) (Operator, error)
	DeleteDeactivatedOperators(ctx context.Context) (int, error)
}
