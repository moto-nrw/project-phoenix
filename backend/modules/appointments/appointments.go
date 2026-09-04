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
	"strings"
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

	TargetTypeStaff            = "staff"
	TargetTypeGuardianProfile  = "guardian_profile"
	TargetTypeAllStaff         = "all_staff"
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
}

func (a *Appointment) Validate() error {
	if a == nil {
		return invalid("appointment cannot be nil")
	}
	fields, err := validateFields(fieldsFromAppointment(a))
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
	return validateRecipientFields(AppointmentRecipientFields{
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

type Query interface {
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
	FindRecurrenceRule(context.Context, int64) (*RecurrenceRule, error)
	FindRecurrenceRules(context.Context, []int64) ([]*RecurrenceRule, error)
	FindOccurrenceOverrides(context.Context, []int64, []Date) ([]*AppointmentOccurrenceOverride, error)
	FindOccurrenceOverridesByStartDates(context.Context, []int64, []Date) ([]*AppointmentOccurrenceOverride, error)
	FindCancelledOccurrenceOverrides(context.Context, []int64) ([]*AppointmentOccurrenceOverride, error)
	FindAppointmentRecipient(context.Context, int64) (*AppointmentRecipient, error)
	FindAppointmentRecipients(context.Context, int64) ([]*AppointmentRecipient, error)
	FindAppointmentRecipientsByAppointmentIDs(context.Context, []int64) ([]*AppointmentRecipient, error)
	FindAppointmentRecipientStudents(context.Context, []int64) ([]*AppointmentRecipientStudent, error)
	CountAppointmentRecipientStudents(context.Context, int64) (int, error)
}

type Command interface {
	// CreateAppointment writes the appointment and its targeting intent in one
	// UnitOfWork. A failed target insert rolls back the appointment insert.
	CreateAppointment(context.Context, CreateAppointment) (*Appointment, []*AppointmentTarget, error)
	UpdateAppointment(context.Context, UpdateAppointment) (*Appointment, error)
	DeleteAppointment(context.Context, int64) error
	CancelAppointment(context.Context, int64) (bool, error)
	SoftDeleteAppointment(context.Context, int64) error
	DeleteFeedTombstonesBefore(context.Context, time.Time) (int, error)
	ReplaceAppointmentTargets(context.Context, int64, []AppointmentTargetFields) ([]*AppointmentTarget, error)
	CreateRecurrenceRule(context.Context, *RecurrenceRule) error
	DeleteRecurrenceRule(context.Context, int64) error
	CreateOccurrenceOverride(context.Context, *AppointmentOccurrenceOverride) error
	DeleteOccurrenceOverrides(context.Context, int64) error
	// CancelAppointmentOccurrence upserts the cancellation and bumps the parent
	// revision in one UnitOfWork. It returns false when already cancelled.
	CancelAppointmentOccurrence(context.Context, int64, Date) (bool, error)
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

type engine interface {
	Query
	Command
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("appointments: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindAppointment(ctx context.Context, id int64) (*Appointment, error) {
	if id <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return m.engine.FindAppointment(ctx, id)
}

func (m *Module) FindAppointmentForUpdate(ctx context.Context, id int64) (*Appointment, error) {
	if id <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return m.engine.FindAppointmentForUpdate(ctx, id)
}

func (m *Module) FindReminderCandidateForUpdate(ctx context.Context, id int64) (*Appointment, error) {
	if id <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return m.engine.FindReminderCandidateForUpdate(ctx, id)
}

func (m *Module) FindReminderCandidatesForUpdate(ctx context.Context, ids []int64) ([]*Appointment, error) {
	ids = positiveIDs(ids)
	if len(ids) == 0 {
		return []*Appointment{}, nil
	}
	return m.engine.FindReminderCandidatesForUpdate(ctx, ids)
}

func (m *Module) ListAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to Date) ([]*Appointment, error) {
	if staffID <= 0 || from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, invalid("staff ID and a valid appointment window are required")
	}
	return m.engine.ListAppointmentsVisibleToStaff(ctx, staffID, from, to)
}

func (m *Module) ListStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) ([]*Appointment, error) {
	if staffID <= 0 || since.IsZero() {
		return nil, invalid("staff ID and tombstone cutoff are required")
	}
	return m.engine.ListStaffCancellationTombstones(ctx, staffID, since)
}

func (m *Module) ListAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to Date) ([]*Appointment, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, invalid("a valid appointment window is required")
	}
	return m.engine.ListAppointmentsVisibleToGuardians(ctx, positiveIDs(guardianIDs), positiveIDs(studentIDs), from, to)
}

