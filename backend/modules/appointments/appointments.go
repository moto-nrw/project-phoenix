// Package appointments is the public Appointments capability. It owns the
// appointment lifecycle, targeting intent, resolved recipients, replies, and
// reminder-delivery claims. Other owners use Query or Command instead of
// reading the owned calendar tables directly.
package appointments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

var berlin = mustBerlinLocation()

const (
	DeliveryModeRSVPRequired  = "rsvp_required"
	DeliveryModeInformational = "informational"

	OverviewVisibilityOrganizer = "organizer"
	OverviewVisibilityStaff     = "staff"
	OverviewVisibilityAll       = "all"

	TargetTypeStaff           = "staff"
	TargetTypeGuardianProfile = "guardian_profile"
	TargetTypeAllStaff        = "all_staff"
	// TargetTypeAllTeachers reaches the Lehrkräfte of the school in moto
	// schule (#4798): the staff whose account holds a school-portal role.
	TargetTypeAllTeachers      = "all_teachers"
	TargetTypeAllSchoolParents = "all_school_parents"
	TargetTypeParentsByClass   = "parents_by_class"
	TargetTypeParentsByGroup   = "parents_by_group"
	TargetTypeParentsByStudent = "parents_by_student"

	RecipientTypeStaff           = "staff"
	RecipientTypeGuardianProfile = "guardian_profile"

	ResponseStatusPending  = "pending"
	ResponseStatusAccepted = "accepted"
	ResponseStatusDeclined = "declined"
	ResponseStatusInfo     = "info"

	RecurrenceFrequencyDaily   = "daily"
	RecurrenceFrequencyWeekly  = "weekly"
	RecurrenceFrequencyMonthly = "monthly"
	RecurrenceFrequencyYearly  = "yearly"

	// School event types mark a Schultermin a Lehrkraft or the school
	// administration entered in moto schule (#4798). An appointment without
	// one is an OGS appointment.
	SchoolEventTypeTrip         = "ausflug"
	SchoolEventTypePedagogicDay = "paedagogischer_tag"
	SchoolEventTypeLessonsEnd   = "unterrichtsende"
	SchoolEventTypeOther        = "sonstiges"
	// SchoolEventTypeParentConference marks the appointment of a booked
	// Elternsprechtag slot (#4801). Only a booking writes it; nobody enters
	// it by hand, so ValidSchoolEventType does not offer it.
	SchoolEventTypeParentConference = "elternsprechtag"

	// MaxRecurrenceOccurrenceCount keeps count-bounded expansion finite. Longer
	// series use EndsOn instead.
	MaxRecurrenceOccurrenceCount = 366
)

var validRecurrenceWeekdays = map[string]bool{
	"monday": true, "tuesday": true, "wednesday": true, "thursday": true,
	"friday": true, "saturday": true, "sunday": true,
}

var (
	ErrAppointmentNotFound          = errors.New("appointment not found")
	ErrAppointmentRecipientNotFound = errors.New("appointment recipient not found")
	ErrInvalidAppointment           = errors.New("invalid appointment")
	ErrAppointmentLifecycleConflict = errors.New("appointment changed by a concurrent lifecycle transition")
)

// InvalidError carries a validation reason while preserving a stable error
// identity for callers and metrics.
type InvalidError struct{ Reason string }

func (e *InvalidError) Error() string { return e.Reason }
func (e *InvalidError) Unwrap() error { return ErrInvalidAppointment }

// Date is a calendar day. Its canonical representation cannot carry a clock
// or timezone and therefore cannot shift while crossing the Postgres adapter.
type Date string

func NewDate(year int, month time.Month, day int) Date {
	return Date(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(dateLayout))
}

func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil || parsed.Format(dateLayout) != value {
		return "", fmt.Errorf("invalid calendar date %q", value)
	}
	return Date(value), nil
}

