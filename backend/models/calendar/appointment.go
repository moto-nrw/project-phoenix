package calendar

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	DeliveryModeRSVPRequired  = "rsvp_required"
	DeliveryModeInformational = "informational"

	OverviewVisibilityOrganizer = "organizer"
	OverviewVisibilityStaff     = "staff"
	OverviewVisibilityAll       = "all"

	RecipientTypeStaff           = "staff"
	RecipientTypeGuardianProfile = "guardian_profile"

	ResponseStatusPending  = "pending"
	ResponseStatusAccepted = "accepted"
	ResponseStatusDeclined = "declined"
	ResponseStatusInfo     = "info"

	EventSourceAppointment = "appointment"
	EventSourceTimetable   = "timetable"
	EventSourceShift       = "shift"

	TargetTypeStaff            = "staff"
	TargetTypeGuardianProfile  = "guardian_profile"
	TargetTypeAllStaff         = "all_staff"
	TargetTypeAllSchoolParents = "all_school_parents"
	TargetTypeParentsByClass   = "parents_by_class"
	TargetTypeParentsByGroup   = "parents_by_group"
	TargetTypeParentsByStudent = "parents_by_student"

	RecurrenceFrequencyDaily   = "daily"
	RecurrenceFrequencyWeekly  = "weekly"
	RecurrenceFrequencyMonthly = "monthly"
	RecurrenceFrequencyYearly  = "yearly"

	// MaxRecurrenceOccurrenceCount caps a count-bounded recurrence so occurrence
	// expansion stays bounded (a leap-year of daily occurrences is the most a
	// realistic appointment needs; EndsOn covers longer ranges).
	MaxRecurrenceOccurrenceCount = 366
)

// validRecurrenceWeekdays holds the lowercase day names produced by
// time.Weekday.String(). The occurrence matcher (services/calendar) compares
// candidate.Weekday().String() lowercased against the rule's weekdays, so a
// value outside this set silently matches nothing — reject it at validation
// time instead of letting the appointment appear to have no occurrences.
var validRecurrenceWeekdays = map[string]bool{
	"monday":    true,
	"tuesday":   true,
	"wednesday": true,
	"thursday":  true,
	"friday":    true,
	"saturday":  true,
	"sunday":    true,
}

