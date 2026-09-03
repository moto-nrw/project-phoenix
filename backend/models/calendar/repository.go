package calendar

import (
	"context"
)

type RecurrenceRuleRepository interface {
	Create(ctx context.Context, rule *RecurrenceRule) error
	FindByAppointmentID(ctx context.Context, appointmentID int64) (*RecurrenceRule, error)
	FindByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*RecurrenceRule, error)
	DeleteByAppointmentID(ctx context.Context, appointmentID int64) error
}

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

type AppointmentOccurrenceOverrideRepository interface {
	Create(ctx context.Context, override *AppointmentOccurrenceOverride) error
	Update(ctx context.Context, override *AppointmentOccurrenceOverride) error
	FindByAppointmentIDsAndOccurrenceDates(ctx context.Context, appointmentIDs []int64, occurrenceDates []Date) ([]*AppointmentOccurrenceOverride, error)
	// FindByAppointmentIDsAndStartDates returns moved occurrences whose effective
	// start falls in the supplied dates. Reminder scans use it to find an
	// occurrence moved outside its rule's normal expansion window.
	FindByAppointmentIDsAndStartDates(ctx context.Context, appointmentIDs []int64, startDates []Date) ([]*AppointmentOccurrenceOverride, error)
	// FindCancelledByAppointmentIDs returns every cancelled occurrence override
	// for the given appointments — used to emit iCalendar EXDATEs so subscribed
	// external calendars drop occurrences cancelled via "Nur diesen Termin".
	FindCancelledByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*AppointmentOccurrenceOverride, error)
	// CancelOccurrence marks the occurrence cancelled, inserting the override or
	// updating an existing one. It is conflict-safe (INSERT ... ON CONFLICT DO
	// UPDATE), so two concurrent cancellations of the same occurrence converge
	// instead of one hitting the (tenant_id, appointment_id, occurrence_date)
	// unique constraint and failing.
	CancelOccurrence(ctx context.Context, appointmentID int64, occurrenceDate Date) error
	// DeleteByAppointmentID removes every occurrence override for an appointment.
	// Used when a series edit replaces the recurrence rule wholesale so stale
	// per-occurrence cancellations from the old cadence cannot suppress valid
	// occurrences (or leak EXDATEs) in the edited series.
	DeleteByAppointmentID(ctx context.Context, appointmentID int64) error
	Delete(ctx context.Context, id any) error
}
