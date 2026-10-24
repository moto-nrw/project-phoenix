package application

// The Delivery outbox kinds the appointment mails are written with. The
// literals pin the outbox contract the suites assert.
const (
	emailKindAppointmentPublished = "appointment_published"
	emailKindAppointmentUpdated   = "appointment_updated"
	emailKindAppointmentCancelled = "appointment_cancelled"
	emailKindAppointmentReminder  = "appointment_reminder"
)
