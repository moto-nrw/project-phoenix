package devicefleet

import (
	"context"
	"errors"
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

// FindUnregisteredTagScan returns one scan visible to the caller with its
// device identity, or ErrUnregisteredTagScanNotFound.
func (m *Module) FindUnregisteredTagScan(ctx context.Context, id int64) (UnregisteredTagScan, error) {
	if id <= 0 {
		return UnregisteredTagScan{}, m.reject("find_unregistered_tag_scan", ErrInvalidUnregisteredTagScan)
	}
	return m.engine.FindUnregisteredTagScan(ctx, id)
}

// ListUnregisteredTagScans returns the scans matching filter, newest first.
// A caller without an ambient tenant sees every tenant it may read.
func (m *Module) ListUnregisteredTagScans(ctx context.Context, filter UnregisteredTagScanFilter) ([]UnregisteredTagScan, error) {
	for _, id := range filter.TenantIDs {
		if id <= 0 {
			return nil, m.reject("list_unregistered_tag_scans", ErrInvalidUnregisteredTagScan)
		}
	}
	if filter.Limit < 0 {
		return nil, m.reject("list_unregistered_tag_scans", ErrInvalidUnregisteredTagScan)
	}
	return m.engine.ListUnregisteredTagScans(ctx, filter)
}

// RecordUnregisteredTagScan appends one scan for the caller's tenant.
func (m *Module) RecordUnregisteredTagScan(ctx context.Context, input RecordUnregisteredTagScan) (UnregisteredTagScan, error) {
	return m.engine.RecordUnregisteredTagScan(ctx, input)
}

// ResolveUnregisteredTagScan marks one open scan as handled. A missing scan
// reports ErrUnregisteredTagScanNotFound; a handled one reports
// ErrUnregisteredTagScanResolved and is never stamped twice.
func (m *Module) ResolveUnregisteredTagScan(ctx context.Context, input ResolveUnregisteredTagScan) (UnregisteredTagScan, error) {
	if input.ID <= 0 || input.OperatorID <= 0 {
		return UnregisteredTagScan{}, m.reject("resolve_unregistered_tag_scan", ErrInvalidUnregisteredTagScan)
	}
	return m.engine.ResolveUnregisteredTagScan(ctx, input)
}

// DeleteExpiredUnregisteredTagScans removes the caller tenant's scans older
// than cutoff.
func (m *Module) DeleteExpiredUnregisteredTagScans(ctx context.Context, cutoff time.Time) (int64, error) {
	if cutoff.IsZero() {
		return 0, m.reject("delete_expired_unregistered_tag_scans", ErrInvalidUnregisteredTagScan)
	}
	return m.engine.DeleteExpiredUnregisteredTagScans(ctx, cutoff)
}
