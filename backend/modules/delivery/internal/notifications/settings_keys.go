package notifications

// Keys of the Settings Platform registry this package reads, and the guardian
// permission its parent fan-out re-checks. The registry and the Security
// Runtime own them; this package may import neither, so it restates the
// names, and settings_keys_test.go pins every one to its owner's name.
const (
	settingDispatchEnabled                     = "notifications.dispatch_enabled"
	settingActiveWindowStart                   = "notifications.active_window_start"
	settingActiveWindowEnd                     = "notifications.active_window_end"
	settingAbsenceReportedEnabled              = "notifications.absence_reported_enabled"
	settingOnDutyOnly                          = "notifications.on_duty_only"
	settingRemindersPickupUpcomingEnabled      = "reminders.pickup_upcoming_enabled"
	settingRemindersPickupOverdueEnabled       = "reminders.pickup_overdue_enabled"
	settingRemindersActivityStartEnabled       = "reminders.activity_start_enabled"
	settingRemindersActivityOverdueEnabled     = "reminders.activity_overdue_enabled"
	settingRemindersActivityRunningOverEnabled = "reminders.activity_running_over_enabled"
	settingParentNotesEnabled                  = "operations.parent_notes_enabled"
	settingStaffMessagingEnabled               = "operations.staff_messaging_enabled"
	settingCalendarAppointmentReminder         = "calendar.appointment_reminder_enabled"
	// Module switches (#4895): pushes to parents need Eltern-App, pushes to
	// devices of the school portal need Unterricht am Vormittag.
	settingModuleElternApp           = "module.eltern_app"
	settingModuleUnterrichtVormittag = "module.unterricht_vormittag"
	guardianPermissionPortalAccess   = "parent_portal.access"
)
