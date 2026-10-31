package api

import "encoding/json"

// These are HTTP contract values, kept at the dev-tool boundary because the
// architecture policy forbids CLI packages from importing a domain model.
const (
	profileSettingSchoolWriteScope      = "operations.school_portal_write_scope"
	profileSettingSchoolMorning         = "operations.school_portal_morning_attendance"
	profileSettingAnalyticsEnabled      = "analytics.enabled"
	profileSettingPresenceMode          = "operations.presence_mode"
	profileSettingAttendanceNFC         = "module.nfc_anwesenheit"
	profileSettingModuleTraeger         = "module.traeger_uebersicht"
	profileSettingModuleFerien          = "module.ferienbetreuung"
	profileSettingModuleVertraege       = "module.vertraege"
	profileSettingModuleBeitraege       = "module.elternbeitraege"
	profileSettingAttendanceWeb         = "attendance.web_enabled"
	profileSettingSessionEndTime        = "operations.session_end_time"
	profileSettingOverviewScope         = "operations.operational_overview_scope"
	profileSettingAttendanceScope       = "operations.attendance_edit_scope"
	profileSettingBlockStartScope       = "operations.block_start_scope"
	profileSettingBlockCompleteScope    = "operations.block_complete_scope"
	profileSettingAbsenceScope          = "operations.student_absence_edit_scope"
	profileSettingParentSickMode        = "operations.parent_sick_reports_enabled"
	profileSettingParentExcusedMode     = "operations.parent_excused_reports_enabled"
	profileSettingAbsenceReviewScope    = "operations.parent_absence_review_scope"
	profileSettingRequestReviewScope    = "operations.parent_request_review_scope"
	profileSettingGroupMode             = "operations.group_mode"
	profileSettingWebSpontaneous        = "attendance.web_spontaneous_activities_enabled"
	profileSettingEnrollmentEnabled     = "enrollment.enabled"
	profileSettingCareOfferingsEnabled  = "enrollment.care_offerings_enabled"
	profileSettingBookingsAuthoritative = "enrollment.bookings_authoritative"
	profileSettingDevicePIN             = "security.ogs_device_pin"
	profileSettingStudentPhotos         = "operations.student_photos_enabled"
	profileSettingWeekendFollowsFriday  = "operations.weekend_follows_friday"
	profileSettingBirthdayIncludeStaff  = "operations.birthday_display_include_staff"
	profileSettingAttendanceLog         = "gdpr.attendance_log_enabled"
	profileSettingFeedbackEnabled       = "feedback.enabled"
	profilePresenceDetailed             = "detailed"
	profilePresenceBinary               = "binary"
	profileGroupModeFixed               = "fixed_groups"
	profileGroupModeOpenCare            = "open_care"
)

// The payroll keys of the Abrechnung page (#5032), HTTP contract values like
// the ones above.
const (
	profileSettingPayrollRegelarbeit        = "payroll.lohnart_regelarbeit"
	profileSettingPayrollPlusStunden        = "payroll.lohnart_plus_stunden"
	profileSettingPayrollAuszahlung         = "payroll.lohnart_auszahlung"
	profileSettingPayrollFreizeitausgleich  = "payroll.lohnart_freizeitausgleich"
	profileSettingPayrollKrank              = "payroll.lohnart_krank"
	profileSettingPayrollUrlaub             = "payroll.lohnart_urlaub"
	profileSettingPayrollFortbildung        = "payroll.lohnart_fortbildung"
	profileSettingPayrollEinheitKrank       = "payroll.einheit_krank"
	profileSettingPayrollEinheitUrlaub      = "payroll.einheit_urlaub"
	profileSettingPayrollEinheitFortbildung = "payroll.einheit_fortbildung"
	profileSettingPayrollBeraternummer      = "payroll.datev_beraternummer"
	profileSettingPayrollMandantennummer    = "payroll.datev_mandantennummer"
	profileSettingPayrollDispatchAuto       = "payroll.dispatch_auto"
	profileSettingPayrollDispatchChannel    = "payroll.dispatch_channel"
	profileSettingPayrollDispatchFormat     = "payroll.dispatch_format"
	profileSettingPayrollDispatchEmail      = "payroll.dispatch_email"
	profilePayrollUnitHours                 = "stunden"
	profilePayrollUnitDays                  = "tage"
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
		Key:              DefaultProfileKey,
		OrganizationName: "Ganztag Nord e. V.",
		OrganizationSlug: "demo-traeger-nord",
		SchoolName:       "OGS Sonnenhof",
		SchoolSlug:       DefaultProfileKey,
		// A personal address: the Träger-Portal shows it as its
		// first person; the reserved TLD .example never receives mail.
		SchoolAdminEmail:    "ines.lindner@ganztag-nord.example",
		SchoolAdminPassword: "Vollbetrieb1234%",
		Settings:            fullOperationProfileSettings(),
		Expected:            fullOperationExpectedState(),
	}
}

