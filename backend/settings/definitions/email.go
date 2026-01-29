package definitions

import (
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/settings"
)

func init() {
	// Email settings
	settings.MustRegister(settings.Definition{
		Key:             "email.enabled",
		Type:            config.ValueTypeBool,
		Default:         "true",
		Category:        "email",
		Tab:             "email",
		Label:           "E-Mail aktiviert",
		Description:     "Aktiviert oder deaktiviert den E-Mail-Versand",
		Scopes:          []config.Scope{config.ScopeSystem},
		ViewPerm:        "config:read",
		EditPerm:        "config:manage",
		RequiresRestart: true,
		DisplayOrder:    10,
	})

	settings.MustRegister(settings.Definition{
		Key:             "email.smtp_host",
		Type:            config.ValueTypeString,
		Default:         "",
		Category:        "smtp",
		Tab:             "email",
		Label:           "SMTP Server",
		Description:     "Hostname des SMTP-Servers",
		Scopes:          []config.Scope{config.ScopeSystem},
		ViewPerm:        "config:read",
		EditPerm:        "config:manage",
		Validation:      settings.StringMaxLength(255),
		RequiresRestart: true,
		DisplayOrder:    20,
	})

	settings.MustRegister(settings.Definition{
		Key:             "email.smtp_port",
		Type:            config.ValueTypeInt,
		Default:         "587",
		Category:        "smtp",
		Tab:             "email",
		Label:           "SMTP Port",
		Description:     "Port des SMTP-Servers",
		Scopes:          []config.Scope{config.ScopeSystem},
		ViewPerm:        "config:read",
		EditPerm:        "config:manage",
		Validation:      settings.IntRange(1, 65535),
		RequiresRestart: true,
		DisplayOrder:    30,
	})

	settings.MustRegister(settings.Definition{
		Key:             "email.smtp_user",
		Type:            config.ValueTypeString,
		Default:         "",
		Category:        "smtp",
		Tab:             "email",
		Label:           "SMTP Benutzername",
		Description:     "Benutzername für die SMTP-Authentifizierung",
		Scopes:          []config.Scope{config.ScopeSystem},
		ViewPerm:        "config:read",
		EditPerm:        "config:manage",
		Validation:      settings.StringMaxLength(255),
		RequiresRestart: true,
		DisplayOrder:    40,
	})

	settings.MustRegister(settings.Definition{
		Key:             "email.smtp_password",
		Type:            config.ValueTypeString,
		Default:         "",
		Category:        "smtp",
		Tab:             "email",
		Label:           "SMTP Passwort",
		Description:     "Passwort für die SMTP-Authentifizierung",
		Scopes:          []config.Scope{config.ScopeSystem},
		ViewPerm:        "config:read",
		EditPerm:        "config:manage",
		IsSensitive:     true,
		RequiresRestart: true,
		DisplayOrder:    50,
	})

	settings.MustRegister(settings.Definition{
		Key:          "email.from_address",
		Type:         config.ValueTypeString,
		Default:      "noreply@example.com",
		Category:     "sender",
		Tab:          "email",
		Label:        "Absender-Adresse",
		Description:  "E-Mail-Adresse, die als Absender verwendet wird",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.StringMaxLength(255),
		DisplayOrder: 60,
	})

	settings.MustRegister(settings.Definition{
		Key:          "email.from_name",
		Type:         config.ValueTypeString,
		Default:      "Project Phoenix",
		Category:     "sender",
		Tab:          "email",
		Label:        "Absender-Name",
		Description:  "Name, der als Absender angezeigt wird",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.StringMaxLength(100),
		DisplayOrder: 70,
	})

	// Invitation settings
	settings.MustRegister(settings.Definition{
		Key:          "email.invitation_expiry_hours",
		Type:         config.ValueTypeInt,
		Default:      "48",
		Category:     "invitation",
		Tab:          "email",
		Label:        "Einladungs-Gültigkeit (Stunden)",
		Description:  "Wie lange eine Einladungs-E-Mail gültig ist",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(1, 168),
		DisplayOrder: 100,
	})

	settings.MustRegister(settings.Definition{
		Key:          "email.password_reset_expiry_minutes",
		Type:         config.ValueTypeInt,
		Default:      "30",
		Category:     "password_reset",
		Tab:          "email",
		Label:        "Passwort-Reset Gültigkeit (Minuten)",
		Description:  "Wie lange ein Passwort-Reset-Link gültig ist",
		Scopes:       []config.Scope{config.ScopeSystem},
		ViewPerm:     "config:read",
		EditPerm:     "config:manage",
		Validation:   settings.IntRange(10, 120),
		DisplayOrder: 110,
	})
}
