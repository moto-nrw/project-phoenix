package devicefleet

import (
	"errors"
	"time"
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

func (d *Device) UpdateLastSeen() {
	now := time.Now()
	d.LastSeen = &now
}

func (d *Device) SetStatus(status DeviceStatus) error {
	if !ValidDeviceStatus(status) {
		return errors.New("invalid device status")
	}
	d.Status = status
	return nil
}

func (d *Device) GetLastSeenDuration() *time.Duration {
	if d.LastSeen == nil {
		return nil
	}

	duration := time.Since(*d.LastSeen)
	return &duration
}
