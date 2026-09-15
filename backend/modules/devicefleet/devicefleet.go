// Package devicefleet is the public Device Fleet capability. It owns
// iot.devices, display.displays, and audit.unregistered_tag_scans and answers
// every device, info-point, and unregistered-scan question for other owners
// without leaking those tables into their repositories.
package devicefleet

import (
	"context"
	"errors"
	"time"
)

type deviceNotFoundError struct{}

func (deviceNotFoundError) Error() string       { return "device not found" }
func (deviceNotFoundError) RepositoryNotFound() {}

type displayNotFoundError struct{}

func (displayNotFoundError) Error() string       { return "display not found" }
func (displayNotFoundError) RepositoryNotFound() {}

// Stable errors this owner returns. Callers match on these values; the
// German strings are user-facing contract text.
var (
	ErrDeviceNotFound        error = deviceNotFoundError{}
	ErrInvalidDevice               = errors.New("invalid device data")
	ErrDuplicateDeviceID           = errors.New("duplicate device id")
	ErrDeviceProtected             = errors.New("device is protected")
	ErrDisplayNotFound       error = displayNotFoundError{}
	ErrDisplayInactive             = errors.New("display inactive")
	ErrInvalidDisplayInput         = errors.New("invalid display input")
	ErrTenantRequired              = errors.New("devicefleet: tenant is required")
	ErrDashboardTokenMissing       = errors.New("devicefleet: display token is required")
)

// InvalidDisplayError names which display rule a request broke. Its message
// is the stable text the admin UI renders.
type InvalidDisplayError struct{ Reason string }

func (e *InvalidDisplayError) Error() string { return ErrInvalidDisplayInput.Error() + ": " + e.Reason }

// Unwrap keeps errors.Is(err, ErrInvalidDisplayInput) true for every reason.
func (e *InvalidDisplayError) Unwrap() error { return ErrInvalidDisplayInput }

// DeviceStatus is the lifecycle state of one fleet device.
type DeviceStatus string

// The four device states the fleet recognises.
const (
	DeviceStatusActive      DeviceStatus = "active"
	DeviceStatusInactive    DeviceStatus = "inactive"
	DeviceStatusMaintenance DeviceStatus = "maintenance"
	DeviceStatusOffline     DeviceStatus = "offline"
)

// DeviceTypeVirtual marks devices that exist only as an attribution target.
const DeviceTypeVirtual = "virtual"

// WebManualDeviceID names the reserved per-tenant device that records manual
// web check-ins. It is excluded from listings and cannot be edited.
const WebManualDeviceID = "WEB-MANUAL-001"

// ValidDeviceStatus reports whether status is one of the four known states.
func ValidDeviceStatus(status DeviceStatus) bool {
	switch status {
	case DeviceStatusActive, DeviceStatusInactive, DeviceStatusMaintenance, DeviceStatusOffline:
		return true
	default:
		return false
	}
}

// Device is the fleet's view of one registered device. RoomName is resolved
// through the Facilities owner and is empty when the device has no room or
// the room belongs to another tenant.
type Device struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeviceID              string
	DeviceType            string
	Name                  *string
	Status                DeviceStatus
	APIKey                *string
	LastSeen              *time.Time
	RegisteredByID        *int64
	RoomID                *int64
	ArchivedAt            *time.Time
	TransferredToDeviceID *int64
	RoomName              *string
}

// IsActive reports whether the device is in the active state.
func (d Device) IsActive() bool { return d.Status == DeviceStatusActive }

// IsOffline reports whether the device is in the offline state.
func (d Device) IsOffline() bool { return d.Status == DeviceStatusOffline }

// HasAPIKey reports whether the device carries a usable API key.
func (d Device) HasAPIKey() bool { return d.APIKey != nil && *d.APIKey != "" }

// CreateDevice registers one device. An empty APIKey is minted by the owner.
type CreateDevice struct {
	DeviceID       string
	DeviceType     string
	Name           *string
	Status         DeviceStatus
	APIKey         *string
	LastSeen       *time.Time
	RegisteredByID *int64
	RoomID         *int64
}

