package config

import (
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// Permission constants for scope permissions
const (
	permConfigManage   = "config:manage"
	permSchoolSettings = "school:settings"
)

// GetDefaultDefinitions returns the code-defined setting definitions
// These are synced to the database at startup
func GetDefaultDefinitions() []*config.SettingDefinition {
	return []*config.SettingDefinition{
		// === Session Settings ===
		{
			Key:          "session.timeout_minutes",
			Type:         config.SettingTypeInt,
			DefaultValue: mustMarshalValue(30),
			Category:     "session",
			Description:  "Minuten der Inaktivität bevor automatischer Checkout",
			Validation: &config.Validation{
				Min: floatPtr(1),
				Max: floatPtr(480),
			},
			AllowedScopes: []string{"system", "school", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"school": permSchoolSettings,
				"og":     "owner",
			},
			GroupName: "session",
			SortOrder: 1,
		},
		{
			Key:           "session.auto_checkout",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(true),
			Category:      "session",
			Description:   "Automatischer Checkout bei Session-Ende",
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			GroupName: "session",
			SortOrder: 2,
		},
		{
			Key:          "session.warning_minutes",
			Type:         config.SettingTypeInt,
			DefaultValue: mustMarshalValue(5),
			Category:     "session",
			Description:  "Minuten vor Timeout für Warnung",
			Validation: &config.Validation{
				Min: floatPtr(1),
				Max: floatPtr(60),
			},
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			GroupName: "session",
			SortOrder: 3,
		},

		// === Pickup Settings ===
		{
			Key:           "pickup.has_earliest_time",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(false),
			Category:      "pickup",
			Description:   "Gibt es eine frühste Abholzeit?",
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			GroupName: "pickup_time",
			SortOrder: 1,
		},
		{
			Key:           "pickup.earliest_time",
			Type:          config.SettingTypeTime,
			DefaultValue:  mustMarshalValue("15:00"),
			Category:      "pickup",
			Description:   "Frühste Abholzeit",
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			DependsOn: &config.SettingDependency{
				Key:       "pickup.has_earliest_time",
				Condition: "equals",
				Value:     true,
			},
			GroupName: "pickup_time",
			SortOrder: 2,
		},
		{
			Key:           "pickup.has_latest_time",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(false),
			Category:      "pickup",
			Description:   "Gibt es eine späteste Abholzeit?",
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			GroupName: "pickup_time",
			SortOrder: 3,
		},
		{
			Key:           "pickup.latest_time",
			Type:          config.SettingTypeTime,
			DefaultValue:  mustMarshalValue("17:00"),
			Category:      "pickup",
			Description:   "Späteste Abholzeit",
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			DependsOn: &config.SettingDependency{
				Key:       "pickup.has_latest_time",
				Condition: "equals",
				Value:     true,
			},
			GroupName: "pickup_time",
			SortOrder: 4,
		},

		// === Notification Settings ===
		{
			Key:           "notifications.absence_enabled",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(true),
			Category:      "notifications",
			Description:   "Benachrichtigungen bei Abwesenheit aktiviert",
			AllowedScopes: []string{"system", "school"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"school": permSchoolSettings,
			},
			GroupName: "notifications_absence",
			SortOrder: 1,
		},
		{
			Key:           "notifications.absence_channels",
			Type:          config.SettingTypeJSON,
			DefaultValue:  mustMarshalValue([]string{"email"}),
			Category:      "notifications",
			Description:   "Benachrichtigungskanäle bei Abwesenheit",
			AllowedScopes: []string{"system", "school", "user"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"school": permSchoolSettings,
				"user":   "self",
			},
			DependsOn: &config.SettingDependency{
				Key:       "notifications.absence_enabled",
				Condition: "equals",
				Value:     true,
			},
			GroupName: "notifications_absence",
			SortOrder: 2,
		},

		// === UI Settings ===
		{
			Key:          "ui.theme",
			Type:         config.SettingTypeEnum,
			DefaultValue: mustMarshalValue("light"),
			Category:     "appearance",
			Description:  "Farbschema der Benutzeroberfläche",
			Validation: &config.Validation{
				Options: []string{"light", "dark", "system"},
			},
			AllowedScopes: []string{"system", "user"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"user":   "self",
			},
			GroupName: "appearance",
			SortOrder: 1,
		},
		{
			Key:          "ui.language",
			Type:         config.SettingTypeEnum,
			DefaultValue: mustMarshalValue("de"),
			Category:     "appearance",
			Description:  "Sprache der Benutzeroberfläche",
			Validation: &config.Validation{
				Options: []string{"de", "en"},
			},
			AllowedScopes: []string{"system", "user"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"user":   "self",
			},
			GroupName: "appearance",
			SortOrder: 2,
		},

		// === Audit Settings ===
		{
			Key:           "audit.track_setting_changes",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(true),
			Category:      "audit",
			Description:   "Änderungen an Einstellungen protokollieren",
			AllowedScopes: []string{"system"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
			},
			GroupName: "audit",
			SortOrder: 1,
		},
		{
			Key:          "audit.setting_retention_days",
			Type:         config.SettingTypeInt,
			DefaultValue: mustMarshalValue(365),
			Category:     "audit",
			Description:  "Aufbewahrungsdauer für Einstellungsänderungen in Tagen",
			Validation: &config.Validation{
				Min: floatPtr(30),
				Max: floatPtr(3650),
			},
			AllowedScopes: []string{"system"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
			},
			GroupName: "audit",
			SortOrder: 2,
		},

		// === Check-in Settings ===
		{
			Key:           "checkin.require_pin",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(false),
			Category:      "checkin",
			Description:   "PIN-Eingabe bei Check-in erforderlich",
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			GroupName: "checkin",
			SortOrder: 1,
		},
		{
			Key:           "checkin.allow_manual",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(true),
			Category:      "checkin",
			Description:   "Manuelle Check-ins erlauben (ohne RFID)",
			AllowedScopes: []string{"system", "og"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"og":     "owner",
			},
			GroupName: "checkin",
			SortOrder: 2,
		},

		// === Device Settings ===
		{
			Key:           "device.beep_on_scan",
			Type:          config.SettingTypeBool,
			DefaultValue:  mustMarshalValue(true),
			Category:      "device",
			Description:   "Ton bei RFID-Scan abspielen",
			AllowedScopes: []string{"system", "device"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"device": "iot:manage",
			},
			GroupName: "device",
			SortOrder: 1,
		},
		{
			Key:          "device.display_timeout_seconds",
			Type:         config.SettingTypeInt,
			DefaultValue: mustMarshalValue(30),
			Category:     "device",
			Description:  "Sekunden bis Display-Timeout",
			Validation: &config.Validation{
				Min: floatPtr(5),
				Max: floatPtr(300),
			},
			AllowedScopes: []string{"system", "device"},
			ScopePermissions: map[string]string{
				"system": permConfigManage,
				"device": "iot:manage",
			},
			GroupName: "device",
			SortOrder: 2,
		},
	}
}

// Helper functions

func mustMarshalValue(value any) json.RawMessage {
	data, err := config.MarshalValue(value)
	if err != nil {
		panic(err)
	}
	return data
}

func floatPtr(f float64) *float64 {
	return &f
}
