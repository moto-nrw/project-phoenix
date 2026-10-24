package compose_test

// The Delivery Platform outbox kinds and related entity type the appointment
// mails are written with. The literals pin the outbox contract the suites
// assert.
const (
	emailKindAppointmentPublished = "appointment_published"
	emailKindAppointmentUpdated   = "appointment_updated"
	emailKindAppointmentCancelled = "appointment_cancelled"
	emailKindAppointmentReminder  = "appointment_reminder"
	emailRelatedTypeAppointment   = "calendar_appointment"
)
