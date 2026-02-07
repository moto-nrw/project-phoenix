package definitions

// Action definitions are registered here.
// Use settings.MustRegister with Type: config.ValueTypeAction
// and the action-specific fields (ActionEndpoint, ActionConfirmationTitle, etc.)
//
// Example:
//
//	settings.MustRegister(settings.Definition{
//		Key:          "system.clear_cache",
//		Type:         config.ValueTypeAction,
//		Category:     "maintenance",
//		Tab:          "system",
//		DisplayOrder: 100,
//		Label:        "Cache leeren",
//		Description:  "Löscht den internen Anwendungscache",
//		Scopes:       []config.Scope{config.ScopeSystem},
//		ViewPerm:     "config:read",
//		EditPerm:     "config:manage",
//		Icon:         "trash",
//		ActionEndpoint:             "/api/settings/actions/system.clear_cache/execute",
//		ActionMethod:               "POST",
//		ActionRequiresConfirmation: true,
//		ActionConfirmationTitle:    "Cache leeren?",
//		ActionConfirmationMessage:  "Alle zwischengespeicherten Daten werden gelöscht.",
//		ActionConfirmationButton:   "Cache leeren",
//		ActionSuccessMessage:       "Cache erfolgreich geleert",
//		ActionErrorMessage:         "Fehler beim Leeren des Caches",
//		ActionIsDangerous:          false,
//	})
//
// Don't forget to register the corresponding handler in
// services/config/builtin_action_handlers.go

func init() {
	// Register action definitions here
}
