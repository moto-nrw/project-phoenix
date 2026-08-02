package calendar

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type AppointmentRepository interface {
	Create(ctx context.Context, appointment *Appointment) error
	FindByID(ctx context.Context, id int64) (*Appointment, error)
	// FindByIDForUpdate locks an appointment until the current transaction
	// completes. It serializes reminder delivery with lifecycle changes and
	// per-occurrence overrides that affect that delivery.
	FindByIDForUpdate(ctx context.Context, id int64) (*Appointment, error)
	Update(ctx context.Context, appointment *Appointment) error
	// BumpRevision advances the revision counter without touching other fields,
	// used when a change affecting the exported calendar lives in a child table.
	BumpRevision(ctx context.Context, appointmentID int64) error
	Delete(ctx context.Context, id any) error
	// Cancel marks the appointment cancelled_at=now and bumps the revision with a
	// conditional (cancelled_at IS NULL) update, so a concurrent content edit
	// cannot silently reactivate it. It reports whether THIS call performed the
	// transition (true) or found it already cancelled (false) — only the
	// transitioning caller should send the cancellation notice. Stays visible
	// interactively.
	Cancel(ctx context.Context, appointmentID int64) (bool, error)
	// SoftDelete marks the appointment deleted_at=now and bumps the revision. Used
	// for feed-visible appointments so they vanish from interactive calendars but
	// remain exportable as a durable STATUS:CANCELLED tombstone.
	SoftDelete(ctx context.Context, appointmentID int64) error
	// ListVisible*/ListOrganized* return only live (deleted_at IS NULL) rows.
	ListVisibleForStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*Appointment, error)
	ListVisibleForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, studentIDs []int64, from, to timezone.Date) ([]*Appointment, error)
	ListOrganizedByStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*Appointment, error)
	// ListCancellationTombstonesForGuardianProfiles returns guardian-visible
	// appointments cancelled OR soft-deleted on/after `since`, regardless of their
	// event dates — the feed re-exports them as STATUS:CANCELLED so subscribers
	// purge them. Retention is bounded by the cancellation/deletion time, not by
	// the date lookback window.
	ListCancellationTombstonesForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, studentIDs []int64, since time.Time) ([]*Appointment, error)
	// ListGuardianReminderCandidates returns the live, un-cancelled appointments
	// of the current tenant that could produce a guardian-facing occurrence in
	// the window: they carry the organizer's "notify guardians" opt-in and have
	// at least one guardian recipient. Recurrence is only bounded here (the same
	// window predicate the calendar listings use) — which concrete occurrences
	// fall due is decided in the service, which owns occurrence expansion.
	ListGuardianReminderCandidates(ctx context.Context, from, to timezone.Date) ([]*Appointment, error)
	// LockReminderCandidate re-reads a reminder candidate while holding its row
	// lock. It returns nil when a lifecycle transition has made the appointment
	// ineligible since the scheduler's coarse candidate scan.
	LockReminderCandidate(ctx context.Context, id int64) (*Appointment, error)
}

type RecurrenceRuleRepository interface {
	Create(ctx context.Context, rule *RecurrenceRule) error
	FindByAppointmentID(ctx context.Context, appointmentID int64) (*RecurrenceRule, error)
	FindByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*RecurrenceRule, error)
	DeleteByAppointmentID(ctx context.Context, appointmentID int64) error
}

type AppointmentRecipientRepository interface {
	CreateMany(ctx context.Context, recipients []*AppointmentRecipient) error
	FindByID(ctx context.Context, id int64) (*AppointmentRecipient, error)
	ReplaceForAppointment(ctx context.Context, appointmentID int64, recipients []*AppointmentRecipient) error
	FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*AppointmentRecipient, error)
	UpdateResponse(ctx context.Context, recipientID int64, status string) error
	// ClaimReminderPush records the one push delivery allowed for an appointment
	// revision, occurrence, and guardian. It returns false when a prior scheduler
	// scan already claimed the same delivery.
	ClaimReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate timezone.Date, guardianProfileID int64) (bool, error)
	ReleaseReminderPush(ctx context.Context, appointmentID int64, revision int, occurrenceDate timezone.Date, guardianProfileID int64) error
}

type AppointmentRecipientStudentRepository interface {
	CreateMany(ctx context.Context, links []*AppointmentRecipientStudent) error
	FindByRecipientIDs(ctx context.Context, recipientIDs []int64) ([]*AppointmentRecipientStudent, error)
}

type AppointmentTargetRepository interface {
	ReplaceForAppointment(ctx context.Context, appointmentID int64, targets []*AppointmentTarget) error
	FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*AppointmentTarget, error)
}

type AppointmentOccurrenceOverrideRepository interface {
	Create(ctx context.Context, override *AppointmentOccurrenceOverride) error
	Update(ctx context.Context, override *AppointmentOccurrenceOverride) error
	FindByAppointmentIDsAndOccurrenceDates(ctx context.Context, appointmentIDs []int64, occurrenceDates []timezone.Date) ([]*AppointmentOccurrenceOverride, error)
	// FindByAppointmentIDsAndStartDates returns moved occurrences whose effective
	// start falls in the supplied dates. Reminder scans use it to find an
	// occurrence moved outside its rule's normal expansion window.
	FindByAppointmentIDsAndStartDates(ctx context.Context, appointmentIDs []int64, startDates []timezone.Date) ([]*AppointmentOccurrenceOverride, error)
	// FindCancelledByAppointmentIDs returns every cancelled occurrence override
	// for the given appointments — used to emit iCalendar EXDATEs so subscribed
	// external calendars drop occurrences cancelled via "Nur diesen Termin".
	FindCancelledByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*AppointmentOccurrenceOverride, error)
	// CancelOccurrence marks the occurrence cancelled, inserting the override or
	// updating an existing one. It is conflict-safe (INSERT ... ON CONFLICT DO
	// UPDATE), so two concurrent cancellations of the same occurrence converge
	// instead of one hitting the (tenant_id, appointment_id, occurrence_date)
	// unique constraint and failing.
	CancelOccurrence(ctx context.Context, appointmentID int64, occurrenceDate timezone.Date) error
	// DeleteByAppointmentID removes every occurrence override for an appointment.
	// Used when a series edit replaces the recurrence rule wholesale so stale
	// per-occurrence cancellations from the old cadence cannot suppress valid
	// occurrences (or leak EXDATEs) in the edited series.
	DeleteByAppointmentID(ctx context.Context, appointmentID int64) error
	Delete(ctx context.Context, id any) error
}
