package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
)

// The doubles and owner constructors the external suites route through.
// Those suites compose the typed root, which imports this package, so they
// cannot be internal tests.
type (
	RouterDeviceScan = routerDeviceScan
	RouterStaffClock = routerStaffClock
)

// NewRouterTestResource is the resource with every collaborator doubled.
var NewRouterTestResource = newRouterTestResource

// NewDeviceAdministration is Device Fleet's administration over a fleet.
var NewDeviceAdministration = devicefleet.NewAdministration

// NewSessionLifecycle is the kiosk session lifecycle over its owners.
var NewSessionLifecycle = devicescanCompose.NewSessionLifecycle

// UnavailableMirrorSupervisions fails the trailing supervision read of a
// session start.
type UnavailableMirrorSupervisions struct{}

func (UnavailableMirrorSupervisions) ActiveSupervisions(context.Context, int64) ([]devicescanCompose.Supervision, error) {
	return nil, errors.New("supervision response lookup unavailable")
}