func (d Date) String() string         { return string(d) }
func (d Date) IsZero() bool           { return d == "" }
func (d Date) Before(other Date) bool { return d < other }
func (d Date) After(other Date) bool  { return d > other }
func (d Date) AddDays(days int) Date {
	value := d.UTCMidnight().AddDate(0, 0, days)
	return NewDate(value.Year(), value.Month(), value.Day())
}
func (d Date) DaysUntil(other Date) int {
	return int(other.UTCMidnight().Sub(d.UTCMidnight()) / (24 * time.Hour))
}
func (d Date) Weekday() time.Weekday { return d.UTCMidnight().Weekday() }
func (d Date) Year() int             { return d.UTCMidnight().Year() }
func (d Date) Month() time.Month     { return d.UTCMidnight().Month() }
func (d Date) Day() int              { return d.UTCMidnight().Day() }
func (d Date) Format(layout string) string {
	return d.UTCMidnight().Format(layout)
}
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

func (d Date) UTCMidnight() time.Time {
	parsed, _ := time.Parse(dateLayout, d.String())
	return parsed
}

func (d Date) BerlinMidnight() time.Time {
	parsed := d.UTCMidnight()
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, berlin)
}

func (d Date) EndOfDay() time.Time {
	parsed := d.UTCMidnight()
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, berlin)
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

func (d *Date) UnmarshalText(value []byte) error {
	parsed, err := ParseDate(string(value))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d *Date) UnmarshalJSON(value []byte) error {
	if string(value) == "null" {
		*d = ""
		return nil
	}
	var raw string
	if err := json.Unmarshal(value, &raw); err != nil {
		return err
	}
	return d.UnmarshalText([]byte(raw))
}

func mustBerlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return location
}

type Appointment struct {
	ID                 int64      `json:"id"`
	TenantID           int64      `json:"tenant_id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	OrganizerStaffID   int64      `json:"organizer_staff_id"`
	Title              string     `json:"title"`
	Description        *string    `json:"description,omitempty"`
	Location           *string    `json:"location,omitempty"`
	StartDate          Date       `json:"start_date"`
	EndDate            Date       `json:"end_date"`
	StartTime          time.Time  `json:"start_time"`
	EndTime            time.Time  `json:"end_time"`
	AllDay             bool       `json:"all_day"`
	DeliveryMode       string     `json:"delivery_mode"`
	OverviewVisibility string     `json:"overview_visibility"`
	CancelledAt        *time.Time `json:"cancelled_at,omitempty"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
	NotifyGuardians    bool       `json:"notify_guardians"`
	SchoolEventType    *string    `json:"school_event_type,omitempty"`
	Revision           int        `json:"revision"`
}

type AppointmentFields struct {
	OrganizerStaffID   int64
	Title              string
	Description        *string
	Location           *string
	StartDate          Date
	EndDate            Date
	StartTime          time.Time
	EndTime            time.Time
	AllDay             bool
	DeliveryMode       string
	OverviewVisibility string
	NotifyGuardians    bool
	SchoolEventType    *string
}

func (a *Appointment) Validate() error {
	if a == nil {
		return invalid("appointment cannot be nil")
	}
	fields, err := ValidateAppointmentFields(fieldsFromAppointment(a))
	if err != nil {
		return err
	}
	applyFields(a, fields)
	return nil
}

type AppointmentTarget struct {
	ID            int64     `json:"id"`
	TenantID      int64     `json:"tenant_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	AppointmentID int64     `json:"appointment_id"`
	TargetType    string    `json:"target_type"`
	TargetID      *int64    `json:"target_id,omitempty"`
	TargetValue   *string   `json:"target_value,omitempty"`
}

type AppointmentTargetFields struct {
	TargetType  string
	TargetID    *int64
	TargetValue *string
}

type RecurrenceRule struct {
	ID              int64     `json:"id"`
	TenantID        int64     `json:"tenant_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	AppointmentID   int64     `json:"appointment_id"`
	Frequency       string    `json:"frequency"`
	IntervalCount   int       `json:"interval_count"`
	Weekdays        []string  `json:"weekdays,omitempty"`
	MonthDays       []int     `json:"month_days,omitempty"`
	EndsOn          *Date     `json:"ends_on,omitempty"`
	OccurrenceCount *int      `json:"occurrence_count,omitempty"`
}

