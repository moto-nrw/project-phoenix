package definitions

// Tab definitions are registered here.
// Use settings.MustRegisterTab to define tabs that organize settings in the UI.
//
// Example:
//
//	settings.MustRegisterTab(settings.TabDefinition{
//		Key:                "general",
//		Name:               "Allgemein",
//		Icon:               "cog",
//		DisplayOrder:       0,
//		RequiredPermission: "",  // Empty = visible to all authenticated users
//	})
//
// Tab properties:
//   - Key: Unique identifier (used in setting definitions)
//   - Name: Display name in the UI
//   - Icon: Icon name (lucide icons)
//   - DisplayOrder: Sort order (lower = first)
//   - RequiredPermission: Permission needed to see this tab (empty = all users)

func init() {
	// Register tab definitions here
}
