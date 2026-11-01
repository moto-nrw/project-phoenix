package ports

import "context"

// StaffQualificationReminders sends the due reminders of the staff
// qualifications (Nachweise) of the school in ctx (#4808): once at the start
// of the lead and once on the expiry day. The root binds Workforce.
type StaffQualificationReminders func(ctx context.Context) error
