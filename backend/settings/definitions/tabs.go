package definitions

import (
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// ============================================================
	// SETTINGS TABS - Define UI tabs for settings organization
	// ============================================================

	// General settings - visible to all authenticated users
	settings.MustRegisterTab(settings.TabDefinition{
		Key:          "general",
		Name:         "Allgemein",
		Icon:         "cog",
		DisplayOrder: 0,
	})

	// Email settings - admin only
	settings.MustRegisterTab(settings.TabDefinition{
		Key:                "email",
		Name:               "E-Mail",
		Icon:               "mail",
		DisplayOrder:       20,
		RequiredPermission: "config:manage",
	})

	// Display settings - visible to all users
	settings.MustRegisterTab(settings.TabDefinition{
		Key:          "display",
		Name:         "Anzeige",
		Icon:         "monitor",
		DisplayOrder: 30,
	})

	// Notifications settings - visible to all users
	settings.MustRegisterTab(settings.TabDefinition{
		Key:          "notifications",
		Name:         "Benachrichtigungen",
		Icon:         "bell",
		DisplayOrder: 40,
	})

	// Rooms settings - admin only
	settings.MustRegisterTab(settings.TabDefinition{
		Key:                "rooms",
		Name:               "Räume",
		Icon:               "door-open",
		DisplayOrder:       50,
		RequiredPermission: "config:manage",
	})

	// System settings - admin only
	settings.MustRegisterTab(settings.TabDefinition{
		Key:                "system",
		Name:               "System",
		Icon:               "server",
		DisplayOrder:       90,
		RequiredPermission: "config:manage",
	})

	// Test tab - admin only (for testing features)
	settings.MustRegisterTab(settings.TabDefinition{
		Key:                "test",
		Name:               "Test Features",
		Icon:               "flask",
		DisplayOrder:       100,
		RequiredPermission: "config:manage",
	})
}
