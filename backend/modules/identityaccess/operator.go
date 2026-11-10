package identityaccess

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Operator is the platform login identity of a moto operator. DeactivatedAt
// is set while the account is deactivated (#4047).
type Operator struct {
	ID             int64
	Email          string
	DisplayName    string
	PasswordHash   string
	Active         bool
	LastLogin      *time.Time
	MFAAttempts    int
	MFALockedUntil *time.Time
	DeactivatedAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// CredentialGeneration is the state of the operator's password; every
	// password write moves it on (#6308).
	CredentialGeneration int64
}

// OperatorDeletionDue is when the hourly cleanup deletes an account
// deactivated at deactivatedAt: 1 January (Berlin) of the second year after
// it. By then the operator ledger, which keeps an entry until the end of the
// following year, holds no entry of the account any more (#4047).
func OperatorDeletionDue(deactivatedAt time.Time) time.Time {
	return time.Date(deactivatedAt.In(calendar.Berlin).Year()+2, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
}

// OperatorDirectory reads operator identity rows for the operator dashboard.
// FindOperator returns ErrOperatorNotFound for an unknown operator.
type OperatorDirectory interface {
	FindOperator(ctx context.Context, id int64) (Operator, error)
	ListOperators(ctx context.Context) ([]Operator, error)
}

// OperatorSessionLiveness confirms on every operator request that the
// operator is active and that the refresh family the access token names is
// still live and the operator's own. A refusal is ErrOperatorInactive or
// ErrOperatorNotFound; any other error means the check could not run.
type OperatorSessionLiveness interface {
	VerifyOperatorSession(ctx context.Context, operatorID int64, familyID string) error
}
