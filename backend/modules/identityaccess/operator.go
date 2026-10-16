package identityaccess

import (
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
}

// OperatorDeletionDue is when the hourly cleanup deletes an account
// deactivated at deactivatedAt: 1 January (Berlin) of the second year after
// it. By then the operator ledger, which keeps an entry until the end of the
// following year, holds no entry of the account any more (#4047).
func OperatorDeletionDue(deactivatedAt time.Time) time.Time {
	return time.Date(deactivatedAt.In(calendar.Berlin).Year()+2, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
}
