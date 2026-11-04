package contract

// Notification types of the qualification reminders (#4808): once at the
// start of the lead and once on the expiry day of a Nachweis. The root sends
// them. The Leitung's notice goes to the holders of staff:stammdaten and is
// on by default; the affected person's own notice is theirs to switch on.
const (
	NotificationStaffQualificationExpiry    = "staff_qualification_expiry"
	NotificationStaffQualificationOwnExpiry = "staff_qualification_own_expiry"
)