// UpdateDevice replaces every writable column of one device.
type UpdateDevice struct {
	ID                    int64
	DeviceID              string
	DeviceType            string
	Name                  *string
	Status                DeviceStatus
	APIKey                *string
	LastSeen              *time.Time
	RegisteredByID        *int64
	RoomID                *int64
	ArchivedAt            *time.Time
	TransferredToDeviceID *int64
}

// DeviceFilter restricts a device listing. Every field is optional.
type DeviceFilter struct {
	DeviceIDContains  *string
	NameContains      *string
	Status            *DeviceStatus
	DeviceType        *string
	ExcludeDeviceType *string
	ExcludeDeviceID   *string
	SeenAfter         *time.Time
	SeenBefore        *time.Time
	RoomID            *int64
	RegisteredByID    *int64
	HasName           *bool
}

// Display is one registered info-point screen. Its raw access token is never
// stored or returned by a read.
type Display struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	IsActive  bool
}

// DeviceQuery answers device reads for every owner.
type DeviceQuery interface {
	FindDevice(context.Context, int64) (Device, error)
	FindDeviceForUpdate(context.Context, int64) (Device, error)
	FindDeviceByDeviceID(context.Context, string) (Device, error)
	FindDeviceByAPIKey(context.Context, string) (Device, error)
	ListDevices(context.Context, DeviceFilter) ([]Device, error)
	ListDevicesByID(context.Context, []int64) ([]Device, error)
	ListOfflineDevices(context.Context, time.Duration) ([]Device, error)
	CountDevicesByType(context.Context) (map[string]int, error)
	// CountDevicesByTenant and ListDevicesByTenant answer the operator
	// dashboard's cross-tenant questions without a foreign join.
	CountDevicesByTenant(context.Context) (map[int64]int, error)
	ListDevicesByTenant(context.Context, []int64) ([]Device, error)

	// DeviceOnlineWindow and the two predicates below are the owner's
	// online/offline decision (#586, Rule 12): the row holds last_seen, this
	// owner holds the rule and the per-tenant window.
	DeviceOnlineWindow(context.Context) time.Duration
	IsDeviceOnline(context.Context, Device) bool
	IsDeviceOnlineAt(context.Context, Device, time.Time) bool
}

// DeviceCommand owns every write to iot.devices.
type DeviceCommand interface {
	CreateDevice(context.Context, CreateDevice) (Device, error)
	UpdateDevice(context.Context, UpdateDevice) (Device, error)
	UpdateDeviceColumns(context.Context, UpdateDevice, []string) (int64, error)
	DeleteDevice(context.Context, int64) error
	UpdateDeviceStatus(context.Context, string, DeviceStatus) error
	UpdateDeviceLastSeen(context.Context, int64, time.Time) error
	UpdateDeviceRoom(context.Context, int64, int64) error
}

// DisplayQuery answers info-point reads.
type DisplayQuery interface {
	ListDisplays(context.Context) ([]Display, error)
	// Dashboard resolves a raw display token to the public aggregate.
	// It returns ErrDisplayNotFound for unknown, disabled, or offboarded
	// screens and ErrDisplayInactive for a deactivated one.
	Dashboard(context.Context, string) (Dashboard, error)
}

// DisplayCommand owns every write to display.displays.
type DisplayCommand interface {
	CreateDisplay(context.Context, string) (Display, string, error)
	UpdateDisplay(context.Context, int64, *string, *bool) (Display, error)
	RegenerateDisplayToken(context.Context, int64) (string, error)
	DeleteDisplay(context.Context, int64) error
}

// Query is every read this owner answers.
type Query interface {
	DeviceQuery
	DisplayQuery
	UnregisteredTagScanQuery
}

// Command is every write this owner performs.
type Command interface {
	DeviceCommand
	DisplayCommand
	UnregisteredTagScanCommand
}

// Capability is the full Device Fleet contract.
type Capability interface {
	Query
	Command
}