// fullOperationProfileSettings adds histories and the payroll configuration
// to the Vollbetrieb school only. The Anmeldung profiles reuse
// fullOperationSettings but seed neither history nor Personalnummern, so a
// complete DATEV setup there would still report every colleague as missing
// (#5032).
func fullOperationProfileSettings() map[string]SeedSetting {
	settings := fullOperationSettings()
	for key, setting := range demoHistorySettings() {
		settings[key] = setting
	}
	for key, setting := range demoPayrollSettings() {
		settings[key] = setting
	}
	return settings
}

// demoPayrollSettings completes the Abrechnung page with invented numbers
// (#5032). The registry defaults stay empty, since real numbers belong to
// each Träger's DATEV; every payroll key is admin-only, so the school writes
// them. The Lohnbüro (#4805) gets the automatic send by e-mail to an address
// of the reserved example domain, so a month close on a demo stack sends no
// payroll data anywhere real.
func demoPayrollSettings() map[string]SeedSetting {
	text := func(value string) SeedSetting {
		return SeedSetting{Value: json.RawMessage(`"` + value + `"`), ManagedBy: SettingManagedByTenant}
	}
	return map[string]SeedSetting{
		profileSettingPayrollRegelarbeit:        text("9001"),
		profileSettingPayrollPlusStunden:        text("9002"),
		profileSettingPayrollAuszahlung:         text("9003"),
		profileSettingPayrollFreizeitausgleich:  text("9004"),
		profileSettingPayrollKrank:              text("9005"),
		profileSettingPayrollUrlaub:             text("9006"),
		profileSettingPayrollFortbildung:        text("9007"),
		profileSettingPayrollEinheitKrank:       text(profilePayrollUnitHours),
		profileSettingPayrollEinheitUrlaub:      text(profilePayrollUnitDays),
		profileSettingPayrollEinheitFortbildung: text(profilePayrollUnitHours),
		profileSettingPayrollBeraternummer:      text("9900001"),
		profileSettingPayrollMandantennummer:    text("90001"),
		profileSettingPayrollDispatchAuto:       {Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant},
		profileSettingPayrollDispatchChannel:    text("email"),
		profileSettingPayrollDispatchFormat:     text("datev_lodas"),
		profileSettingPayrollDispatchEmail:      text("lohnbuero@example.com"),
	}
}

func fullOperationSettings() map[string]SeedSetting {
	settings := fullOperationFeatureSettings()
	for key, setting := range notOfferedModuleSettings() {
		settings[key] = setting
	}
	return settings
}

// demoHistorySettings switch on the histories the seed fills. The registry
// keeps both off for real schools.
func demoHistorySettings() map[string]SeedSetting {
	return map[string]SeedSetting{
		// The Statistik's rooms and a child's room history show the seeded
		// past (#5031); without the log the nightly cleanup deletes it.
		profileSettingAttendanceLog: {Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant},
		// The Feedbackhistorie of a child shows the seeded feedback (#5029);
		// the feedback API refuses to store entries while it is off.
		profileSettingFeedbackEnabled: {Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant},
	}
}

// notOfferedModuleSettings cancels the modules moto does not offer yet, so
// the Module tab shows booked and not booked switches (#4892). Writes run in
// key order: Elternbeiträge goes before the Verträge it requires.
func notOfferedModuleSettings() map[string]SeedSetting {
	cancelled := SeedSetting{Value: json.RawMessage(`false`), ManagedBy: SettingManagedByOperator}
	return map[string]SeedSetting{
		profileSettingModuleTraeger:   cancelled,
		profileSettingModuleFerien:    cancelled,
		profileSettingModuleVertraege: cancelled,
		profileSettingModuleBeitraege: cancelled,
	}
}