func (r *RecurrenceRule) Validate() error {
	if r == nil {
		return invalid("recurrence rule cannot be nil")
	}
	if r.AppointmentID <= 0 {
		return invalid("appointment_id is required")
	}
	switch r.Frequency {
	case RecurrenceFrequencyDaily, RecurrenceFrequencyWeekly, RecurrenceFrequencyMonthly, RecurrenceFrequencyYearly:
	default:
		return invalid("invalid recurrence frequency")
	}
	if r.IntervalCount <= 0 {
		return invalid("interval_count must be positive")
	}
	if r.EndsOn != nil && r.OccurrenceCount != nil {
		return invalid("only one recurrence end mode is allowed")
	}
	if r.OccurrenceCount != nil && *r.OccurrenceCount <= 0 {
		return invalid("occurrence_count must be positive")
	}
	if r.OccurrenceCount != nil && *r.OccurrenceCount > MaxRecurrenceOccurrenceCount {
		return invalid("occurrence_count exceeds the maximum of 366")
	}
	weekdays, err := normalizeWeekdays(r.Weekdays)
	if err != nil {
		return err
	}
	r.Weekdays = weekdays
	monthDays, err := normalizeMonthDays(r.MonthDays)
	if err != nil {
		return err
	}
	r.MonthDays = monthDays
	return nil
}

type AppointmentOccurrenceOverride struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenant_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	AppointmentID  int64      `json:"appointment_id"`
	OccurrenceDate Date       `json:"occurrence_date"`
	Cancelled      bool       `json:"cancelled"`
	Title          *string    `json:"title,omitempty"`
	Description    *string    `json:"description,omitempty"`
	Location       *string    `json:"location,omitempty"`
	StartDate      *Date      `json:"start_date,omitempty"`
	EndDate        *Date      `json:"end_date,omitempty"`
	StartTime      *time.Time `json:"start_time,omitempty"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	AllDay         *bool      `json:"all_day,omitempty"`
}

type AppointmentRecipient struct {
	ID                int64      `json:"id"`
	TenantID          int64      `json:"tenant_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	AppointmentID     int64      `json:"appointment_id"`
	RecipientType     string     `json:"recipient_type"`
	StaffID           *int64     `json:"staff_id,omitempty"`
	GuardianProfileID *int64     `json:"guardian_profile_id,omitempty"`
	Status            string     `json:"status"`
	RespondedAt       *time.Time `json:"responded_at,omitempty"`
}

func (r *AppointmentRecipient) Validate() error {
	if r == nil {
		return invalid("appointment recipient cannot be nil")
	}
	if r.AppointmentID <= 0 {
		return invalid("appointment_id is required")
	}
	return ValidateRecipientFields(AppointmentRecipientFields{
		RecipientType: r.RecipientType, StaffID: r.StaffID,
		GuardianProfileID: r.GuardianProfileID, Status: r.Status,
	})
}

