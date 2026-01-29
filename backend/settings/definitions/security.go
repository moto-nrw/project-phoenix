package definitions

import (
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// Password security settings
	settings.MustRegister(settings.Definition{
		Key:          "security.password_min_length",
		Type:         config.ValueTypeInt,
		Default:      "8",
		Category:     "password",
		Tab:          "security",
		Label:        "Minimale Passwortlänge",
		Description:  "Mindestanzahl der Zeichen für ein Passwort",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(6, 32),
		DisplayOrder: 100,
	})

	settings.MustRegister(settings.Definition{
		Key:          "security.password_require_uppercase",
		Type:         config.ValueTypeBool,
		Default:      "true",
		Category:     "password",
		Tab:          "security",
		Label:        "Großbuchstaben erforderlich",
		Description:  "Passwörter müssen mindestens einen Großbuchstaben enthalten",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		DisplayOrder: 110,
	})

	settings.MustRegister(settings.Definition{
		Key:          "security.password_require_lowercase",
		Type:         config.ValueTypeBool,
		Default:      "true",
		Category:     "password",
		Tab:          "security",
		Label:        "Kleinbuchstaben erforderlich",
		Description:  "Passwörter müssen mindestens einen Kleinbuchstaben enthalten",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		DisplayOrder: 120,
	})

	settings.MustRegister(settings.Definition{
		Key:          "security.password_require_digit",
		Type:         config.ValueTypeBool,
		Default:      "true",
		Category:     "password",
		Tab:          "security",
		Label:        "Ziffern erforderlich",
		Description:  "Passwörter müssen mindestens eine Ziffer enthalten",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		DisplayOrder: 130,
	})

	settings.MustRegister(settings.Definition{
		Key:          "security.password_require_special",
		Type:         config.ValueTypeBool,
		Default:      "true",
		Category:     "password",
		Tab:          "security",
		Label:        "Sonderzeichen erforderlich",
		Description:  "Passwörter müssen mindestens ein Sonderzeichen enthalten",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		DisplayOrder: 140,
	})

	// JWT settings
	settings.MustRegister(settings.Definition{
		Key:             "security.jwt_access_token_expiry_minutes",
		Type:            config.ValueTypeInt,
		Default:         "15",
		Category:        "jwt",
		Tab:             "security",
		Label:           "Access Token Gültigkeit (Minuten)",
		Description:     "Wie lange ein Access Token gültig ist",
		Scopes:          []config.Scope{config.ScopeSystem},
		ViewPerm:        "config:read",
		EditPerm:        "config:manage",
		Validation:      settings.IntRange(5, 60),
		RequiresRestart: true,
		DisplayOrder:    200,
	})

	settings.MustRegister(settings.Definition{
		Key:             "security.jwt_refresh_token_expiry_hours",
		Type:            config.ValueTypeInt,
		Default:         "24",
		Category:        "jwt",
		Tab:             "security",
		Label:           "Refresh Token Gültigkeit (Stunden)",
		Description:     "Wie lange ein Refresh Token gültig ist",
		Scopes:          []config.Scope{config.ScopeSystem},
		ViewPerm:        "config:read",
		EditPerm:        "config:manage",
		Validation:      settings.IntRange(1, 168),
		RequiresRestart: true,
		DisplayOrder:    210,
	})
}
