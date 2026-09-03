package calendar

import (
	"context"
)

type AppointmentRecipientRepository interface {
	CreateMany(ctx context.Context, recipients []*AppointmentRecipient) error
	FindByID(ctx context.Context, id int64) (*AppointmentRecipient, error)
	FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*AppointmentRecipient, error)
	// FindByAppointmentIDs returns the recipients of every listed appointment
	// in one read, grouped by AppointmentID on the caller's side (#2940).
	FindByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*AppointmentRecipient, error)
	UpdateResponse(ctx context.Context, recipientID int64, status string) error
	// ClaimReminderPush records the one push delivery allowed for an appointment
	// revision, occurrence, and guardian. It returns false when a prior scheduler
	// scan already claimed the same delivery.
	ClaimReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate Date, guardianProfileID int64) (bool, error)
	ReleaseReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate Date, guardianProfileID int64) error
}

type AppointmentRecipientStudentRepository interface {
	CreateMany(ctx context.Context, links []*AppointmentRecipientStudent) error
	FindByRecipientIDs(ctx context.Context, recipientIDs []int64) ([]*AppointmentRecipientStudent, error)
}