func (m *Module) ListGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) ([]*Appointment, error) {
	if since.IsZero() {
		return nil, invalid("tombstone cutoff is required")
	}
	return m.engine.ListGuardianCancellationTombstones(ctx, positiveIDs(guardianIDs), positiveIDs(studentIDs), since)
}

func (m *Module) ListGuardianReminderCandidates(ctx context.Context, from, to Date) ([]*Appointment, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, invalid("a valid reminder window is required")
	}
	return m.engine.ListGuardianReminderCandidates(ctx, from, to)
}

func (m *Module) FindAppointmentTargets(ctx context.Context, appointmentID int64) ([]*AppointmentTarget, error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return m.engine.FindAppointmentTargets(ctx, appointmentID)
}

func (m *Module) FindRecurrenceRule(ctx context.Context, appointmentID int64) (*RecurrenceRule, error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return m.engine.FindRecurrenceRule(ctx, appointmentID)
}

func (m *Module) FindRecurrenceRules(ctx context.Context, appointmentIDs []int64) ([]*RecurrenceRule, error) {
	return m.engine.FindRecurrenceRules(ctx, positiveIDs(appointmentIDs))
}

func (m *Module) FindOccurrenceOverrides(ctx context.Context, appointmentIDs []int64, dates []Date) ([]*AppointmentOccurrenceOverride, error) {
	return m.engine.FindOccurrenceOverrides(ctx, positiveIDs(appointmentIDs), dates)
}

func (m *Module) FindOccurrenceOverridesByStartDates(ctx context.Context, appointmentIDs []int64, dates []Date) ([]*AppointmentOccurrenceOverride, error) {
	return m.engine.FindOccurrenceOverridesByStartDates(ctx, positiveIDs(appointmentIDs), dates)
}

func (m *Module) FindCancelledOccurrenceOverrides(ctx context.Context, appointmentIDs []int64) ([]*AppointmentOccurrenceOverride, error) {
	return m.engine.FindCancelledOccurrenceOverrides(ctx, positiveIDs(appointmentIDs))
}

func (m *Module) FindAppointmentRecipient(ctx context.Context, recipientID int64) (*AppointmentRecipient, error) {
	if recipientID <= 0 {
		return nil, invalid("appointment recipient ID is required")
	}
	return m.engine.FindAppointmentRecipient(ctx, recipientID)
}

func (m *Module) FindAppointmentRecipients(ctx context.Context, appointmentID int64) ([]*AppointmentRecipient, error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return m.engine.FindAppointmentRecipients(ctx, appointmentID)
}

func (m *Module) FindAppointmentRecipientsByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*AppointmentRecipient, error) {
	appointmentIDs = positiveIDs(appointmentIDs)
	if len(appointmentIDs) == 0 {
		return []*AppointmentRecipient{}, nil
	}
	return m.engine.FindAppointmentRecipientsByAppointmentIDs(ctx, appointmentIDs)
}

func (m *Module) FindAppointmentRecipientStudents(ctx context.Context, recipientIDs []int64) ([]*AppointmentRecipientStudent, error) {
	recipientIDs = positiveIDs(recipientIDs)
	if len(recipientIDs) == 0 {
		return []*AppointmentRecipientStudent{}, nil
	}
	return m.engine.FindAppointmentRecipientStudents(ctx, recipientIDs)
}

func (m *Module) CountAppointmentRecipientStudents(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, invalid("student ID is required")
	}
	return m.engine.CountAppointmentRecipientStudents(ctx, studentID)
}

func (m *Module) CreateAppointment(ctx context.Context, input CreateAppointment) (*Appointment, []*AppointmentTarget, error) {
	fields, err := validateFields(input.AppointmentFields)
	if err != nil {
		return nil, nil, err
	}
	input.AppointmentFields = fields
	return m.engine.CreateAppointment(ctx, input)
}

func (m *Module) UpdateAppointment(ctx context.Context, input UpdateAppointment) (*Appointment, error) {
	if input.ID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	fields, err := validateFields(input.AppointmentFields)
	if err != nil {
		return nil, err
	}
	input.AppointmentFields = fields
	return m.engine.UpdateAppointment(ctx, input)
}

