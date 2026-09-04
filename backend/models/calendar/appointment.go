package calendar

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
)

const (
	DeliveryModeRSVPRequired  = appointments.DeliveryModeRSVPRequired
	DeliveryModeInformational = appointments.DeliveryModeInformational

	OverviewVisibilityOrganizer = appointments.OverviewVisibilityOrganizer
	OverviewVisibilityStaff     = appointments.OverviewVisibilityStaff
	OverviewVisibilityAll       = appointments.OverviewVisibilityAll

	RecipientTypeStaff           = "staff"
	RecipientTypeGuardianProfile = "guardian_profile"

	ResponseStatusPending  = "pending"
	ResponseStatusAccepted = "accepted"
	ResponseStatusDeclined = "declined"
	ResponseStatusInfo     = "info"

	EventSourceAppointment = "appointment"
	EventSourceTimetable   = "timetable"
	EventSourceShift       = "shift"

	TargetTypeStaff            = appointments.TargetTypeStaff
	TargetTypeGuardianProfile  = appointments.TargetTypeGuardianProfile
	TargetTypeAllStaff         = appointments.TargetTypeAllStaff
	TargetTypeAllSchoolParents = appointments.TargetTypeAllSchoolParents
	TargetTypeParentsByClass   = appointments.TargetTypeParentsByClass
	TargetTypeParentsByGroup   = appointments.TargetTypeParentsByGroup
	TargetTypeParentsByStudent = appointments.TargetTypeParentsByStudent

	RecurrenceFrequencyDaily   = appointments.RecurrenceFrequencyDaily
	RecurrenceFrequencyWeekly  = appointments.RecurrenceFrequencyWeekly
	RecurrenceFrequencyMonthly = appointments.RecurrenceFrequencyMonthly
	RecurrenceFrequencyYearly  = appointments.RecurrenceFrequencyYearly

	MaxRecurrenceOccurrenceCount = appointments.MaxRecurrenceOccurrenceCount
)

// Appointment is the legacy transport shape. Persistence and lifecycle rules
// are owned by modules/appointments; this type remains while the calendar HTTP
// surface is migrated without changing its JSON contract.
type Appointment struct {
	Model `bun:"schema:calendar,table:appointments"`
	TenantModel

	OrganizerStaffID   int64      `bun:"organizer_staff_id,notnull" json:"organizer_staff_id"`
	Title              string     `bun:"title,notnull" json:"title"`
	Description        *string    `bun:"description" json:"description,omitempty"`
	Location           *string    `bun:"location" json:"location,omitempty"`
	StartDate          Date       `bun:"start_date,notnull" json:"start_date"`
	EndDate            Date       `bun:"end_date,notnull" json:"end_date"`
	StartTime          time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime            time.Time  `bun:"end_time,notnull" json:"end_time"`
	AllDay             bool       `bun:"all_day,notnull,default:false" json:"all_day"`
	DeliveryMode       string     `bun:"delivery_mode,notnull" json:"delivery_mode"`
	OverviewVisibility string     `bun:"overview_visibility,notnull,default:'organizer'" json:"overview_visibility"`
	CancelledAt        *time.Time `bun:"cancelled_at" json:"cancelled_at,omitempty"`
	// DeletedAt marks a feed-visible appointment that the organizer deleted. It is
	// distinct from CancelledAt (the "Absagen" action, which stays visible in
	// interactive calendars): a deleted appointment is hidden from every
	// interactive listing but exported to the subscription feed as a durable
	// STATUS:CANCELLED tombstone so offline subscribers eventually purge it. Not a
	// bun soft_delete tag — the feed must be able to SELECT deleted rows, so the
	// deleted_at IS NULL / IS NOT NULL filtering is done explicitly per query.
	DeletedAt *time.Time `bun:"deleted_at" json:"deleted_at,omitempty"`
	// NotifyGuardians persists the organizer's current send_email choice, which
	// controls lifecycle e-mails and reminder eligibility. No bun `default:` —
	// that would make bun omit the `false` zero value on INSERT and let the column
	// default (TRUE) win, silently re-enabling mail for an opted-out appointment.
	NotifyGuardians bool `bun:"notify_guardians,notnull" json:"notify_guardians"`
	// Revision is a monotonically increasing change counter used as the
	// iCalendar SEQUENCE, so subscribed clients recognise edits/cancellations as
	// newer revisions instead of retaining stale events.
	Revision int `bun:"revision,notnull,default:0" json:"revision"`
}

