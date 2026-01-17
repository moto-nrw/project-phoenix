package config

import "net/http"

// =============================================================================
// EXPORTED HANDLERS FOR TESTING
// =============================================================================

// ListSettingsHandler returns the listSettings handler for testing.
func (rs *Resource) ListSettingsHandler() http.HandlerFunc { return rs.listSettings }

// GetSettingHandler returns the getSetting handler for testing.
func (rs *Resource) GetSettingHandler() http.HandlerFunc { return rs.getSetting }

// GetSettingByKeyHandler returns the getSettingByKey handler for testing.
func (rs *Resource) GetSettingByKeyHandler() http.HandlerFunc { return rs.getSettingByKey }

// GetSettingsByCategoryHandler returns the getSettingsByCategory handler for testing.
func (rs *Resource) GetSettingsByCategoryHandler() http.HandlerFunc { return rs.getSettingsByCategory }

// GetSystemStatusHandler returns the getSystemStatus handler for testing.
func (rs *Resource) GetSystemStatusHandler() http.HandlerFunc { return rs.getSystemStatus }

// GetDefaultSettingsHandler returns the getDefaultSettings handler for testing.
func (rs *Resource) GetDefaultSettingsHandler() http.HandlerFunc { return rs.getDefaultSettings }

// CreateSettingHandler returns the createSetting handler for testing.
func (rs *Resource) CreateSettingHandler() http.HandlerFunc { return rs.createSetting }

// UpdateSettingHandler returns the updateSetting handler for testing.
func (rs *Resource) UpdateSettingHandler() http.HandlerFunc { return rs.updateSetting }

// UpdateSettingValueHandler returns the updateSettingValue handler for testing.
func (rs *Resource) UpdateSettingValueHandler() http.HandlerFunc { return rs.updateSettingValue }

// DeleteSettingHandler returns the deleteSetting handler for testing.
func (rs *Resource) DeleteSettingHandler() http.HandlerFunc { return rs.deleteSetting }

// ImportSettingsHandler returns the importSettings handler for testing.
func (rs *Resource) ImportSettingsHandler() http.HandlerFunc { return rs.importSettings }

// InitializeDefaultsHandler returns the initializeDefaults handler for testing.
func (rs *Resource) InitializeDefaultsHandler() http.HandlerFunc { return rs.initializeDefaults }

// GetRetentionSettingsHandler returns the getRetentionSettings handler for testing.
func (rs *Resource) GetRetentionSettingsHandler() http.HandlerFunc { return rs.getRetentionSettings }

// UpdateRetentionSettingsHandler returns the updateRetentionSettings handler for testing.
func (rs *Resource) UpdateRetentionSettingsHandler() http.HandlerFunc {
	return rs.updateRetentionSettings
}

// TriggerRetentionCleanupHandler returns the triggerRetentionCleanup handler for testing.
func (rs *Resource) TriggerRetentionCleanupHandler() http.HandlerFunc {
	return rs.triggerRetentionCleanup
}

// GetRetentionStatsHandler returns the getRetentionStats handler for testing.
func (rs *Resource) GetRetentionStatsHandler() http.HandlerFunc { return rs.getRetentionStats }