func (m *Module) DeleteAppointment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("appointment ID is required")
	}
	return m.engine.DeleteAppointment(ctx, id)
}

func (m *Module) CancelAppointment(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, invalid("appointment ID is required")
	}
	return m.engine.CancelAppointment(ctx, id)
}

func (m *Module) SoftDeleteAppointment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("appointment ID is required")
	}
	return m.engine.SoftDeleteAppointment(ctx, id)
}

func (m *Module) DeleteFeedTombstonesBefore(ctx context.Context, before time.Time) (int, error) {
	if before.IsZero() {
		return 0, invalid("tombstone cutoff is required")
	}
	return m.engine.DeleteFeedTombstonesBefore(ctx, before)
}

func (m *Module) ReplaceAppointmentTargets(ctx context.Context, appointmentID int64, targets []AppointmentTargetFields) ([]*AppointmentTarget, error) {
	if appointmentID <= 0 {
		return nil, invalid("appointment ID is required")
	}
	return m.engine.ReplaceAppointmentTargets(ctx, appointmentID, targets)
}

func (m *Module) CreateRecurrenceRule(ctx context.Context, rule *RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	return m.engine.CreateRecurrenceRule(ctx, rule)
}

func (m *Module) DeleteRecurrenceRule(ctx context.Context, appointmentID int64) error {
	if appointmentID <= 0 {
		return invalid("appointment ID is required")
	}
	return m.engine.DeleteRecurrenceRule(ctx, appointmentID)
}

func (m *Module) CreateOccurrenceOverride(ctx context.Context, override *AppointmentOccurrenceOverride) error {
	if override == nil || override.AppointmentID <= 0 || override.OccurrenceDate.IsZero() {
		return invalid("appointment ID and occurrence date are required")
	}
	return m.engine.CreateOccurrenceOverride(ctx, override)
}

func (m *Module) DeleteOccurrenceOverrides(ctx context.Context, appointmentID int64) error {
	if appointmentID <= 0 {
		return invalid("appointment ID is required")
	}
	return m.engine.DeleteOccurrenceOverrides(ctx, appointmentID)
}

func (m *Module) CancelAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate Date) (bool, error) {
	if appointmentID <= 0 || occurrenceDate.IsZero() {
		return false, invalid("appointment ID and occurrence date are required")
	}
	return m.engine.CancelAppointmentOccurrence(ctx, appointmentID, occurrenceDate)
}

func (m *Module) CreateAppointmentRecipients(ctx context.Context, appointmentID int64, fields []AppointmentRecipientFields) ([]*AppointmentRecipient, []*AppointmentRecipientStudent, error) {
	if appointmentID <= 0 {
		return nil, nil, invalid("appointment ID is required")
	}
	normalized := make([]AppointmentRecipientFields, 0, len(fields))
	for _, value := range fields {
		for _, studentID := range value.StudentIDs {
			if studentID <= 0 {
				return nil, nil, invalid("student IDs must be positive")
			}
		}
		value.StudentIDs = positiveIDs(value.StudentIDs)
		if err := validateRecipientFields(value); err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, value)
	}
	return m.engine.CreateAppointmentRecipients(ctx, appointmentID, normalized)
}

func (m *Module) UpdateAppointmentRecipientResponse(ctx context.Context, recipientID int64, status string) error {
	if recipientID <= 0 {
		return invalid("appointment recipient ID is required")
	}
	if !validResponseStatus(status) {
		return invalid("invalid recipient status")
	}
	return m.engine.UpdateAppointmentRecipientResponse(ctx, recipientID, status)
}

func (m *Module) ClaimReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate Date, guardianProfileID int64) (bool, error) {
	if appointmentID <= 0 || revision < 0 || occurrenceDate.IsZero() || guardianProfileID <= 0 {
		return false, invalid("appointment ID, revision, occurrence date, and guardian profile ID are required")
	}
	return m.engine.ClaimReminderPushDelivery(ctx, appointmentID, revision, occurrenceDate, guardianProfileID)
}

