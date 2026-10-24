package peopledirectory

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// DirectoryPersonSnapshot is the complete person read, including metadata and
// already-loaded account/card projections, without a persistence mapping.
type DirectoryPersonSnapshot struct {
	ID        int64                  `json:"id"`
	TenantID  int64                  `json:"tenant_id"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	FirstName string                 `json:"first_name"`
	LastName  string                 `json:"last_name"`
	Birthday  *calendar.Date         `json:"birthday,omitempty"`
	TagID     *string                `json:"tag_id,omitempty"`
	AccountID *int64                 `json:"account_id,omitempty"`
	DeletedAt *time.Time             `json:"-"`
	Account   *PersonAccountSnapshot `json:"account,omitempty"`
	RFIDCard  *PersonRFIDSnapshot    `json:"rfid_card,omitempty"`
}
type PersonAccountSnapshot struct {
	ID            int64      `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Email         string     `json:"email"`
	Username      *string    `json:"username,omitempty"`
	Avatar        string     `json:"avatar,omitempty"`
	Active        bool       `json:"active"`
	IsPasswordOTP bool       `json:"is_password_otp"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
}
type PersonRFIDSnapshot struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TenantID  int64     `json:"tenant_id"`
	Active    bool      `json:"active"`
}

// DirectoryPersonReads retains the existing tenant-scoped identity lookup.
type DirectoryPersonReads interface {
	PersonSnapshotByID(context.Context, int64) (*DirectoryPersonSnapshot, error)
}
