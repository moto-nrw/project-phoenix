package devicefleet

import (
	"errors"
)

func (d *Device) Validate() error {
	if d.DeviceID == "" {
		return errors.New("device ID is required")
	}

	if d.DeviceType == "" {
		return errors.New("device type is required")
	}

	// Validate the status
	if d.Status == "" {
		d.Status = DeviceStatusActive
	} else if !ValidDeviceStatus(d.Status) {
		return errors.New("invalid device status")
	}

	return nil
}