type Appointment struct {
	base.Model `bun:"schema:calendar,table:appointments"`
	base.TenantModel

	OrganizerStaffID   int64         `bun:"organizer_staff_id,notnull" json:"organizer_staff_id"`
	Title              string        `bun:"title,notnull" json:"title"`
	Description        *string       `bun:"description" json:"description,omitempty"`
	Location           *string       `bun:"location" json:"location,omitempty"`
	StartDate          timezone.Date `bun:"start_date,notnull" json:"start_date"`
	EndDate            timezone.Date `bun:"end_date,notnull" json:"end_date"`
	StartTime          time.Time     `bun:"start_time,notnull" json:"start_time"`
	EndTime            time.Time     `bun:"end_time,notnull" json:"end_time"`
	AllDay             bool          `bun:"all_day,notnull,default:false" json:"all_day"`
	DeliveryMode       string        `bun:"delivery_mode,notnull" json:"delivery_mode"`
	OverviewVisibility string        `bun:"overview_visibility,notnull,default:'organizer'" json:"overview_visibility"`
	CancelledAt        *time.Time    `bun:"cancelled_at" json:"cancelled_at,omitempty"`
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

// ErrAppointmentLifecycleConflict is returned by the repository when a content
// update matches zero rows because the appointment was cancelled or deleted by a
// concurrent request between load and write. The service maps it to a conflict.
var ErrAppointmentLifecycleConflict = errors.New("appointment changed by a concurrent lifecycle transition")

func (a *Appointment) Validate() error {
	if a.OrganizerStaffID <= 0 {
		return errors.New("organizer_staff_id is required")
	}
	a.Title = strings.TrimSpace(a.Title)
	if a.Title == "" {
		return errors.New("title is required")
	}
	if a.StartDate.IsZero() {
		return errors.New("start_date is required")
	}
	if a.EndDate.IsZero() {
		return errors.New("end_date is required")
	}
	if a.EndDate.Before(a.StartDate) {
		return errors.New("end_date must be on or after start_date")
	}
	if !a.AllDay && !timezone.WallClock(a.EndTime).After(timezone.WallClock(a.StartTime)) && a.StartDate == a.EndDate {
		return errors.New("end_time must be after start_time on same-day appointments")
	}
	switch a.DeliveryMode {
	case DeliveryModeRSVPRequired, DeliveryModeInformational:
	default:
		return errors.New("delivery_mode must be rsvp_required or informational")
	}
	if a.OverviewVisibility == "" {
		a.OverviewVisibility = OverviewVisibilityOrganizer
	}
	switch a.OverviewVisibility {
	case OverviewVisibilityOrganizer, OverviewVisibilityStaff, OverviewVisibilityAll:
		return nil
	default:
		return errors.New("overview_visibility must be organizer, staff, or all")
	}
}

type RecurrenceRule struct {
	base.Model `bun:"schema:calendar,table:recurrence_rules"`
	base.TenantModel

	AppointmentID   int64          `bun:"appointment_id,notnull" json:"appointment_id"`
	Frequency       string         `bun:"frequency,notnull" json:"frequency"`
	IntervalCount   int            `bun:"interval_count,notnull,default:1" json:"interval_count"`
	Weekdays        []string       `bun:"weekdays,array" json:"weekdays,omitempty"`
	MonthDays       []int          `bun:"month_days,array" json:"month_days,omitempty"`
	EndsOn          *timezone.Date `bun:"ends_on" json:"ends_on,omitempty"`
	OccurrenceCount *int           `bun:"occurrence_count" json:"occurrence_count,omitempty"`
}

func (r *RecurrenceRule) Validate() error {
	if r.AppointmentID <= 0 {
		return errors.New("appointment_id is required")
	}
	switch r.Frequency {
	case RecurrenceFrequencyDaily, RecurrenceFrequencyWeekly, RecurrenceFrequencyMonthly, RecurrenceFrequencyYearly:
	default:
		return errors.New("invalid recurrence frequency")
	}
	if r.IntervalCount <= 0 {
		return errors.New("interval_count must be positive")
	}
	if r.EndsOn != nil && r.OccurrenceCount != nil {
		return errors.New("only one recurrence end mode is allowed")
	}
	if r.OccurrenceCount != nil && *r.OccurrenceCount <= 0 {
		return errors.New("occurrence_count must be positive")
	}
	// Bound the count so occurrence expansion (feed overlap checks, single-
	// occurrence validation) can never scan an unbounded number of periods.
	if r.OccurrenceCount != nil && *r.OccurrenceCount > MaxRecurrenceOccurrenceCount {
		return errors.New("occurrence_count exceeds the maximum of 366")
	}
	// Normalise (lowercase) and de-duplicate weekdays: a duplicate would make the
	// occurrence expansion emit the same date twice, exhausting occurrence_count
	// early, and would export an invalid BYDAY=MO,MO in the RRULE.
	if len(r.Weekdays) > 0 {
		seen := make(map[string]bool, len(r.Weekdays))
		deduped := make([]string, 0, len(r.Weekdays))
		for _, weekday := range r.Weekdays {
			normalized := strings.ToLower(strings.TrimSpace(weekday))
			if !validRecurrenceWeekdays[normalized] {
				return errors.New("weekdays must be valid day names (monday–sunday)")
			}
			if seen[normalized] {
				continue
			}
			seen[normalized] = true
			deduped = append(deduped, normalized)
		}
		r.Weekdays = deduped
	}
	// Same for month days — a duplicate day double-counts against occurrence_count.
	if len(r.MonthDays) > 0 {
		seen := make(map[int]bool, len(r.MonthDays))
		deduped := make([]int, 0, len(r.MonthDays))
		for _, day := range r.MonthDays {
			if day < 1 || day > 31 {
				return errors.New("month_days must be between 1 and 31")
			}
			if seen[day] {
				continue
			}
			seen[day] = true
			deduped = append(deduped, day)
		}
		r.MonthDays = deduped
	}
	return nil
}

type AppointmentRecipient struct {
	base.Model `bun:"schema:calendar,table:appointment_recipients"`
	base.TenantModel

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
	base.Model `bun:"schema:calendar,table:appointment_recipient_students"`
	base.TenantModel

	RecipientID int64 `bun:"recipient_id,notnull" json:"recipient_id"`
	StudentID   int64 `bun:"student_id,notnull" json:"student_id"`
}

type AppointmentTarget struct {
	base.Model `bun:"schema:calendar,table:appointment_targets"`
	base.TenantModel

	AppointmentID int64   `bun:"appointment_id,notnull" json:"appointment_id"`
	TargetType    string  `bun:"target_type,notnull" json:"target_type"`
	TargetID      *int64  `bun:"target_id" json:"target_id,omitempty"`
	TargetValue   *string `bun:"target_value" json:"target_value,omitempty"`
}

type AppointmentOccurrenceOverride struct {
	base.Model `bun:"schema:calendar,table:appointment_occurrence_overrides"`
	base.TenantModel

	AppointmentID  int64          `bun:"appointment_id,notnull" json:"appointment_id"`
	OccurrenceDate timezone.Date  `bun:"occurrence_date,notnull" json:"occurrence_date"`
	Cancelled      bool           `bun:"cancelled,notnull,default:false" json:"cancelled"`
	Title          *string        `bun:"title" json:"title,omitempty"`
	Description    *string        `bun:"description" json:"description,omitempty"`
	Location       *string        `bun:"location" json:"location,omitempty"`
	StartDate      *timezone.Date `bun:"start_date" json:"start_date,omitempty"`
	EndDate        *timezone.Date `bun:"end_date" json:"end_date,omitempty"`
	StartTime      *time.Time     `bun:"start_time" json:"start_time,omitempty"`
	EndTime        *time.Time     `bun:"end_time" json:"end_time,omitempty"`
	AllDay         *bool          `bun:"all_day" json:"all_day,omitempty"`
}
