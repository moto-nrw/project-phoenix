package calendar

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type AppointmentRepository interface {
	Create(ctx context.Context, appointment *Appointment) error
	FindByID(ctx context.Context, id int64) (*Appointment, error)
	Update(ctx context.Context, appointment *Appointment) error
	// BumpRevision advances the revision counter without touching other fields,
	// used when a change affecting the exported calendar lives in a child table.
	BumpRevision(ctx context.Context, appointmentID int64) error
	Delete(ctx context.Context, id any) error
	// SoftDelete marks the appointment deleted_at=now and bumps the revision. Used
	// for feed-visible appointments so they vanish from interactive calendars but
	// remain exportable as a durable STATUS:CANCELLED tombstone.
	SoftDelete(ctx context.Context, appointmentID int64) error
	// ListVisible*/ListOrganized* return only live (deleted_at IS NULL) rows.
	ListVisibleForStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*Appointment, error)
	ListVisibleForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, studentIDs []int64, from, to timezone.Date) ([]*Appointment, error)
	ListOrganizedByStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*Appointment, error)
	// ListDeletedTombstonesForGuardianProfiles returns guardian-visible
	// appointments soft-deleted on/after deletedSince, regardless of their event
	// dates — the feed re-exports them as STATUS:CANCELLED so subscribers purge
	// them. Retention is bounded by deletedSince, not by the date lookback window.
	ListDeletedTombstonesForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, studentIDs []int64, deletedSince time.Time) ([]*Appointment, error)
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
