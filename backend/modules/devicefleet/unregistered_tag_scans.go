package devicefleet

import (
	"context"
	"errors"
	"strings"
	"time"
)

type unregisteredTagScanNotFoundError struct{}

func (unregisteredTagScanNotFoundError) Error() string       { return "unregistered tag scan not found" }
func (unregisteredTagScanNotFoundError) RepositoryNotFound() {}

// Stable unregistered-tag-scan errors this owner returns.
var (
	ErrUnregisteredTagScanNotFound error = unregisteredTagScanNotFoundError{}
	ErrInvalidUnregisteredTagScan        = errors.New("invalid unregistered tag scan")
	ErrUnregisteredTagScanResolved       = errors.New("unregistered tag scan already resolved")
)

// UnregisteredTagScanRetentionDays is how long a scan stays reviewable. The
// audit retention function refuses an earlier cutoff for tenant roles.
const UnregisteredTagScanRetentionDays = 90

// UnregisteredTagScan is one RFID tag UID a device scanned while no person
// was assigned to it. DeviceIdentifier and DeviceName come from this owner's
// device inventory and stay unset when the device is gone or belongs to
// another tenant.
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

// RecordUnregisteredTagScan appends one scan for the caller's tenant. A zero
// ScannedAt is stamped by the owner's clock.
type RecordUnregisteredTagScan struct {
	TagUID    string
	DeviceID  *int64
	ScannedAt time.Time
}

// ResolveUnregisteredTagScan marks one open scan as handled by an operator.
// A blank note is stored as no note.
type ResolveUnregisteredTagScan struct {
	ID         int64
	OperatorID int64
	Note       *string
}

// UnregisteredTagScanFilter restricts a listing. A nil TenantIDs means every
// tenant the caller may read; an empty one matches nothing. Limit falls back
// to the owner's default of 500 newest scans.
type UnregisteredTagScanFilter struct {
	TenantIDs      []int64
	UnresolvedOnly bool
	Limit          int
}

// UnregisteredTagScanQuery answers the operator review reads.
type UnregisteredTagScanQuery interface {
	FindUnregisteredTagScan(context.Context, int64) (UnregisteredTagScan, error)
	ListUnregisteredTagScans(context.Context, UnregisteredTagScanFilter) ([]UnregisteredTagScan, error)
}

// UnregisteredTagScanCommand owns every write to audit.unregistered_tag_scans.
type UnregisteredTagScanCommand interface {
	RecordUnregisteredTagScan(context.Context, RecordUnregisteredTagScan) (UnregisteredTagScan, error)
	ResolveUnregisteredTagScan(context.Context, ResolveUnregisteredTagScan) (UnregisteredTagScan, error)
	// DeleteExpiredUnregisteredTagScans removes the caller tenant's scans
	// scanned before cutoff and reports how many rows went.
	DeleteExpiredUnregisteredTagScans(context.Context, time.Time) (int64, error)
}

// UnregisteredTagScanRetention removes scans after the owner's fixed
// retention window and reports that window with the deleted count.
type UnregisteredTagScanRetention interface {
	DeleteUnregisteredTagScansAfterRetention(context.Context) (deleted int, retentionDays int, err error)
}

var (
	ErrTagScanIDRequired         = errors.New("scan ID is required")
	ErrTagScanOperatorIDRequired = errors.New("operator ID is required")
)

// Normalize trims the tag UID and stamps a missing scan time.
func (r *RecordUnregisteredTagScan) Normalize(now time.Time) {
	r.TagUID = strings.TrimSpace(r.TagUID)
	if r.ScannedAt.IsZero() {
		r.ScannedAt = now
	}
}

// Validate checks whether the scan can be stored.
func (r RecordUnregisteredTagScan) Validate() error {
	if strings.TrimSpace(r.TagUID) == "" {
		return ErrInvalidUnregisteredTagScan
	}
	if r.DeviceID != nil && *r.DeviceID <= 0 {
		return ErrInvalidUnregisteredTagScan
	}
	return nil
}
