package devicescan

import (
	"context"
	"time"
)

// PrincipalDevice is the authenticated kiosk of a request.
type PrincipalDevice struct {
	ID         int64
	TenantID   int64
	DeviceID   string
	DeviceType string
	Name       *string
	Status     string
	LastSeen   *time.Time
	Active     bool
}

// DevicePrincipal reads the verified kiosk principal supplied by the root.
type DevicePrincipal func(context.Context) (*PrincipalDevice, bool)

func (read DevicePrincipal) Device(ctx context.Context) (*PrincipalDevice, bool) { return read(ctx) }

// PrincipalReader reads the identity the device authentication bound.
type PrincipalReader interface {
	Device(ctx context.Context) (*PrincipalDevice, bool)
}
