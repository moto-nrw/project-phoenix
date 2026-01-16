package devices

import "net/http"

// ListDevicesHandler returns the listDevices handler for testing.
func (rs *Resource) ListDevicesHandler() http.HandlerFunc { return rs.listDevices }

// GetDeviceHandler returns the getDevice handler for testing.
func (rs *Resource) GetDeviceHandler() http.HandlerFunc { return rs.getDevice }

// GetDeviceByDeviceIDHandler returns the getDeviceByDeviceID handler for testing.
func (rs *Resource) GetDeviceByDeviceIDHandler() http.HandlerFunc { return rs.getDeviceByDeviceID }

// CreateDeviceHandler returns the createDevice handler for testing.
func (rs *Resource) CreateDeviceHandler() http.HandlerFunc { return rs.createDevice }

// UpdateDeviceHandler returns the updateDevice handler for testing.
func (rs *Resource) UpdateDeviceHandler() http.HandlerFunc { return rs.updateDevice }

// DeleteDeviceHandler returns the deleteDevice handler for testing.
func (rs *Resource) DeleteDeviceHandler() http.HandlerFunc { return rs.deleteDevice }

// UpdateDeviceStatusHandler returns the updateDeviceStatus handler for testing.
func (rs *Resource) UpdateDeviceStatusHandler() http.HandlerFunc { return rs.updateDeviceStatus }

// PingDeviceHandler returns the pingDevice handler for testing.
func (rs *Resource) PingDeviceHandler() http.HandlerFunc { return rs.pingDevice }

// GetDevicesByTypeHandler returns the getDevicesByType handler for testing.
func (rs *Resource) GetDevicesByTypeHandler() http.HandlerFunc { return rs.getDevicesByType }

// GetDevicesByStatusHandler returns the getDevicesByStatus handler for testing.
func (rs *Resource) GetDevicesByStatusHandler() http.HandlerFunc { return rs.getDevicesByStatus }

// GetDevicesByRegisteredByHandler returns the getDevicesByRegisteredBy handler for testing.
func (rs *Resource) GetDevicesByRegisteredByHandler() http.HandlerFunc {
	return rs.getDevicesByRegisteredBy
}

// GetActiveDevicesHandler returns the getActiveDevices handler for testing.
func (rs *Resource) GetActiveDevicesHandler() http.HandlerFunc { return rs.getActiveDevices }

// GetDevicesRequiringMaintenanceHandler returns the getDevicesRequiringMaintenance handler for testing.
func (rs *Resource) GetDevicesRequiringMaintenanceHandler() http.HandlerFunc {
	return rs.getDevicesRequiringMaintenance
}

// GetOfflineDevicesHandler returns the getOfflineDevices handler for testing.
func (rs *Resource) GetOfflineDevicesHandler() http.HandlerFunc { return rs.getOfflineDevices }

// GetDeviceStatisticsHandler returns the getDeviceStatistics handler for testing.
func (rs *Resource) GetDeviceStatisticsHandler() http.HandlerFunc { return rs.getDeviceStatistics }

// DetectNewDevicesHandler returns the detectNewDevices handler for testing.
func (rs *Resource) DetectNewDevicesHandler() http.HandlerFunc { return rs.detectNewDevices }

// ScanNetworkHandler returns the scanNetwork handler for testing.
func (rs *Resource) ScanNetworkHandler() http.HandlerFunc { return rs.scanNetwork }
