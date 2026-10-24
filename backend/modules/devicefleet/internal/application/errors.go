package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// publicError keeps the owner's stable error values and display reasons intact
// when transaction or query diagnostics wrap them.
func publicError(err error) error {
	for _, sentinel := range []error{
		devicefleet.ErrDeviceNotFound, devicefleet.ErrDuplicateDeviceID,
		devicefleet.ErrDeviceProtected, devicefleet.ErrInvalidDevice,
		devicefleet.ErrDisplayNotFound, devicefleet.ErrDisplayInactive,
	} {
		if errors.Is(err, sentinel) {
			return sentinel
		}
	}
	if errors.Is(err, devicefleet.ErrInvalidDisplayInput) {
		var invalid *devicefleet.InvalidDisplayError
		if errors.As(err, &invalid) {
			return &devicefleet.InvalidDisplayError{Reason: invalid.Reason}
		}
		return devicefleet.ErrInvalidDisplayInput
	}
	for _, sentinel := range []error{
		devicefleet.ErrUnregisteredTagScanNotFound, devicefleet.ErrUnregisteredTagScanResolved,
		devicefleet.ErrInvalidUnregisteredTagScan, devicefleet.ErrTenantRequired,
	} {
		if errors.Is(err, sentinel) {
			return sentinel
		}
	}
	return err
}