func (m *Module) ReleaseReminderPushDelivery(ctx context.Context, appointmentID int64, revision int, occurrenceDate Date, guardianProfileID int64) error {
	if appointmentID <= 0 || revision < 0 || occurrenceDate.IsZero() || guardianProfileID <= 0 {
		return invalid("appointment ID, revision, occurrence date, and guardian profile ID are required")
	}
	return m.engine.ReleaseReminderPushDelivery(ctx, appointmentID, revision, occurrenceDate, guardianProfileID)
}

func validateRecipientFields(fields AppointmentRecipientFields) error {
	switch fields.RecipientType {
	case RecipientTypeStaff:
		if fields.StaffID == nil || *fields.StaffID <= 0 || fields.GuardianProfileID != nil {
			return invalid("staff recipient requires staff_id only")
		}
		if len(fields.StudentIDs) > 0 {
			return invalid("staff recipient cannot have student IDs")
		}
	case RecipientTypeGuardianProfile:
		if fields.GuardianProfileID == nil || *fields.GuardianProfileID <= 0 || fields.StaffID != nil {
			return invalid("guardian recipient requires guardian_profile_id only")
		}
	default:
		return invalid("invalid recipient_type")
	}
	if !validResponseStatus(fields.Status) {
		return invalid("invalid recipient status")
	}
	return nil
}

func validResponseStatus(status string) bool {
	switch status {
	case ResponseStatusPending, ResponseStatusAccepted, ResponseStatusDeclined, ResponseStatusInfo:
		return true
	default:
		return false
	}
}

func validateFields(fields AppointmentFields) (AppointmentFields, error) {
	if fields.OrganizerStaffID <= 0 {
		return fields, invalid("organizer_staff_id is required")
	}
	fields.Title = strings.TrimSpace(fields.Title)
	if fields.Title == "" {
		return fields, invalid("title is required")
	}
	if fields.StartDate.IsZero() {
		return fields, invalid("start_date is required")
	}
	if fields.EndDate.IsZero() {
		return fields, invalid("end_date is required")
	}
	if fields.EndDate.Before(fields.StartDate) {
		return fields, invalid("end_date must be on or after start_date")
	}
	if !fields.AllDay && !normalizeWallClock(fields.EndTime).After(normalizeWallClock(fields.StartTime)) && fields.StartDate == fields.EndDate {
		return fields, invalid("end_time must be after start_time on same-day appointments")
	}
	switch fields.DeliveryMode {
	case DeliveryModeRSVPRequired, DeliveryModeInformational:
	default:
		return fields, invalid("delivery_mode must be rsvp_required or informational")
	}
	if fields.OverviewVisibility == "" {
		fields.OverviewVisibility = OverviewVisibilityOrganizer
	}
	switch fields.OverviewVisibility {
	case OverviewVisibilityOrganizer, OverviewVisibilityStaff, OverviewVisibilityAll:
		return fields, nil
	default:
		return fields, invalid("overview_visibility must be organizer, staff, or all")
	}
}

func fieldsFromAppointment(value *Appointment) AppointmentFields {
	return AppointmentFields{
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		NotifyGuardians: value.NotifyGuardians,
	}
}

func applyFields(value *Appointment, fields AppointmentFields) {
	value.OrganizerStaffID = fields.OrganizerStaffID
	value.Title = fields.Title
	value.Description = fields.Description
	value.Location = fields.Location
	value.StartDate = fields.StartDate
	value.EndDate = fields.EndDate
	value.StartTime = fields.StartTime
	value.EndTime = fields.EndTime
	value.AllDay = fields.AllDay
	value.DeliveryMode = fields.DeliveryMode
	value.OverviewVisibility = fields.OverviewVisibility
	value.NotifyGuardians = fields.NotifyGuardians
}

func normalizeWallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func positiveIDs(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeWeekdays(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if !validRecurrenceWeekdays[normalized] {
			return nil, invalid("weekdays must be valid day names (monday–sunday)")
		}
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result, nil
}

func normalizeMonthDays(values []int) ([]int, error) {
	seen := make(map[int]bool, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value < 1 || value > 31 {
			return nil, invalid("month_days must be between 1 and 31")
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}

func invalid(reason string) error { return &InvalidError{Reason: reason} }

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAppointmentNotFound), errors.Is(err, ErrAppointmentRecipientNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidAppointment):
		return "invalid"
	case errors.Is(err, ErrAppointmentLifecycleConflict):
		return "lifecycle_conflict"
	default:
		return "internal_error"
	}
}
