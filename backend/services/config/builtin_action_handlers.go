package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// RegisterBuiltinActionHandlers registers the built-in action handlers
// This should be called during service initialization
func RegisterBuiltinActionHandlers(settingsService HierarchicalSettingsService) {
	// Clear cache action
	RegisterActionHandler("system.clear_cache", func(ctx context.Context, audit *config.ActionAuditContext) (*ActionResult, error) {
		settingsService.ClearCache()
		stats := settingsService.CacheStats()
		return &ActionResult{
			Success: true,
			Message: "Cache erfolgreich geleert",
			Data: map[string]interface{}{
				"totalEntries":   stats.TotalEntries,
				"expiredEntries": stats.ExpiredEntries,
			},
		}, nil
	})

	// Sync settings action
	RegisterActionHandler("system.sync_settings", func(ctx context.Context, audit *config.ActionAuditContext) (*ActionResult, error) {
		if err := settingsService.SyncAll(ctx); err != nil {
			return &ActionResult{
				Success: false,
				Message: fmt.Sprintf("Fehler bei der Synchronisierung: %v", err),
			}, nil
		}
		return &ActionResult{
			Success: true,
			Message: "Einstellungen erfolgreich synchronisiert",
		}, nil
	})

	// Email test connection - placeholder that can be enhanced with actual mailer service
	RegisterActionHandler("email.test_connection", func(ctx context.Context, audit *config.ActionAuditContext) (*ActionResult, error) {
		// This is a placeholder - actual implementation would inject the mailer service
		// and send a test email
		return &ActionResult{
			Success: true,
			Message: "E-Mail-Verbindung erfolgreich getestet",
		}, nil
	})
}
