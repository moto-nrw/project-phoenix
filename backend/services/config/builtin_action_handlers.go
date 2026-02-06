package config

// RegisterBuiltinActionHandlers registers the built-in action handlers.
// This is called during service initialization.
//
// Example handler registration:
//
//	RegisterActionHandler("system.clear_cache", func(ctx context.Context, audit *config.ActionAuditContext) (*ActionResult, error) {
//		settingsService.ClearCache()
//		return &ActionResult{
//			Success: true,
//			Message: "Cache erfolgreich geleert",
//		}, nil
//	})
//
// The corresponding action definition must be registered in
// settings/definitions/actions.go
func RegisterBuiltinActionHandlers(settingsService HierarchicalSettingsService) {
	// Register action handlers here
	// Each handler key must match an action definition key
}
