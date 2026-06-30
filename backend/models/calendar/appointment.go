package calendar

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	DeliveryModeRSVPRequired  = "rsvp_required"
	DeliveryModeInformational = "informational"

	RecipientTypeStaff           = "staff"
	RecipientTypeGuardianProfile = "guardian_profile"

	ResponseStatusPending  = "pending"
	ResponseStatusAccepted = "accepted"
	ResponseStatusDeclined = "declined"
	ResponseStatusInfo     = "info"

	EventSourceAppointment = "appointment"
	EventSourceTimetable   = "timetable"

	TargetTypeStaff            = "staff"
	TargetTypeGuardianProfile  = "guardian_profile"
	TargetTypeAllStaff         = "all_staff"
	TargetTypeParentsByClass   = "parents_by_class"
	TargetTypeParentsByGroup   = "parents_by_group"
	TargetTypeParentsByStudent = "parents_by_student"

	RecurrenceFrequencyDaily   = "daily"
	RecurrenceFrequencyWeekly  = "weekly"
	RecurrenceFrequencyMonthly = "monthly"
	RecurrenceFrequencyYearly  = "yearly"
)

type Appointment struct {
	base.Model `bun:"schema:calendar,table:appointments"`
	base.TenantModel

	OrganizerStaffID int64         `bun:"organizer_staff_id,notnull" json:"organizer_staff_id"`
	Title            string        `bun:"title,notnull" json:"title"`
	Description      *string       `bun:"description" json:"description,omitempty"`
	Location         *string       `bun:"location" json:"location,omitempty"`
	StartDate        timezone.Date `bun:"start_date,notnull" json:"start_date"`
	EndDate          timezone.Date `bun:"end_date,notnull" json:"end_date"`
	StartTime        time.Time     `bun:"start_time,notnull" json:"start_time"`
	EndTime          time.Time     `bun:"end_time,notnull" json:"end_time"`
	AllDay           bool          `bun:"all_day,notnull,default:false" json:"all_day"`
	DeliveryMode     string        `bun:"delivery_mode,notnull" json:"delivery_mode"`
	CancelledAt      *time.Time    `bun:"cancelled_at" json:"cancelled_at,omitempty"`
}

func (a *Appointment) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`calendar.appointments AS "appointment"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`calendar.appointments AS "appointment"`)
	}
	return nil
}

func (a *Appointment) TableName() string { return "calendar.appointments" }

func (a *Appointment) GetID() any              { return a.ID }
func (a *Appointment) GetCreatedAt() time.Time { return a.CreatedAt }
func (a *Appointment) GetUpdatedAt() time.Time { return a.UpdatedAt }

func (a *Appointment) Validate() error {
	if a.OrganizerStaffID <= 0 {
		return errors.New("organizer_staff_id is required")
	}
	a.Title = strings.TrimSpace(a.Title)
	if a.Title == "" {
		return errors.New("title is required")
	}
	if a.EndDate.Before(a.StartDate) {
		return errors.New("end_date must be on or after start_date")
	}
	if !a.AllDay && !timezone.WallClock(a.EndTime).After(timezone.WallClock(a.StartTime)) && a.StartDate == a.EndDate {
		return errors.New("end_time must be after start_time on same-day appointments")
	}
	switch a.DeliveryMode {
	case DeliveryModeRSVPRequired, DeliveryModeInformational:
		return nil
	default:
		return errors.New("delivery_mode must be rsvp_required or informational")
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

func (r *RecurrenceRule) TableName() string       { return "calendar.recurrence_rules" }
func (r *RecurrenceRule) GetID() any              { return r.ID }
func (r *RecurrenceRule) GetCreatedAt() time.Time { return r.CreatedAt }
func (r *RecurrenceRule) GetUpdatedAt() time.Time { return r.UpdatedAt }

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

func (r *AppointmentRecipient) TableName() string       { return "calendar.appointment_recipients" }
func (r *AppointmentRecipient) GetID() any              { return r.ID }
func (r *AppointmentRecipient) GetCreatedAt() time.Time { return r.CreatedAt }
func (r *AppointmentRecipient) GetUpdatedAt() time.Time { return r.UpdatedAt }

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

func (r *AppointmentRecipientStudent) TableName() string {
	return "calendar.appointment_recipient_students"
}
func (r *AppointmentRecipientStudent) GetID() any              { return r.ID }
func (r *AppointmentRecipientStudent) GetCreatedAt() time.Time { return r.CreatedAt }
func (r *AppointmentRecipientStudent) GetUpdatedAt() time.Time { return r.UpdatedAt }

type AppointmentTarget struct {
	base.Model `bun:"schema:calendar,table:appointment_targets"`
	base.TenantModel

	AppointmentID int64   `bun:"appointment_id,notnull" json:"appointment_id"`
	TargetType    string  `bun:"target_type,notnull" json:"target_type"`
	TargetID      *int64  `bun:"target_id" json:"target_id,omitempty"`
	TargetValue   *string `bun:"target_value" json:"target_value,omitempty"`
}

func (t *AppointmentTarget) TableName() string       { return "calendar.appointment_targets" }
func (t *AppointmentTarget) GetID() any              { return t.ID }
func (t *AppointmentTarget) GetCreatedAt() time.Time { return t.CreatedAt }
func (t *AppointmentTarget) GetUpdatedAt() time.Time { return t.UpdatedAt }

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

func (o *AppointmentOccurrenceOverride) TableName() string {
	return "calendar.appointment_occurrence_overrides"
}
func (o *AppointmentOccurrenceOverride) GetID() any              { return o.ID }
func (o *AppointmentOccurrenceOverride) GetCreatedAt() time.Time { return o.CreatedAt }
func (o *AppointmentOccurrenceOverride) GetUpdatedAt() time.Time { return o.UpdatedAt }