// ErrAppointmentLifecycleConflict is returned by the Appointments capability
// when a content update matches zero rows because the appointment was cancelled
// or deleted by a concurrent request between load and write.
var ErrAppointmentLifecycleConflict = appointments.ErrAppointmentLifecycleConflict

func (a *Appointment) Validate() error {
	if a == nil {
		return errors.New("appointment cannot be nil")
	}
	value := &appointments.Appointment{
		OrganizerStaffID: a.OrganizerStaffID, Title: a.Title, Description: a.Description,
		Location: a.Location, StartDate: a.StartDate, EndDate: a.EndDate,
		StartTime: a.StartTime, EndTime: a.EndTime, AllDay: a.AllDay,
		DeliveryMode: a.DeliveryMode, OverviewVisibility: a.OverviewVisibility,
		NotifyGuardians: a.NotifyGuardians,
	}
	if err := value.Validate(); err != nil {
		return err
	}
	a.Title = value.Title
	a.OverviewVisibility = value.OverviewVisibility
	return nil
}

// RecurrenceRule remains as a compatibility alias for calendar transport and
// expansion logic. Appointments owns validation and persistence.
type RecurrenceRule = appointments.RecurrenceRule

type AppointmentRecipient struct {
	Model `bun:"schema:calendar,table:appointment_recipients"`
	TenantModel

	AppointmentID     int64      `bun:"appointment_id,notnull" json:"appointment_id"`
	RecipientType     string     `bun:"recipient_type,notnull" json:"recipient_type"`
	StaffID           *int64     `bun:"staff_id" json:"staff_id,omitempty"`
	GuardianProfileID *int64     `bun:"guardian_profile_id" json:"guardian_profile_id,omitempty"`
	Status            string     `bun:"status,notnull" json:"status"`
	RespondedAt       *time.Time `bun:"responded_at" json:"responded_at,omitempty"`
}

func (r *AppointmentRecipient) Validate() error {
	if r.AppointmentID <= 0 {
		return errors.New("appointment_id is required")
	}
	switch r.RecipientType {
	case RecipientTypeStaff:
		if r.StaffID == nil || *r.StaffID <= 0 || r.GuardianProfileID != nil {
			return errors.New("staff recipient requires staff_id only")
		}
	case RecipientTypeGuardianProfile:
		if r.GuardianProfileID == nil || *r.GuardianProfileID <= 0 || r.StaffID != nil {
			return errors.New("guardian recipient requires guardian_profile_id only")
		}
	default:
		return errors.New("invalid recipient_type")
	}
	switch r.Status {
	case ResponseStatusPending, ResponseStatusAccepted, ResponseStatusDeclined, ResponseStatusInfo:
		return nil
	default:
		return errors.New("invalid recipient status")
	}
}

type AppointmentRecipientStudent struct {
	Model `bun:"schema:calendar,table:appointment_recipient_students"`
	TenantModel

	RecipientID int64 `bun:"recipient_id,notnull" json:"recipient_id"`
	StudentID   int64 `bun:"student_id,notnull" json:"student_id"`
}

type AppointmentTarget struct {
	Model `bun:"schema:calendar,table:appointment_targets"`
	TenantModel

	AppointmentID int64   `bun:"appointment_id,notnull" json:"appointment_id"`
	TargetType    string  `bun:"target_type,notnull" json:"target_type"`
	TargetID      *int64  `bun:"target_id" json:"target_id,omitempty"`
	TargetValue   *string `bun:"target_value" json:"target_value,omitempty"`
}

// AppointmentOccurrenceOverride remains as a compatibility alias for calendar
// transport and expansion logic. Appointments owns persistence.
type AppointmentOccurrenceOverride = appointments.AppointmentOccurrenceOverride
