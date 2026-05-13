package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	pinPattern := `^\d{4}$`
	config.Register(config.Definition{
		Key:             config.KeyOGSDevicePIN,
		Label:           "OGS Geräte-PIN",
		Description:     "PIN für die Authentifizierung an RFID-Geräten. Wird als Klartext gespeichert und in der Oberfläche maskiert.",
		Type:            config.FieldPassword,
		Default:         "1234",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "devices",
		Category:        "pin",
		SortOrder:       1,
		Validation:      &config.ValidationRules{Pattern: &pinPattern},
		AccessPolicy:    config.AccessAdminOnly,
	})

	// --- MFA / Two-Factor Authentication (issue #1308) ---

	config.Register(config.Definition{
		Key:             config.KeyMFAMode,
		Label:           "Zwei-Faktor-Authentifizierung",
		Description:     "Legt fest, ob ein zweiter Faktor per E-Mail beim Login erforderlich ist. \"Nur Admins\" verlangt 2FA für Schul-Admins, \"Alle\" für alle Mitarbeitenden.",
		Type:            config.FieldSelect,
		Default:         config.MFAModeOff,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "mfa",
		SortOrder:       10,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Aus", Value: config.MFAModeOff},
				{Label: "Nur Admins", Value: config.MFAModeRequiredAdmins},
				{Label: "Alle Mitarbeitenden", Value: config.MFAModeRequiredAll},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyMFATrustedDeviceEnabled,
		Label:           "Vertrauenswürdige Geräte erlauben",
		Description:     "Wenn aktiviert, können Mitarbeitende ihren Browser als vertrauenswürdig markieren und 2FA für die Cookie-Laufzeit überspringen.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "mfa",
		SortOrder:       20,
		DependsOn: &config.Dependency{
			Key:       config.KeyMFAMode,
			Condition: "neq",
			Value:     config.MFAModeOff,
		},
	})

	mfaTrustedDeviceMin := float64(1)
	mfaTrustedDeviceMax := float64(180)
	config.Register(config.Definition{
		Key:             config.KeyMFATrustedDeviceDays,
		Label:           "Vertrauenswürdige Geräte: Gültigkeit (Tage)",
		Description:     "Wie lange ein als vertrauenswürdig markierter Browser ohne erneute 2FA gilt.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "mfa",
		SortOrder:       21,
		Validation:      &config.ValidationRules{Min: &mfaTrustedDeviceMin, Max: &mfaTrustedDeviceMax},
		DependsOn: &config.Dependency{
			Key:       config.KeyMFATrustedDeviceEnabled,
			Condition: "eq",
			Value:     true,
		},
	})

}
