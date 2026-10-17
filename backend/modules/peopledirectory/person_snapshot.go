package peopledirectory

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// DirectoryPersonSnapshot is the complete person read, including metadata and
// already-loaded account/card projections, without a persistence mapping.
type DirectoryPersonSnapshot struct {
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
	Account   *PersonAccountSnapshot
	RFIDCard  *PersonRFIDSnapshot
}
type PersonAccountSnapshot struct {
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
type PersonRFIDSnapshot struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	TenantID  int64
	Active    bool
}

// DirectoryPersonReads retains the existing tenant-scoped identity lookup.
type DirectoryPersonReads interface {
	PersonSnapshotByID(context.Context, int64) (*DirectoryPersonSnapshot, error)
}
