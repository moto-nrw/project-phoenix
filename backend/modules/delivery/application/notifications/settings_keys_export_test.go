package notifications

// RestatedSettingKeys exposes the registry keys this package restates, in the
// order of settings_keys.go, so the external suite can pin them to the
// Settings Platform's names.
var RestatedSettingKeys = []string{
	settingDispatchEnabled,
	settingActiveWindowStart,
	settingActiveWindowEnd,
	settingAbsenceReportedEnabled,
	settingOnDutyOnly,
	settingRemindersPickupUpcomingEnabled,
	settingRemindersPickupOverdueEnabled,
	settingRemindersActivityStartEnabled,
	settingRemindersActivityOverdueEnabled,
	settingParentNotesEnabled,
	settingStaffMessagingEnabled,
	settingCalendarAppointmentReminder,
}