func fullOperationFeatureSettings() map[string]SeedSetting {
	return map[string]SeedSetting{
		profileSettingSchoolWriteScope:   {Value: json.RawMessage(`["class_arrival_exceptions","class_handover","class_trips","day_hints"]`), ManagedBy: SettingManagedByTenant},
		profileSettingSchoolMorning:      {Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant},
		profileSettingOverviewScope:      {Value: json.RawMessage(`"all_staff"`), ManagedBy: SettingManagedByTenant},
		profileSettingAttendanceScope:    {Value: json.RawMessage(`"all_staff"`), ManagedBy: SettingManagedByTenant},
		profileSettingBlockStartScope:    {Value: json.RawMessage(`"all_staff"`), ManagedBy: SettingManagedByTenant},
		profileSettingBlockCompleteScope: {Value: json.RawMessage(`"all_staff"`), ManagedBy: SettingManagedByTenant},
		profileSettingAbsenceScope:       {Value: json.RawMessage(`"all_staff"`), ManagedBy: SettingManagedByTenant},
		profileSettingParentSickMode:     {Value: json.RawMessage(`"immediate"`), ManagedBy: SettingManagedByTenant},
		profileSettingParentExcusedMode:  {Value: json.RawMessage(`"approval"`), ManagedBy: SettingManagedByTenant},
		profileSettingAbsenceReviewScope: {Value: json.RawMessage(`"group_leaders"`), ManagedBy: SettingManagedByTenant},
		// The Betreuungskraft of the demo leads the group of the demo parents'
		// child, so she reviews their requests too, not only their sick notes
		// (#3894). Without it the request scope inherits "admins".
		profileSettingRequestReviewScope: {Value: json.RawMessage(`"group_leaders"`), ManagedBy: SettingManagedByTenant},
		profileSettingPresenceMode: {
			Value: json.RawMessage(`"` + profilePresenceDetailed + `"`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingAttendanceNFC: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingAttendanceWeb: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator,
		},
		// The seeded arrival plans leave several children expected but not
		// checked in. Keeping the demo care day open until 23:59 makes the
		// dashboard's "Schule" tile reviewable throughout the seed day.
		profileSettingSessionEndTime: {
			Value: json.RawMessage(`"23:59"`), ManagedBy: SettingManagedByOperator,
		},
		profileSettingGroupMode: {
			Value: json.RawMessage(`"` + profileGroupModeFixed + `"`), ManagedBy: SettingManagedByTenant,
		},
		// The product screenshots photograph this school, including the
		// spontaneous start on the supervision page (shot spontane-aktivitaet).
		profileSettingWebSpontaneous: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant,
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
		// The public demo must look like a care day at any time (#3921): on
		// Saturday and Sunday the school runs Friday's plan.
		profileSettingWeekendFollowsFriday: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator,
		},
		// The birthday card shows a colleague next to the children (#3922);
		// the seed gives one a birthday on the seed day.
		profileSettingBirthdayIncludeStaff: {Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant},
	}
}

func fullOperationExpectedState() SeedExpectedState {
	return SeedExpectedState{
		Students:        len(DemoStudents),
		Rooms:           len(DemoRooms),
		Groups:          10,
		Staff:           len(DemoStaff),
		Contacts:        demoGuardianProfileCount(),
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
		OrganizationName:    "Ganztag Nord e. V.",
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
		profileSettingAnalyticsEnabled:   {Value: json.RawMessage(`false`), ManagedBy: SettingManagedByTenant},
		profileSettingOverviewScope:      {Value: json.RawMessage(`"own"`), ManagedBy: SettingManagedByTenant},
		profileSettingAttendanceScope:    {Value: json.RawMessage(`"own"`), ManagedBy: SettingManagedByTenant},
		profileSettingBlockStartScope:    {Value: json.RawMessage(`"own"`), ManagedBy: SettingManagedByTenant},
		profileSettingBlockCompleteScope: {Value: json.RawMessage(`"own"`), ManagedBy: SettingManagedByTenant},
		profileSettingAbsenceScope:       {Value: json.RawMessage(`"admins"`), ManagedBy: SettingManagedByTenant},
		profileSettingParentSickMode:     {Value: json.RawMessage(`"approval"`), ManagedBy: SettingManagedByTenant},
		profileSettingParentExcusedMode:  {Value: json.RawMessage(`"immediate"`), ManagedBy: SettingManagedByTenant},
		profileSettingAbsenceReviewScope: {Value: json.RawMessage(`"admins"`), ManagedBy: SettingManagedByTenant},
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
		profileSettingWebSpontaneous: {
			Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant,
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
