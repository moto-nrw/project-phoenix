// Package appointments is the public Appointments capability. It owns the
// appointment lifecycle and the targeting intent captured when an appointment
// is created. Other owners use Query or Command instead of reading
// calendar.appointments or calendar.appointment_targets directly.
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
)

var (
	ErrAppointmentNotFound          = errors.New("appointment not found")
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
	ListAppointmentsVisibleToStaff(context.Context, int64, Date, Date) ([]*Appointment, error)
	ListStaffCancellationTombstones(context.Context, int64, time.Time) ([]*Appointment, error)
	ListAppointmentsVisibleToGuardians(context.Context, []int64, []int64, Date, Date) ([]*Appointment, error)
	ListGuardianCancellationTombstones(context.Context, []int64, []int64, time.Time) ([]*Appointment, error)
	ListGuardianReminderCandidates(context.Context, Date, Date) ([]*Appointment, error)
	FindAppointmentTargets(context.Context, int64) ([]*AppointmentTarget, error)
}

type Command interface {
	// CreateAppointment writes the appointment and its targeting intent in one
	// UnitOfWork. A failed target insert rolls back the appointment insert.
	CreateAppointment(context.Context, CreateAppointment) (*Appointment, []*AppointmentTarget, error)
	UpdateAppointment(context.Context, UpdateAppointment) (*Appointment, error)
	DeleteAppointment(context.Context, int64) error
	BumpAppointmentRevision(context.Context, int64) error
	CancelAppointment(context.Context, int64) (bool, error)
	SoftDeleteAppointment(context.Context, int64) error
	DeleteFeedTombstonesBefore(context.Context, time.Time) (int, error)
	ReplaceAppointmentTargets(context.Context, int64, []AppointmentTargetFields) ([]*AppointmentTarget, error)
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

func (m *Module) BumpAppointmentRevision(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("appointment ID is required")
	}
	return m.engine.BumpAppointmentRevision(ctx, id)
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

func invalid(reason string) error { return &InvalidError{Reason: reason} }

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrAppointmentNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidAppointment):
		return "invalid"
	case errors.Is(err, ErrAppointmentLifecycleConflict):
		return "lifecycle_conflict"
	default:
		return "internal_error"
	}
}
