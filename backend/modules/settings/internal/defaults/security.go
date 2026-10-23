package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func init() {

	registerDefinitions(security_init_pin_0Definitions, security_init_mfa_1Definitions, security_init_lockout_2Definitions)

	// --- MFA / Two-Factor Authentication (issue #1308) ---

	// --- Account brute-force lockout policy (issue #586) ---

}

var security_init_pin_0Definitions = []config.Definition{
	{
		Key:             config.KeyOGSDevicePIN,
		Label:           "Geräte-PIN",
		Description:     "Mit dieser PIN meldet sich Ihr Team am Tablet an. Nutzen Sie keine PIN, die Sie auch anderswo verwenden.",
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
	},
}

var security_init_mfa_1Definitions = []config.Definition{
	{
		Key:             config.KeyMFAMode,
		Label:           "Zwei-Faktor-Anmeldung (Code per E-Mail)",
		Description:     "Für Verwaltung, Gesundheitsdaten und Personalfinanzen ist ein zweiter Nachweis immer Pflicht. „Aus“ und „Nur Admins“ ändern daran nichts. „Alle Mitarbeitenden“ schützt zusätzlich das ganze Team.",
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
	},
	{
		Key:             config.KeyMFATrustedDeviceEnabled,
		Label:           "Gerät merken erlauben",
		Description:     "Mitarbeitende können beim Anmelden ihr Gerät merken lassen. Dort brauchen sie dann eine Zeit lang keinen Code.",
		Type:            config.FieldBoolean,
		Default:         true,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "mfa",
		SortOrder:       20,
	},
	{
		Key:             config.KeyMFATrustedDeviceDays,
		Label:           "Gültigkeit gemerkter Geräte (Tage)",
		Description:     "So lange braucht ein gemerktes Gerät keinen Code.",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "mfa",
		SortOrder:       21,
		Validation:      config.Range(1, 180),
		DependsOn:       config.DependsOnEq(config.KeyMFATrustedDeviceEnabled, true),
	},
}

var security_init_lockout_2Definitions = []config.Definition{
	{
		Key:             config.KeyAccountLockoutThreshold,
		Label:           "Fehlversuche bis zur Kontosperre",
		Description:     "Nach so vielen falschen Eingaben von PIN oder Code sperrt moto das Konto für eine Weile.",
		Type:            config.FieldNumber,
		Default:         5,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "lockout",
		SortOrder:       30,
		Validation:      config.Range(1, 20),
	},
	{
		Key:             config.KeyAccountLockoutDurationMinutes,
		Label:           "Dauer der Kontosperre (Minuten)",
		Description:     "So lange bleibt das Konto danach gesperrt.",
		Type:            config.FieldNumber,
		Default:         15,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "security",
		Category:        "lockout",
		SortOrder:       31,
		Validation:      config.Range(1, 1440),
	},
}

var pinPattern = `^\d{4}$`
