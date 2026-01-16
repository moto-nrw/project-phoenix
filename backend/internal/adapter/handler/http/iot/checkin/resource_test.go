package checkin

import "net/http"

// DeviceCheckinHandler returns the deviceCheckin handler for testing.
func (rs *Resource) DeviceCheckinHandler() http.HandlerFunc { return rs.deviceCheckin }

// DevicePingHandler returns the devicePing handler for testing.
func (rs *Resource) DevicePingHandler() http.HandlerFunc { return rs.devicePing }

// DeviceStatusHandler returns the deviceStatus handler for testing.
func (rs *Resource) DeviceStatusHandler() http.HandlerFunc { return rs.deviceStatus }
