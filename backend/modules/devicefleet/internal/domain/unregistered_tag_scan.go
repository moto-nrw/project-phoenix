package domain

import (
	"errors"
	"strings"
	"time"
)

// Stable unregistered-tag-scan errors this owner's application raises.
var (
	ErrUnregisteredTagScanNotFound = errors.New("unregistered tag scan not found")
	ErrUnregisteredTagScanInvalid  = errors.New("invalid unregistered tag scan")
	ErrUnregisteredTagScanResolved = errors.New("unregistered tag scan already resolved")
	ErrTenantRequired              = errors.New("tenant is required")
)

// DefaultUnregisteredTagScanLimit bounds an operator listing that names no
// limit of its own.
const DefaultUnregisteredTagScanLimit = 500

// UnregisteredTagScan is one RFID tag UID a device scanned while no person
// was assigned to it. DeviceIdentifier and DeviceName are resolved from the
// owner's device store and stay unset when the device is gone or belongs to
// another tenant, exactly like the retired LEFT JOIN.
type UnregisteredTagScan struct {
	ID                   int64
	TenantID             int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	TagUID               string
	DeviceID             *int64
	ScannedAt            time.Time
	ResolvedAt           *time.Time
	ResolvedByOperatorID *int64
	ResolutionNote       *string
	DeviceIdentifier     *string
	DeviceName           *string
}

// RecordUnregisteredTagScan appends one scan for the caller's tenant.
type RecordUnregisteredTagScan struct {
	TagUID    string
	DeviceID  *int64
	ScannedAt time.Time
}

// Normalize trims the tag UID and stamps a missing scan time.
func (r *RecordUnregisteredTagScan) Normalize(now time.Time) {
	r.TagUID = strings.TrimSpace(r.TagUID)
	if r.ScannedAt.IsZero() {
		r.ScannedAt = now
	}
}

// Validate reports whether the scan can be stored.
func (r RecordUnregisteredTagScan) Validate() error {
	if strings.TrimSpace(r.TagUID) == "" {
		return ErrUnregisteredTagScanInvalid
	}
	if r.DeviceID != nil && *r.DeviceID <= 0 {
		return ErrUnregisteredTagScanInvalid
	}
	return nil
}

// ResolveUnregisteredTagScan marks one open scan as handled by an operator.
type ResolveUnregisteredTagScan struct {
	ID         int64
	OperatorID int64
	Note       *string
	ResolvedAt time.Time
}

// Normalize drops an empty note and stamps a missing resolution time.
func (r *ResolveUnregisteredTagScan) Normalize(now time.Time) {
	if r.Note != nil {
		trimmed := strings.TrimSpace(*r.Note)
		if trimmed == "" {
			r.Note = nil
		} else {
			r.Note = &trimmed
		}
	}
	if r.ResolvedAt.IsZero() {
		r.ResolvedAt = now
	}
}

// Validate reports whether the resolution names a scan and an operator.
func (r ResolveUnregisteredTagScan) Validate() error {
	if r.ID <= 0 || r.OperatorID <= 0 {
		return ErrUnregisteredTagScanInvalid
	}
	return nil
}

// UnregisteredTagScanFilter restricts a listing. A nil TenantIDs means every
// tenant the caller may read; an empty one matches nothing.
type UnregisteredTagScanFilter struct {
	TenantIDs      []int64
	UnresolvedOnly bool
	Limit          int
}