type AppointmentRecipientStudent struct {
	ID          int64     `json:"id"`
	TenantID    int64     `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	RecipientID int64     `json:"recipient_id"`
	StudentID   int64     `json:"student_id"`
}

// AppointmentRecipientFields is one resolved appointment audience member.
// StudentIDs is populated only for guardian recipients and becomes rows in
// calendar.appointment_recipient_students in the same UnitOfWork.
type AppointmentRecipientFields struct {
	RecipientType     string
	StaffID           *int64
	GuardianProfileID *int64
	Status            string
	StudentIDs        []int64
}

type CreateAppointment struct {
	AppointmentFields
	Targets []AppointmentTargetFields
}

type UpdateAppointment struct {
	ID int64
	AppointmentFields
}

// Query reads every Appointments record family; consumers that need one
// family depend on its narrow facade.
type Query interface {
	AppointmentQuery
	RecurrenceQuery
	RecipientQuery
}

// AppointmentQuery reads appointment rows: by ID, by calendar window, the
// reminder candidates and the targeting intent.
type AppointmentQuery interface {
	FindAppointment(context.Context, int64) (*Appointment, error)
	FindAppointmentForUpdate(context.Context, int64) (*Appointment, error)
	// FindReminderCandidateForUpdate returns nil without an error when a row
	// became ineligible after the candidate scan.
	FindReminderCandidateForUpdate(context.Context, int64) (*Appointment, error)
	// FindReminderCandidatesForUpdate is the batch form used by reminder scans.
	// Ineligible IDs are omitted and rows are locked in ID order.
	FindReminderCandidatesForUpdate(context.Context, []int64) ([]*Appointment, error)
	ListAppointmentsVisibleToStaff(context.Context, int64, Date, Date) ([]*Appointment, error)
	ListStaffCancellationTombstones(context.Context, int64, time.Time) ([]*Appointment, error)
	ListAppointmentsVisibleToGuardians(context.Context, []int64, []int64, Date, Date) ([]*Appointment, error)
	ListGuardianCancellationTombstones(context.Context, []int64, []int64, time.Time) ([]*Appointment, error)
	ListGuardianReminderCandidates(context.Context, Date, Date) ([]*Appointment, error)
	FindAppointmentTargets(context.Context, int64) ([]*AppointmentTarget, error)
}

// RecurrenceQuery reads recurrence rules and per-occurrence overrides.
type RecurrenceQuery interface {
	FindRecurrenceRule(context.Context, int64) (*RecurrenceRule, error)
	FindRecurrenceRules(context.Context, []int64) ([]*RecurrenceRule, error)
	FindOccurrenceOverrides(context.Context, []int64, []Date) ([]*AppointmentOccurrenceOverride, error)
	FindOccurrenceOverridesByStartDates(context.Context, []int64, []Date) ([]*AppointmentOccurrenceOverride, error)
	FindCancelledOccurrenceOverrides(context.Context, []int64) ([]*AppointmentOccurrenceOverride, error)
}

// RecipientQuery reads recipients and the students they were addressed
// through.
type RecipientQuery interface {
	FindAppointmentRecipient(context.Context, int64) (*AppointmentRecipient, error)
	FindAppointmentRecipients(context.Context, int64) ([]*AppointmentRecipient, error)
	FindAppointmentRecipientsByAppointmentIDs(context.Context, []int64) ([]*AppointmentRecipient, error)
	FindAppointmentRecipientStudents(context.Context, []int64) ([]*AppointmentRecipientStudent, error)
	CountAppointmentRecipientStudents(context.Context, int64) (int, error)
}

// Command writes every Appointments record family; consumers that need one
// family depend on its narrow facade.
type Command interface {
	AppointmentCommand
	RecurrenceCommand
	RecipientCommand
}

// AppointmentCommand writes appointment rows, their lifecycle and their
// targeting intent.
type AppointmentCommand interface {
	// CreateAppointment writes the appointment and its targeting intent in one
	// UnitOfWork. A failed target insert rolls back the appointment insert.
	CreateAppointment(context.Context, CreateAppointment) (*Appointment, []*AppointmentTarget, error)
	UpdateAppointment(context.Context, UpdateAppointment) (*Appointment, error)
	DeleteAppointment(context.Context, int64) error
	CancelAppointment(context.Context, int64) (bool, error)
	SoftDeleteAppointment(context.Context, int64) error
	DeleteFeedTombstonesBefore(context.Context, time.Time) (int, error)
	ReplaceAppointmentTargets(context.Context, int64, []AppointmentTargetFields) ([]*AppointmentTarget, error)
}

// RecurrenceCommand writes recurrence rules and per-occurrence overrides.
type RecurrenceCommand interface {
	CreateRecurrenceRule(context.Context, *RecurrenceRule) error
	DeleteRecurrenceRule(context.Context, int64) error
	CreateOccurrenceOverride(context.Context, *AppointmentOccurrenceOverride) error
	DeleteOccurrenceOverrides(context.Context, int64) error
	// CancelAppointmentOccurrence upserts the cancellation and bumps the parent
	// revision in one UnitOfWork. It returns false when already cancelled.
	CancelAppointmentOccurrence(context.Context, int64, Date) (bool, error)
}

// RecipientCommand writes recipients, their RSVPs and the reminder push
// delivery claims.
type RecipientCommand interface {
	// CreateAppointmentRecipients writes recipients and their student links in
	// one UnitOfWork. A failed link insert rolls back every recipient insert.
	CreateAppointmentRecipients(context.Context, int64, []AppointmentRecipientFields) ([]*AppointmentRecipient, []*AppointmentRecipientStudent, error)
	UpdateAppointmentRecipientResponse(context.Context, int64, string) error
	ClaimReminderPushDelivery(context.Context, int64, int, Date, int64) (bool, error)
	ReleaseReminderPushDelivery(context.Context, int64, int, Date, int64) error
}

type Capability interface {
	Query
	Command
}
