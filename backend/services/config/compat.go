package config

// Compat wrapper for migrating existing ConfigService callers to the new SettingsService.
//
// The existing Service interface (config_service.go) operates on the old config.settings
// table with key/value/category columns. The new SettingsService operates on
// config.setting_values with the registry-driven approach.
//
// Both services coexist during the migration period:
// - Old Service: handles existing config.settings reads (timeout settings, system flags)
// - New SettingsService: handles the 12 new tenant-scoped settings from .env migration
//
// When ready to migrate callers (Phase 2+), add wrapper methods here that delegate
// old-style calls like GetStringValue("session_timeout") to the new
// SettingsService.Resolve(ctx, "operations.session_timeout") with key mapping.
//
// TODO: Implement wrapper when migrating scheduler and IoT callers from old config to new settings.
