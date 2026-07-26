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
		DependsOn:       config.DependsOnEq(config.KeyAttendanceNFCEnabled, true),
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
		DependsOn:       config.DependsOnNeq(config.KeyMFAMode, config.MFAModeOff),
	})

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
		Validation:      config.Range(1, 180),
		DependsOn:       config.DependsOnEq(config.KeyMFATrustedDeviceEnabled, true),
	})

	// --- Account brute-force lockout policy (issue #586) ---

	config.Register(config.Definition{
		Key:             config.KeyAccountLockoutThreshold,
		Label:           "Konto-Sperre: Fehlversuche",
		Description:     "Anzahl fehlgeschlagener PIN- oder 2FA-Versuche, nach denen ein Konto vorübergehend gesperrt wird.",
		Type:            config.FieldNumber,
		Default:         5,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "lockout",
		SortOrder:       30,
		Validation:      config.Range(1, 20),
	})

	config.Register(config.Definition{
		Key:             config.KeyAccountLockoutDurationMinutes,
		Label:           "Konto-Sperre: Dauer (Minuten)",
		Description:     "Wie lange ein Konto nach Überschreiten der Fehlversuche gesperrt bleibt, bevor wieder Versuche möglich sind.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "lockout",
		SortOrder:       31,
		Validation:      config.Range(1, 1440),
	})

}