// engine is the composed runtime behind the facade. Device authentication
// and the public dashboard route resolve a device or display before any
// tenant is known, so this owner never demands an ambient tenant of its own:
// the store applies the caller's tenant as a predicate when there is one and
// RLS remains the boundary when there is not.
type engine interface {
	ObserveRejection(string, time.Duration, error)

	FindDevice(context.Context, int64) (Device, error)
	FindDeviceForUpdate(context.Context, int64) (Device, error)
	FindDeviceByDeviceID(context.Context, string) (Device, error)
	FindDeviceByAPIKey(context.Context, string) (Device, error)
	ListDevices(context.Context, DeviceFilter) ([]Device, error)
	ListDevicesByID(context.Context, []int64) ([]Device, error)
	ListOfflineDevices(context.Context, time.Duration) ([]Device, error)
	CountDevicesByType(context.Context) (map[string]int, error)
	CountDevicesByTenant(context.Context) (map[int64]int, error)
	ListDevicesByTenant(context.Context, []int64) ([]Device, error)
	DeviceOnlineWindow(context.Context) time.Duration
	IsDeviceOnlineAt(context.Context, Device, time.Time) bool
	Now() time.Time
	CreateDevice(context.Context, CreateDevice) (Device, error)
	UpdateDevice(context.Context, UpdateDevice) (Device, error)
	UpdateDeviceColumns(context.Context, UpdateDevice, []string) (int64, error)
	DeleteDevice(context.Context, int64) error
	UpdateDeviceStatus(context.Context, string, DeviceStatus) error
	UpdateDeviceLastSeen(context.Context, int64, time.Time) error
	UpdateDeviceRoom(context.Context, int64, int64) error

	ListDisplays(context.Context) ([]Display, error)
	CreateDisplay(context.Context, string) (Display, string, error)
	UpdateDisplay(context.Context, int64, *string, *bool) (Display, error)
	RegenerateDisplayToken(context.Context, int64) (string, error)
	DeleteDisplay(context.Context, int64) error
	Dashboard(context.Context, string) (Dashboard, error)

	FindUnregisteredTagScan(context.Context, int64) (UnregisteredTagScan, error)
	ListUnregisteredTagScans(context.Context, UnregisteredTagScanFilter) ([]UnregisteredTagScan, error)
	RecordUnregisteredTagScan(context.Context, RecordUnregisteredTagScan) (UnregisteredTagScan, error)
	ResolveUnregisteredTagScan(context.Context, ResolveUnregisteredTagScan) (UnregisteredTagScan, error)
	DeleteExpiredUnregisteredTagScans(context.Context, time.Time) (int64, error)
}

// Module is the public facade every caller holds.
type Module struct{ engine engine }

// NewModule wraps a composed engine.
func NewModule(engine engine) *Module {
	if engine == nil {
		panic("devicefleet: engine is required")
	}
	return &Module{engine: engine}
}

// ErrorCode maps an owner error to the stable label the runtime evidence
// groups operations by.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrDeviceNotFound):
		return "device_not_found"
	case errors.Is(err, ErrDuplicateDeviceID):
		return "duplicate_device_id"
	case errors.Is(err, ErrDeviceProtected):
		return "device_protected"
	case errors.Is(err, ErrInvalidDevice):
		return "invalid_device"
	case errors.Is(err, ErrDisplayNotFound):
		return "display_not_found"
	case errors.Is(err, ErrDisplayInactive):
		return "display_inactive"
	case errors.Is(err, ErrInvalidDisplayInput):
		return "invalid_display"
	case errors.Is(err, ErrDashboardTokenMissing):
		return "display_token_missing"
	case errors.Is(err, ErrUnregisteredTagScanNotFound):
		return "unregistered_tag_scan_not_found"
	case errors.Is(err, ErrUnregisteredTagScanResolved):
		return "unregistered_tag_scan_resolved"
	case errors.Is(err, ErrInvalidUnregisteredTagScan):
		return "invalid_unregistered_tag_scan"
	case errors.Is(err, ErrTenantRequired):
		return "tenant_required"
	default:
		return "internal"
	}
}

func (m *Module) reject(operation string, err error) error {
	m.engine.ObserveRejection(operation, 0, err)
	return err
}
