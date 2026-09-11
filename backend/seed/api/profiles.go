package api

import "encoding/json"

// These are HTTP contract values, kept at the dev-tool boundary because the
// architecture policy forbids CLI packages from importing a domain model.
const (
	profileSettingPresenceMode          = "operations.presence_mode"
	profileSettingAttendanceNFC         = "attendance.nfc_enabled"
	profileSettingAttendanceWeb         = "attendance.web_enabled"
	profileSettingGroupMode             = "operations.group_mode"
	profileSettingCareConcept           = "operations.care_concept"
	profileSettingEnrollmentEnabled     = "enrollment.enabled"
	profileSettingCareOfferingsEnabled  = "enrollment.care_offerings_enabled"
	profileSettingBookingsAuthoritative = "enrollment.bookings_authoritative"
	profilePresenceDetailed             = "detailed"
	profilePresenceBinary               = "binary"
	profileGroupModeFixed               = "fixed_groups"
	profileGroupModeOpenCare            = "open_care"
	profileCareConceptFixedSchedule     = "fixed_schedule"
	profileCareConceptOpenRooms         = "open_rooms"
)

type demoProfileDefinition struct {
	Key                 string
	OrganizationName    string
	OrganizationSlug    string
	SchoolName          string
	SchoolSlug          string
	SchoolAdminEmail    string
	SchoolAdminPassword string
	Settings            map[string]SeedSetting
	Expected            SeedExpectedState
}

func fullOperationProfileDefinition() demoProfileDefinition {
	return demoProfileDefinition{
		Key:                 DefaultProfileKey,
		OrganizationName:    "Demo-Träger Nord",
		OrganizationSlug:    "demo-traeger-nord",
		SchoolName:          "Demo-Schule Vollbetrieb",
		SchoolSlug:          DefaultProfileKey,
		SchoolAdminEmail:    "vollbetrieb-admin@example.test",
		SchoolAdminPassword: "Vollbetrieb1234%",
		Settings:            fullOperationSettings(),
		Expected:            fullOperationExpectedState(),
	}
}

func fullOperationSettings() map[string]SeedSetting {
	return map[string]SeedSetting{
		profileSettingPresenceMode: {
			Value: json.RawMessage(`"` + profilePresenceDetailed + `"`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingAttendanceNFC: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingAttendanceWeb: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingGroupMode: {
			Value: json.RawMessage(`"` + profileGroupModeFixed + `"`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingCareConcept: {
			Value: json.RawMessage(`"` + profileCareConceptFixedSchedule + `"`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingEnrollmentEnabled: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingCareOfferingsEnabled: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingBookingsAuthoritative: {
			Value: json.RawMessage(`false`), ManagedBy: SettingManagedByOperator,
		},
	}
}

func fullOperationExpectedState() SeedExpectedState {
	return SeedExpectedState{
		Students:        len(DemoStudents),
		Rooms:           len(DemoRooms),
		Groups:          10,
		Staff:           len(DemoStaff),
		Contacts:        len(DemoGuardians),
		ParentAccounts:  6,
		PhysicalDevices: len(DemoDevices),
		HasEnrollment:   true,
		HasAttendance:   true,
		HasHistory:      true,
		ScheduledActivities: SeedScheduledActivityState{
			Minimum: 1, Rooms: true, Students: true, Staff: true,
		},
	}
}

func manualProfileDefinition() demoProfileDefinition {
	return demoProfileDefinition{
		Key:                 ManualProfileKey,
		OrganizationName:    "Demo-Träger Nord",
		OrganizationSlug:    "demo-traeger-nord",
		SchoolName:          "Demo-Schule Manuell",
		SchoolSlug:          ManualProfileKey,
		SchoolAdminEmail:    "manuell-admin@example.test",
		SchoolAdminPassword: "Manuell1234%",
		Settings:            manualProfileSettings(),
		Expected: SeedExpectedState{
			Students: 12, Groups: 2, Staff: 2, Contacts: 12,
			PhysicalDevices: 0, HasAttendance: true, HasHistory: true,
			PresentStudents: 4, CheckedOutStudents: 4, WeeklyPlans: 12,
		},
	}
}

func manualProfileSettings() map[string]SeedSetting {
	return map[string]SeedSetting{
		profileSettingPresenceMode: {
			Value: json.RawMessage(`"` + profilePresenceBinary + `"`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingAttendanceNFC: {
			Value: json.RawMessage(`false`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingAttendanceWeb: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingGroupMode: {
			Value: json.RawMessage(`"` + profileGroupModeOpenCare + `"`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingCareConcept: {
			Value: json.RawMessage(`"` + profileCareConceptOpenRooms + `"`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingEnrollmentEnabled: {
			Value: json.RawMessage(`false`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingCareOfferingsEnabled: {
			Value: json.RawMessage(`false`), ManagedBy: SettingManagedByTenant,
		},
		profileSettingBookingsAuthoritative: {
			Value: json.RawMessage(`false`), ManagedBy: SettingManagedByOperator,
		},
	}
}

func cloneProfileSettings(settings map[string]SeedSetting) map[string]SeedSetting {
	cloned := make(map[string]SeedSetting, len(settings))
	for key, setting := range settings {
		cloned[key] = setting
	}
	return cloned
}
