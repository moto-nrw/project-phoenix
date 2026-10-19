package care

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Person is the complete directory snapshot used by the workflow's identity
// field-state reads. It owns no persistence or account lifecycle.
type Person struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	FirstName string
	LastName  string
	Birthday  *calendar.Date
	TagID     *string
	AccountID *int64
	DeletedAt *time.Time
	Account   *PersonAccount
	RFIDCard  *PersonRFIDCard
}
type PersonAccount struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Email         string
	Username      *string
	Avatar        string
	Active        bool
	IsPasswordOTP bool
	LastLogin     *time.Time
}
type PersonRFIDCard struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	TenantID  int64
	Active    bool
}
