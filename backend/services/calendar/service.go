package calendar

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"golang.org/x/sync/singleflight"
)

const (
	EventSourceAppointment = calModels.EventSourceAppointment
	EventSourceTimetable   = calModels.EventSourceTimetable
	EventSourceShift       = calModels.EventSourceShift

	maxCalendarWindowDays = 92

	// parentSearchCandidateFactor over-fetches guardian text matches in
	// RecipientOptions so that reachability/visibility filtering does not
	// truncate reachable parents ranked after unreachable matches. limit is
	// capped at 50, so the candidate pool stays bounded (≤ 250 rows).
	parentSearchCandidateFactor = 5
)

var (
	ErrInvalidRequest = errors.New("invalid calendar request")
	ErrForbidden      = errors.New("calendar access forbidden")
	ErrNotFound       = errors.New("calendar item not found")
	// ErrConflict signals that a concurrent lifecycle transition (cancel/delete)
	// raced the current operation — e.g. an edit that began before the appointment
	// was cancelled. Rendered as HTTP 409.
	ErrConflict = errors.New("calendar item changed concurrently")
)

type Service interface {
	ListMyStaffEvents(ctx context.Context, from, to timezone.Date) ([]Event, error)
	ListMyParentEvents(ctx context.Context, accountID int64, from, to timezone.Date) ([]Event, error)
	CreateStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error)
	GetStaffAppointmentDetail(ctx context.Context, appointmentID int64) (*AppointmentDetail, error)
	UpdateStaffAppointment(ctx context.Context, appointmentID int64, req UpdateAppointmentRequest) (*AppointmentDetail, error)
	CancelStaffAppointment(ctx context.Context, appointmentID int64) (*AppointmentDetail, error)
	DeleteStaffAppointment(ctx context.Context, appointmentID int64) error
	CancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate timezone.Date) error
	GetStaffAppointmentOverview(ctx context.Context, appointmentID int64) (*AppointmentOverview, error)
	GetParentAppointmentOverview(ctx context.Context, accountID, appointmentID int64) (*AppointmentOverview, error)
	StaffAppointmentICS(ctx context.Context, appointmentID int64) (filename, content string, err error)
	ParentAppointmentICS(ctx context.Context, accountID, appointmentID int64) (filename, content string, err error)
	ParentCalendarFeedURL(ctx context.Context, accountID int64) (httpsURL, webcalURL string, err error)
	RotateParentCalendarFeed(ctx context.Context, accountID int64) (httpsURL, webcalURL string, err error)
	ParentCalendarFeedByToken(ctx context.Context, token string) (filename, content string, err error)
	StaffCalendarFeedURL(ctx context.Context) (httpsURL, webcalURL string, err error)
	RotateStaffCalendarFeed(ctx context.Context) (httpsURL, webcalURL string, err error)
	StaffCalendarFeedByToken(ctx context.Context, token string) (filename, content string, err error)
	RespondToStaffInvitation(ctx context.Context, recipientID int64, status string) error
	RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error
	RecipientOptions(ctx context.Context, query string, limit int) (*RecipientOptions, error)
}

type FeedCleanupService interface {
	CleanupExpiredFeedTombstones(ctx context.Context) (int, error)
}

type FullService interface {
	Service
	FeedCleanupService
	StaffCalDAVService
	GuardianNotificationAudiences(context.Context, []int64) (map[int64]GuardianNotificationAudience, error)
	ReminderEffects() ReminderEffects
}

// StaffCalDAVService keeps the CalDAV protocol adapter behind one deep
// calendar interface: callers receive either the UI credentials or one fully
// authorised, privacy-filtered calendar snapshot.
type StaffCalDAVService interface {
	StaffCalendarAccess(ctx context.Context) (StaffCalendarAccessInfo, error)
	RotateStaffCalendarAccess(ctx context.Context) (StaffCalendarAccessInfo, error)
	AuthenticateStaffCalDAV(ctx context.Context, username, appPassword string) (*StaffCalDAVCalendar, error)
}

type Config struct {
	Appointments         appointments.Capability
	StaffRepo            userModels.StaffRepository
	StudentRepo          userModels.StudentRepository
	GuardianProfileRepo  userModels.GuardianProfileRepository
	StudentGuardianRepo  userModels.StudentGuardianRepository
	ChildRepo            parentModels.ChildRepository
	GroupRepo            educationModels.GroupRepository
	InstanceStaffRepo    scheduleModels.InstanceStaffRepository
	ActivityInstanceRepo scheduleModels.ActivityInstanceRepository
	// RoomRepo resolves room names for timetable events in one batch per
	// window (#2078). Optional: nil leaves Location empty instead of failing
	// the whole calendar, mirroring StaffShiftRepo/ShiftTypeRepo below.
	RoomRepo         facilitiesModels.RoomRepository
	StaffShiftRepo   scheduleModels.StaffShiftRepository
	ShiftTypeRepo    scheduleModels.ShiftTypeRepository
	UserContext      usercontext.UserContextService
	DB               *bun.DB
	CalendarRenderer CalendarRenderer

	// Notification dependencies (all optional — nil disables e-mail; the in-app
	// calendar is unaffected).
	Outbox                 OutboxEnqueuer
	PushOutbox             PushOutboxCanceller
	SchoolRepo             platformModels.SchoolRepository
	Settings               LogoResolver
	CalDAVPolicy           CalDAVPolicy
	AccountRepo            FeedAccountRepo
	StaffFeedRepo          authModels.StaffCalendarFeedTokenRepository
	StaffFeedTombstoneRepo calModels.StaffFeedTombstoneRepository
	PersonRepo             userModels.PersonRepository
	ParentsURL             string
	FrontendURL            string
	CalDAVURL              string

	// Notifier and Preferences drive the guardian push/in-app notification that
	// accompanies the appointment e-mails (#1671). Both optional and both
	// required together: without consent there is nobody to address, so a nil
	// Preferences disables the push rather than broadcasting past consent.
	Notifier notifications.Service
	// ReminderNotifier waits for Web Push acceptance so the scheduler can retry
	// a transient failure without permanently claiming the reminder delivery.
	ReminderNotifier notifications.SynchronousService
	Preferences      notifications.PreferenceService

	// Logger is nil-safe (see service.logger()); notification dispatch is
	// fire-and-forget and reports its failures here instead of to the caller.
	Logger *slog.Logger
}

type CalendarRecurrence struct {
	Frequency string
	Interval  int
	Weekdays  []string
	MonthDays []int
	Until     string
	Count     *int
}

type CalendarEvent struct {
	UID          string
	Summary      string
	Description  string
	Location     string
	StartDate    string
	EndDate      string
	StartClock   time.Time
	EndClock     time.Time
	AllDay       bool
	Cancelled    bool
	Sequence     int
	Stamp        time.Time
	LastModified time.Time
	Recurrence   *CalendarRecurrence
	ExDates      []string
}

type CalendarRenderer interface {
	RenderCalendar(context.Context, string, []CalendarEvent) (string, error)
	RenderCalendarObject(context.Context, CalendarEvent) (string, error)
}

type CalDAVPolicy interface {
	Enabled(context.Context) (bool, error)
	EnabledForTenant(context.Context, int64) (bool, error)
}

type StaffCalendarAccessInfo struct {
	URL       string
	WebcalURL string
	CalDAV    *StaffCalDAVCredentials
}

type StaffCalDAVCredentials struct {
	ServerURL   string
	Username    string
	AppPassword string
}

type StaffCalDAVCalendar struct {
	AccountID string
	TenantID  int64
	Revision  string
	Items     []StaffCalDAVItem
}

type StaffCalDAVItem struct {
	Name       string
	UID        string
	Content    []byte
	ETag       string
	ModifiedAt time.Time
}

type service struct {
	cfg          Config
	feedCreation singleflight.Group
}

func NewService(cfg Config) FullService {
	return &service{cfg: cfg}
}

func (s *service) withinAppointmentWrite(ctx context.Context, command func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return fmt.Errorf("calendar appointment command: %w", err)
	}
	return tenant.WithinTenant(ctx, tenantID, command)
}

type Event struct {
	ID               string    `json:"id"`
	Source           string    `json:"source"`
	AppointmentID    *string   `json:"appointment_id,omitempty"`
	OccurrenceDate   *string   `json:"occurrence_date,omitempty"`
	TimetableID      *string   `json:"timetable_id,omitempty"`
	StudentID        *string   `json:"student_id,omitempty"`
	StudentName      *string   `json:"student_name,omitempty"`
	TenantID         *string   `json:"tenant_id,omitempty"`
	SchoolName       *string   `json:"school_name,omitempty"`
	Title            string    `json:"title"`
	Description      *string   `json:"description,omitempty"`
	Location         *string   `json:"location,omitempty"`
	StartDate        string    `json:"start_date"`
	EndDate          string    `json:"end_date"`
	StartTime        string    `json:"start_time"`
	EndTime          string    `json:"end_time"`
	AllDay           bool      `json:"all_day"`
	Cancelled        bool      `json:"cancelled"`
	Recurring        bool      `json:"recurring"`
	DeliveryMode     *string   `json:"delivery_mode,omitempty"`
	ResponseStatus   *string   `json:"response_status,omitempty"`
	RecipientID      *string   `json:"recipient_id,omitempty"`
	OrganizerStaffID *string   `json:"organizer_staff_id,omitempty"`
	CanRespond       bool      `json:"can_respond"`
	CanEdit          bool      `json:"can_edit"`
	CanViewOverview  bool      `json:"can_view_overview"`
	ModifiedAt       time.Time `json:"-"`
}

type AppointmentDetail struct {
	Appointment *calModels.Appointment            `json:"appointment"`
	Recurrence  *calModels.RecurrenceRule         `json:"recurrence,omitempty"`
	Recipients  []*calModels.AppointmentRecipient `json:"recipients"`
	Targets     []*calModels.AppointmentTarget    `json:"targets"`
}

type CreateAppointmentRequest struct {
	Title              string              `json:"title"`
	Description        *string             `json:"description,omitempty"`
	Location           *string             `json:"location,omitempty"`
	StartDate          timezone.Date       `json:"start_date"`
	EndDate            timezone.Date       `json:"end_date"`
	StartTime          time.Time           `json:"start_time"`
	EndTime            time.Time           `json:"end_time"`
	AllDay             bool                `json:"all_day"`
	DeliveryMode       string              `json:"delivery_mode"`
	OverviewVisibility string              `json:"overview_visibility"`
	Recurrence         *RecurrenceRequest  `json:"recurrence,omitempty"`
	Targets            []AppointmentTarget `json:"targets"`
	SendEmail          bool                `json:"send_email"`
}

// UpdateAppointmentRequest carries the editable fields of an existing
// appointment. Targeting (recipients) and delivery_mode are intentionally
// immutable after creation: re-resolving the audience on every edit would wipe
// the RSVP responses already collected. Changing the audience means cancelling
// and re-creating the appointment.
type UpdateAppointmentRequest struct {
	Title              string             `json:"title"`
	Description        *string            `json:"description,omitempty"`
	Location           *string            `json:"location,omitempty"`
	StartDate          timezone.Date      `json:"start_date"`
	EndDate            timezone.Date      `json:"end_date"`
	StartTime          time.Time          `json:"start_time"`
	EndTime            time.Time          `json:"end_time"`
	AllDay             bool               `json:"all_day"`
	OverviewVisibility string             `json:"overview_visibility"`
	Recurrence         *RecurrenceRequest `json:"recurrence,omitempty"`
	SendEmail          bool               `json:"send_email"`
	SendEmailSet       bool               `json:"-"`
}

type AppointmentOverview struct {
	AppointmentID      string                `json:"appointment_id"`
	DeliveryMode       string                `json:"delivery_mode"`
	OverviewVisibility string                `json:"overview_visibility"`
	Attendees          []AppointmentAttendee `json:"attendees"`
}

type AppointmentAttendee struct {
	RecipientID   string     `json:"recipient_id"`
	RecipientType string     `json:"recipient_type"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
}

type RecurrenceRequest struct {
	Frequency       string         `json:"frequency"`
	IntervalCount   int            `json:"interval_count"`
	Weekdays        []string       `json:"weekdays,omitempty"`
	MonthDays       []int          `json:"month_days,omitempty"`
	EndsOn          *timezone.Date `json:"ends_on,omitempty"`
	OccurrenceCount *int           `json:"occurrence_count,omitempty"`
}

type AppointmentTarget struct {
	Type  string  `json:"type"`
	ID    *int64  `json:"id,omitempty"`
	Value *string `json:"value,omitempty"`
}

type RecipientOptions struct {
	Staff    []StaffOption   `json:"staff"`
	Parents  []ParentOption  `json:"parents"`
	Groups   []GroupOption   `json:"groups"`
	Classes  []string        `json:"classes"`
	Students []StudentOption `json:"students"`
}

type StaffOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ParentOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GroupOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StudentOption struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	SchoolClass string  `json:"school_class,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
}

func (s *service) ListMyStaffEvents(ctx context.Context, from, to timezone.Date) ([]Event, error) {
	if err := validateWindow(from, to); err != nil {
		return nil, err
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}

	appointments, err := s.listAppointmentsVisibleToStaff(ctx, staff.ID, toCalendarDate(from), toCalendarDate(to))
	if err != nil {
		return nil, err
	}
	appointmentEvents, err := s.expandAppointmentEvents(ctx, appointments, staff.ID, from, to)
	if err != nil {
		return nil, err
	}
	timetableEvents, err := s.staffTimetableEvents(ctx, staff.ID, from, to)
	if err != nil {
		return nil, err
	}
	shiftEvents, err := s.staffShiftEvents(ctx, staff.ID, from, to)
	if err != nil {
		return nil, err
	}

	events := append(appointmentEvents, timetableEvents...)
	events = append(events, shiftEvents...)
	sortEvents(events)
	return events, nil
}

func (s *service) ListMyParentEvents(ctx context.Context, accountID int64, from, to timezone.Date) ([]Event, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if err := validateWindow(from, to); err != nil {
		return nil, err
	}

	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return nil, err
	}
	childrenByTenant := groupChildrenByTenant(children)
	events := []Event{}
	for tenantID, tenantChildren := range childrenByTenant {
		tenantID := tenantID
		tenantChildren := tenantChildren
		if err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
			studentIDs := distinctChildStudentIDs(tenantChildren)
			appointments, err := s.listAppointmentsVisibleToGuardians(txCtx, guardianProfileIDs, studentIDs, toCalendarDate(from), toCalendarDate(to))
			if err != nil {
				return err
			}
			appointmentEvents, err := s.expandGuardianAppointmentEvents(txCtx, appointments, guardianProfileIDs, studentIDs, from, to)
			if err != nil {
				return err
			}
			schoolName := ""
			tenantIDString := formatID(tenantID)
			if len(tenantChildren) > 0 {
				schoolName = tenantChildren[0].SchoolName
			}
			for i := range appointmentEvents {
				appointmentEvents[i].TenantID = &tenantIDString
				if schoolName != "" {
					appointmentEvents[i].SchoolName = &schoolName
				}
			}
			events = append(events, appointmentEvents...)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sortEvents(events)
	return events, nil
}

func (s *service) CreateStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error) {
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		var commandErr error
		result, commandErr = s.createStaffAppointment(txCtx, req)
		return commandErr
	})
	return result, err
}

func (s *service) createStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error) {
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	if req.DeliveryMode == "" {
		req.DeliveryMode = calModels.DeliveryModeRSVPRequired
	}
	if req.OverviewVisibility == "" {
		req.OverviewVisibility = calModels.OverviewVisibilityOrganizer
	}
	if req.EndDate.IsZero() {
		req.EndDate = req.StartDate
	}

	appointment := &calModels.Appointment{
		OrganizerStaffID:   staff.ID,
		Title:              req.Title,
		Description:        req.Description,
		Location:           req.Location,
		StartDate:          toCalendarDate(req.StartDate),
		EndDate:            toCalendarDate(req.EndDate),
		StartTime:          timezone.NormalizeWallClock(req.StartTime),
		EndTime:            timezone.NormalizeWallClock(req.EndTime),
		AllDay:             req.AllDay,
		DeliveryMode:       req.DeliveryMode,
		OverviewVisibility: req.OverviewVisibility,
		// Persist the notification opt-in so a later cancellation honours it: an
		// appointment created without send_email never mails guardians.
		NotifyGuardians: req.SendEmail,
	}
	if err := appointment.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	recurrence := recurrenceRuleFromRequest(req.Recurrence)
	if recurrence != nil {
		recurrence.AppointmentID = 1
		if err := recurrence.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
		if recurrence.EndsOn != nil && recurrence.EndsOn.Before(appointment.StartDate) {
			return nil, fmt.Errorf("%w: recurrence end must be on or after start date", ErrInvalidRequest)
		}
		if _, ok := firstRecurrenceOccurrence(appointment, recurrence); !ok {
			return nil, fmt.Errorf("%w: recurrence produces no occurrences", ErrInvalidRequest)
		}
		recurrence.AppointmentID = 0
	}

	recipientFields, targets, err := s.resolveTargets(ctx, appointment.DeliveryMode, req.Targets)
	if err != nil {
		return nil, err
	}

	created, persistedTargets, err := s.cfg.Appointments.CreateAppointment(ctx, appointments.CreateAppointment{
		AppointmentFields: appointmentCapabilityFields(appointment),
		Targets:           appointmentTargetFields(targets),
	})
	if err != nil {
		return nil, err
	}
	appointment = calendarAppointment(created)
	targets = calendarTargets(persistedTargets)

	if recurrence != nil {
		recurrence.AppointmentID = appointment.ID
		if err := s.cfg.Appointments.CreateRecurrenceRule(ctx, recurrence); err != nil {
			return nil, err
		}
	}

	recipients, _, err := s.cfg.Appointments.CreateAppointmentRecipients(ctx, appointment.ID, recipientFields)
	if err != nil {
		return nil, err
	}

	if req.SendEmail {
		if err := s.notifyGuardians(ctx, appointment, platformModels.EmailKindAppointmentPublished); err != nil {
			return nil, err
		}
		s.notifyGuardianDevices(ctx, appointment, platformModels.EmailKindAppointmentPublished)
	}

	return &AppointmentDetail{
		Appointment: appointment,
		Recurrence:  recurrence,
		Recipients:  recipients,
		Targets:     targets,
	}, nil
}

// loadOrganizedAppointment fetches an appointment and asserts the current staff
// member is its organizer. Only the organizer may edit, cancel, or delete an
// appointment. Returns ErrNotFound for missing rows and ErrForbidden when the
// caller is not the organizer.
func (s *service) loadOrganizedAppointment(ctx context.Context, appointmentID int64) (*calModels.Appointment, error) {
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointment, err := s.findAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appointment == nil {
		return nil, ErrNotFound
	}
	// A soft-deleted appointment lives on only as a feed tombstone; it is gone from
	// every interactive surface, so lifecycle operations (edit/cancel/delete/detail)
	// treat it as not found.
	if appointment.DeletedAt != nil {
		return nil, ErrNotFound
	}
	if appointment.OrganizerStaffID != staff.ID {
		return nil, fmt.Errorf("%w: only the organizer may modify this appointment", ErrForbidden)
	}
	return appointment, nil
}

func (s *service) findAppointment(ctx context.Context, appointmentID int64) (*calModels.Appointment, error) {
	appointment, err := s.cfg.Appointments.FindAppointment(ctx, appointmentID)
	if errors.Is(err, appointments.ErrAppointmentNotFound) {
		return nil, nil
	}
	return calendarAppointment(appointment), err
}

func (s *service) findAppointmentForUpdate(ctx context.Context, appointmentID int64) (*calModels.Appointment, error) {
	appointment, err := s.cfg.Appointments.FindAppointmentForUpdate(ctx, appointmentID)
	if errors.Is(err, appointments.ErrAppointmentNotFound) {
		return nil, fmt.Errorf("appointment %d not found: %w", appointmentID, sql.ErrNoRows)
	}
	return calendarAppointment(appointment), err
}

func appointmentCapabilityFields(value *calModels.Appointment) appointments.AppointmentFields {
	return appointments.AppointmentFields{
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		NotifyGuardians: value.NotifyGuardians,
	}
}

func appointmentTargetFields(values []*calModels.AppointmentTarget) []appointments.AppointmentTargetFields {
	result := make([]appointments.AppointmentTargetFields, 0, len(values))
	for _, value := range values {
		result = append(result, appointments.AppointmentTargetFields{
			TargetType: value.TargetType, TargetID: value.TargetID, TargetValue: value.TargetValue,
		})
	}
	return result
}

func calendarAppointment(value *appointments.Appointment) *calModels.Appointment {
	if value == nil {
		return nil
	}
	return &calModels.Appointment{
		Model:            calModels.Model{ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt},
		TenantModel:      calModels.TenantModel{TenantID: value.TenantID},
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		CancelledAt: value.CancelledAt, DeletedAt: value.DeletedAt,
		NotifyGuardians: value.NotifyGuardians, Revision: value.Revision,
	}
}

func calendarAppointments(values []*appointments.Appointment) []*calModels.Appointment {
	result := make([]*calModels.Appointment, 0, len(values))
	for _, value := range values {
		result = append(result, calendarAppointment(value))
	}
	return result
}

func calendarTargets(values []*appointments.AppointmentTarget) []*calModels.AppointmentTarget {
	result := make([]*calModels.AppointmentTarget, 0, len(values))
	for _, value := range values {
		result = append(result, &calModels.AppointmentTarget{
			Model:         calModels.Model{ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt},
			TenantModel:   calModels.TenantModel{TenantID: value.TenantID},
			AppointmentID: value.AppointmentID, TargetType: value.TargetType,
			TargetID: value.TargetID, TargetValue: value.TargetValue,
		})
	}
	return result
}

func (s *service) listAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to calModels.Date) ([]*calModels.Appointment, error) {
	values, err := s.cfg.Appointments.ListAppointmentsVisibleToStaff(ctx, staffID, from, to)
	return calendarAppointments(values), err
}

func (s *service) listStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) ([]*calModels.Appointment, error) {
	values, err := s.cfg.Appointments.ListStaffCancellationTombstones(ctx, staffID, since)
	return calendarAppointments(values), err
}

func (s *service) listAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to calModels.Date) ([]*calModels.Appointment, error) {
	values, err := s.cfg.Appointments.ListAppointmentsVisibleToGuardians(ctx, guardianIDs, studentIDs, from, to)
	return calendarAppointments(values), err
}

func (s *service) listGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) ([]*calModels.Appointment, error) {
	values, err := s.cfg.Appointments.ListGuardianCancellationTombstones(ctx, guardianIDs, studentIDs, since)
	return calendarAppointments(values), err
}

// appointmentDetail reloads the full detail (recurrence, recipients, targets)
// for an appointment. Used by the lifecycle operations so callers (and the
// notification layer in Phase B) get the same shape as CreateStaffAppointment.
func (s *service) appointmentDetail(ctx context.Context, appointment *calModels.Appointment) (*AppointmentDetail, error) {
	recurrence, err := s.cfg.Appointments.FindRecurrenceRule(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	recipients, err := s.cfg.Appointments.FindAppointmentRecipients(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	targets, err := s.cfg.Appointments.FindAppointmentTargets(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	return &AppointmentDetail{
		Appointment: appointment,
		Recurrence:  recurrence,
		Recipients:  recipients,
		Targets:     calendarTargets(targets),
	}, nil
}

func (s *service) GetStaffAppointmentDetail(ctx context.Context, appointmentID int64) (*AppointmentDetail, error) {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	return s.appointmentDetail(ctx, appointment)
}

func (s *service) UpdateStaffAppointment(ctx context.Context, appointmentID int64, req UpdateAppointmentRequest) (*AppointmentDetail, error) {
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		var commandErr error
		result, commandErr = s.updateStaffAppointment(txCtx, appointmentID, req)
		return commandErr
	})
	return result, err
}

func (s *service) updateStaffAppointment(ctx context.Context, appointmentID int64, req UpdateAppointmentRequest) (*AppointmentDetail, error) {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	appointment, err = s.findAppointmentForUpdate(ctx, appointment.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// A cancelled appointment is terminal: there is no reactivation flow, so
	// editing it (which would also fire a "Termin geändert" notice while the
	// appointment stays cancelled) is rejected outright.
	if appointment.CancelledAt != nil {
		return nil, fmt.Errorf("%w: appointment is cancelled", ErrInvalidRequest)
	}
	if req.OverviewVisibility == "" {
		req.OverviewVisibility = appointment.OverviewVisibility
	}
	if req.EndDate.IsZero() {
		req.EndDate = req.StartDate
	}

	appointment.Title = req.Title
	appointment.Description = req.Description
	appointment.Location = req.Location
	appointment.StartDate = toCalendarDate(req.StartDate)
	appointment.EndDate = toCalendarDate(req.EndDate)
	appointment.StartTime = timezone.NormalizeWallClock(req.StartTime)
	appointment.EndTime = timezone.NormalizeWallClock(req.EndTime)
	appointment.AllDay = req.AllDay
	appointment.OverviewVisibility = req.OverviewVisibility
	if req.SendEmailSet || req.SendEmail {
		appointment.NotifyGuardians = req.SendEmail
	}
	if err := appointment.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	recurrence := recurrenceRuleFromRequest(req.Recurrence)
	if recurrence != nil {
		recurrence.AppointmentID = appointment.ID
		if err := recurrence.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
		}
		if recurrence.EndsOn != nil && recurrence.EndsOn.Before(appointment.StartDate) {
			return nil, fmt.Errorf("%w: recurrence end must be on or after start date", ErrInvalidRequest)
		}
		if _, ok := firstRecurrenceOccurrence(appointment, recurrence); !ok {
			return nil, fmt.Errorf("%w: recurrence produces no occurrences", ErrInvalidRequest)
		}
	}

	updated, err := s.cfg.Appointments.UpdateAppointment(ctx, appointments.UpdateAppointment{
		ID: appointment.ID, AppointmentFields: appointmentCapabilityFields(appointment),
	})
	if err != nil {
		// A concurrent cancel/delete transitioned the appointment between load and
		// write, so the conditional update matched nothing. Abort before touching
		// recurrence/overrides or sending an "updated" notice; the tenant tx rolls
		// back. Surface a conflict rather than a bogus success.
		if errors.Is(err, calModels.ErrAppointmentLifecycleConflict) {
			return nil, fmt.Errorf("%w: appointment was cancelled or deleted", ErrConflict)
		}
		return nil, err
	}
	appointment = calendarAppointment(updated)
	// Replace the recurrence rule wholesale: the DB enforces one rule per
	// appointment, so drop the old row and recreate if the edit still recurs.
	if err := s.cfg.Appointments.DeleteRecurrenceRule(ctx, appointment.ID); err != nil {
		return nil, err
	}
	if recurrence != nil {
		if err := s.cfg.Appointments.CreateRecurrenceRule(ctx, recurrence); err != nil {
			return nil, err
		}
	}
	// Editing the series is a whole-series operation, so per-occurrence
	// cancellations ("Nur diesen Termin") from the old cadence no longer apply.
	// Drop them; otherwise a date reused by the new recurrence would be silently
	// suppressed (and stale EXDATEs would leak into the subscription feed/ICS).
	if err := s.cfg.Appointments.DeleteOccurrenceOverrides(ctx, appointment.ID); err != nil {
		return nil, err
	}

	// Kill every not-yet-sent appointment e-mail so the worker cannot deliver a
	// stale title/date/location or a reminder after guardian delivery was turned
	// off. The update notice below replaces the lifecycle communication.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment updated"); err != nil {
		return nil, err
	}
	if appointment.NotifyGuardians {
		if err := s.notifyGuardians(ctx, appointment, platformModels.EmailKindAppointmentUpdated); err != nil {
			return nil, err
		}
		s.notifyGuardianDevices(ctx, appointment, platformModels.EmailKindAppointmentUpdated)
	}

	return s.appointmentDetail(ctx, appointment)
}

func (s *service) CancelStaffAppointment(ctx context.Context, appointmentID int64) (*AppointmentDetail, error) {
	var result *AppointmentDetail
	err := s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		var commandErr error
		result, commandErr = s.cancelStaffAppointment(txCtx, appointmentID)
		return commandErr
	})
	return result, err
}

func (s *service) cancelStaffAppointment(ctx context.Context, appointmentID int64) (*AppointmentDetail, error) {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	appointment, err = s.findAppointmentForUpdate(ctx, appointment.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if appointment.CancelledAt == nil {
		// Cancel via the dedicated conditional update (not Update): it only writes
		// cancelled_at/revision, so it can't clobber a concurrent edit and — being
		// WHERE cancelled_at IS NULL AND deleted_at IS NULL — matches nothing once a
		// concurrent cancel or delete has won. `transitioned` is true only for the
		// caller that actually flipped the row.
		transitioned, err := s.cfg.Appointments.CancelAppointment(ctx, appointment.ID)
		if err != nil {
			return nil, err
		}
		if !transitioned {
			// A concurrent cancel or delete won between our load and this update, so
			// the row did NOT transition. Do NOT mark the stale in-memory object
			// cancelled (that would return a "cancelled" view of a possibly-deleted
			// appointment). Reload for the true state: loadOrganizedAppointment
			// surfaces a concurrent delete as not-found and returns the (already)
			// cancelled detail for a concurrent cancel — idempotent, no double notice.
			reloaded, err := s.loadOrganizedAppointment(ctx, appointmentID)
			if err != nil {
				return nil, err
			}
			return s.appointmentDetail(ctx, reloaded)
		}
		now := time.Now()
		appointment.CancelledAt = &now
		// Only the caller that performed the transition does the notification work,
		// so two concurrent cancels can't send duplicate guardian e-mails. And even
		// then, honour the persisted opt-in: an appointment created without
		// send_email never mails guardians on cancellation.
		if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment cancelled"); err != nil {
			return nil, err
		}
		if appointment.NotifyGuardians {
			if err := s.notifyGuardians(ctx, appointment, platformModels.EmailKindAppointmentCancelled); err != nil {
				return nil, err
			}
			s.notifyGuardianDevices(ctx, appointment, platformModels.EmailKindAppointmentCancelled)
		}
	}
	return s.appointmentDetail(ctx, appointment)
}

func (s *service) DeleteStaffAppointment(ctx context.Context, appointmentID int64) error {
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		return s.deleteStaffAppointment(txCtx, appointmentID)
	})
}

func (s *service) deleteStaffAppointment(ctx context.Context, appointmentID int64) error {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return err
	}
	appointment, err = s.findAppointmentForUpdate(ctx, appointment.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	// Deleting is silent (no notice); kill any queued e-mails first so the worker
	// doesn't send a mail for a row we're about to remove or tombstone.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment deleted"); err != nil {
		return err
	}
	// A subscribed external calendar only drops an
	// appointment when it re-receives the SAME UID with STATUS:CANCELLED at a
	// higher SEQUENCE. Hard-deleting an appointment makes it vanish
	// from the feed, which many clients keep rather than purge — the stale event
	// then lingers in the subscribed calendar forever. Every appointment can appear
	// in its organizer's staff feed, so it is SOFT-deleted: it disappears from every
	// interactive staff/parent calendar (those queries filter deleted_at IS NULL),
	// yet the subscription feed re-exports it as a durable STATUS:CANCELLED
	// tombstone (retained by deletion time, independent of the date lookback) with
	// a bumped SEQUENCE so even long-offline subscribers eventually purge it.
	return s.cfg.Appointments.SoftDeleteAppointment(ctx, appointment.ID)
}

func (s *service) CancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate timezone.Date) error {
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		return s.cancelStaffAppointmentOccurrence(txCtx, appointmentID, occurrenceDate)
	})
}

func (s *service) cancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate timezone.Date) error {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return err
	}
	appointment, err = s.findAppointmentForUpdate(ctx, appointment.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if occurrenceDate.IsZero() {
		return fmt.Errorf("%w: occurrence date is required", ErrInvalidRequest)
	}
	// A single-occurrence cancellation only makes sense for a date the series
	// actually generates. Without this guard, cancelling a non-recurring
	// appointment (or a date not in the series) would persist a useless override
	// while the appointment stayed fully visible.
	recurrence, err := s.cfg.Appointments.FindRecurrenceRule(ctx, appointment.ID)
	if err != nil {
		return err
	}
	if recurrence == nil {
		return fmt.Errorf("%w: appointment is not recurring", ErrInvalidRequest)
	}
	if !occurrenceExists(appointment, recurrence, occurrenceDate) {
		return fmt.Errorf("%w: occurrence date is not part of the series", ErrInvalidRequest)
	}
	// Reuse an existing override for this date (e.g. from a prior single-occurrence
	// edit) so cancelling stays idempotent and respects the (appointment, date)
	// uniqueness constraint.
	existing, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, []int64{appointment.ID}, []calModels.Date{toCalendarDate(occurrenceDate)})
	if err != nil {
		return err
	}
	if len(existing) > 0 && existing[0].Cancelled {
		// Already cancelled — idempotent no-op, no revision bump.
		return nil
	}
	// Conflict-safe upsert: a concurrent request cancelling the same occurrence
	// converges on cancelled=true instead of one hitting the unique constraint
	// and returning a 500.
	if _, err := s.cfg.Appointments.CancelAppointmentOccurrence(ctx, appointment.ID, toCalendarDate(occurrenceDate)); err != nil {
		return err
	}
	// A queued create/update notice announces the appointment's first occurrence;
	// removing an occurrence (possibly that first one) could otherwise deliver a
	// mail for a date parents will no longer see. Kill any pending notice, matching
	// the cancel/delete/update stale-notification cleanup.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "occurrence cancelled"); err != nil {
		return err
	}
	return nil
}

func (s *service) GetStaffAppointmentOverview(ctx context.Context, appointmentID int64) (*AppointmentOverview, error) {
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointment, err := s.findAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	// A soft-deleted appointment is gone from every interactive surface, so its
	// overview is not reachable even with a retained appointment ID.
	if appointment == nil || appointment.DeletedAt != nil {
		return nil, ErrNotFound
	}
	recipients, err := s.cfg.Appointments.FindAppointmentRecipients(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	_, recipientID := staffRecipientStatus(recipients, staff.ID)
	if appointment.OrganizerStaffID != staff.ID && recipientID == nil {
		return nil, ErrNotFound
	}
	if !canStaffViewOverview(appointment, staff.ID, recipientID != nil) {
		return nil, ErrForbidden
	}
	return s.buildAppointmentOverview(ctx, appointment, recipients)
}

func (s *service) GetParentAppointmentOverview(ctx context.Context, accountID, appointmentID int64) (*AppointmentOverview, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return nil, err
	}
	childrenByTenant := groupChildrenByTenant(children)
	var foundForbidden bool
	for tenantID, tenantChildren := range childrenByTenant {
		tenantID := tenantID
		tenantChildren := tenantChildren
		var overview *AppointmentOverview
		err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			appointment, err := s.findAppointment(txCtx, appointmentID)
			if err != nil {
				return err
			}
			// A soft-deleted appointment is not viewable by parents either.
			if appointment == nil || appointment.DeletedAt != nil {
				return nil
			}
			recipients, err := s.cfg.Appointments.FindAppointmentRecipients(txCtx, appointment.ID)
			if err != nil {
				return err
			}
			guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
			studentIDs := distinctChildStudentIDs(tenantChildren)
			_, recipientID, err := s.guardianRecipientStatusForStudents(txCtx, recipients, guardianProfileIDs, studentIDs)
			if err != nil {
				return err
			}
			if recipientID == nil {
				return nil
			}
			if !canParentViewOverview(appointment, true) {
				foundForbidden = true
				return nil
			}
			overview, err = s.buildAppointmentOverview(txCtx, appointment, recipients)
			return err
		})
		if err != nil {
			return nil, err
		}
		if overview != nil {
			return overview, nil
		}
	}
	if foundForbidden {
		return nil, ErrForbidden
	}
	return nil, ErrNotFound
}

func (s *service) RespondToStaffInvitation(ctx context.Context, recipientID int64, status string) error {
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	if status != calModels.ResponseStatusAccepted && status != calModels.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
	}

	recipient, err := s.cfg.Appointments.FindAppointmentRecipient(ctx, recipientID)
	if err != nil {
		return err
	}
	if recipient == nil || recipient.StaffID == nil || *recipient.StaffID != staff.ID {
		return ErrNotFound
	}
	if recipient.Status == calModels.ResponseStatusInfo {
		return fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
	}
	appointment, err := s.findAppointment(ctx, recipient.AppointmentID)
	if err != nil {
		return err
	}
	// A soft-deleted appointment can no longer be answered — it is gone from every
	// interactive surface, so a retained recipient ID must not reach the RSVP.
	if appointment == nil || appointment.DeletedAt != nil {
		return ErrNotFound
	}
	if appointment.CancelledAt != nil {
		return fmt.Errorf("%w: appointment is cancelled", ErrInvalidRequest)
	}
	return s.cfg.Appointments.UpdateAppointmentRecipientResponse(ctx, recipientID, status)
}

func (s *service) RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error {
	if accountID <= 0 {
		return fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if status != calModels.ResponseStatusAccepted && status != calModels.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
	}

	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return err
	}
	// recipientID is globally unique (calendar.appointment_recipients.id is a
	// BIGSERIAL PRIMARY KEY, not per-tenant), so it exists in exactly one
	// tenant. Scanning the parent's tenants is safe: RLS inside each
	// WithTenantTx returns the row only in its owning tenant, and allowedProfiles
	// then confirms it belongs to THIS parent there. There is no cross-tenant ID
	// collision — a foreign recipient id simply resolves to nil or fails the
	// allowedProfiles check.
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		allowedProfiles := int64Set(distinctGuardianProfileIDs(tenantChildren))
		allowedStudentIDs := int64Set(distinctChildStudentIDs(tenantChildren))
		var updated bool
		if err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			recipient, err := s.cfg.Appointments.FindAppointmentRecipient(txCtx, recipientID)
			if err != nil {
				return err
			}
			if recipient == nil || recipient.GuardianProfileID == nil {
				return nil
			}
			if _, ok := allowedProfiles[*recipient.GuardianProfileID]; !ok {
				return nil
			}
			visible, err := s.recipientHasVisibleStudent(txCtx, recipient.ID, allowedStudentIDs)
			if err != nil {
				return err
			}
			if !visible {
				return nil
			}
			if recipient.Status == calModels.ResponseStatusInfo {
				return fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
			}
			appointment, err := s.findAppointment(txCtx, recipient.AppointmentID)
			if err != nil {
				return err
			}
			// A soft-deleted appointment is unanswerable — treat as not found.
			if appointment == nil || appointment.DeletedAt != nil {
				return nil
			}
			if appointment.CancelledAt != nil {
				return fmt.Errorf("%w: appointment is cancelled", ErrInvalidRequest)
			}
			if err := s.cfg.Appointments.UpdateAppointmentRecipientResponse(txCtx, recipientID, status); err != nil {
				return err
			}
			updated = true
			return nil
		}); err != nil {
			return err
		}
		if updated {
			return nil
		}
	}
	return ErrNotFound
}

func (s *service) RecipientOptions(ctx context.Context, query string, limit int) (*RecipientOptions, error) {
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	query = strings.ToLower(strings.TrimSpace(query))

	staffRows, err := s.cfg.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	// Only offer staff that target resolution would actually accept
	// (FindReachableCalendarStaffIDs), so the picker never presents a choice
	// that CreateStaffAppointment later rejects with "staff target is not
	// available".
	reachableStaff, err := s.cfg.StaffRepo.FindReachableCalendarStaffIDs(ctx, nil)
	if err != nil {
		return nil, err
	}
	staffOptions := make([]StaffOption, 0, min(limit, len(staffRows)))
	for _, row := range staffRows {
		if !reachableStaff[row.ID] {
			continue
		}
		name := staffName(row)
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			staffOptions = append(staffOptions, StaffOption{ID: formatID(row.ID), Name: name})
			if len(staffOptions) >= limit {
				break
			}
		}
	}

	// Over-fetch text matches before filtering: reachability (active portal
	// account + guardian role) and student visibility are applied below, so
	// capping SearchByText at `limit` would let unreachable matches consume the
	// budget and hide reachable parents ranked later. Fetch a larger candidate
	// pool, then cap the filtered result at `limit`.
	parents, err := s.cfg.GuardianProfileRepo.SearchByText(ctx, query, limit*parentSearchCandidateFactor)
	if err != nil {
		return nil, err
	}
	parentIDs := make([]int64, 0, len(parents))
	for _, parent := range parents {
		parentIDs = append(parentIDs, parent.ID)
	}
	activeParents, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, parentIDs)
	if err != nil {
		return nil, err
	}
	parentOptions := make([]ParentOption, 0, min(limit, len(parents)))
	for _, parent := range parents {
		if len(parentOptions) >= limit {
			break
		}
		if _, ok := activeParents[parent.ID]; !ok {
			continue
		}
		visible, err := s.guardianHasPortalVisibleStudent(ctx, parent.ID)
		if err != nil {
			return nil, err
		}
		if visible {
			parentOptions = append(parentOptions, ParentOption{ID: formatID(parent.ID), Name: guardianName(parent)})
		}
	}

	groups, err := s.cfg.GroupRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	groupOptions := make([]GroupOption, 0, min(limit, len(groups)))
	for _, group := range groups {
		if query == "" || strings.Contains(strings.ToLower(group.Name), query) {
			groupOptions = append(groupOptions, GroupOption{ID: formatID(group.ID), Name: group.Name})
			if len(groupOptions) >= limit {
				break
			}
		}
	}

	classes, err := s.cfg.StudentRepo.ListSchoolClasses(ctx)
	if err != nil {
		return nil, err
	}
	classOptions := make([]string, 0, min(limit, len(classes)))
	for _, className := range classes {
		if query == "" || strings.Contains(strings.ToLower(className), query) {
			classOptions = append(classOptions, className)
			if len(classOptions) >= limit {
				break
			}
		}
	}

	students, err := s.cfg.StudentRepo.FindAllWithGroups(ctx)
	if err != nil {
		return nil, err
	}
	studentOptions := make([]StudentOption, 0, min(limit, len(students)))
	for _, row := range students {
		name := studentDisplayName(row.Student)
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			var groupID *string
			if row.GroupID != nil {
				value := formatID(*row.GroupID)
				groupID = &value
			}
			studentOptions = append(studentOptions, StudentOption{
				ID:          formatID(row.ID),
				Name:        name,
				SchoolClass: row.SchoolClass,
				GroupID:     groupID,
			})
			if len(studentOptions) >= limit {
				break
			}
		}
	}

	return &RecipientOptions{
		Staff:    staffOptions,
		Parents:  parentOptions,
		Groups:   groupOptions,
		Classes:  classOptions,
		Students: studentOptions,
	}, nil
}

func validateWindow(from, to timezone.Date) error {
	if from.IsZero() || to.IsZero() {
		return fmt.Errorf("%w: from and to are required", ErrInvalidRequest)
	}
	if to.Before(from) {
		return fmt.Errorf("%w: to must be on or after from", ErrInvalidRequest)
	}
	if from.DaysUntil(to)+1 > maxCalendarWindowDays {
		return fmt.Errorf("%w: range cannot exceed %d days", ErrInvalidRequest, maxCalendarWindowDays)
	}
	return nil
}

// recipientsByAppointment loads the recipients of every listed appointment in
// one read; the list views used to issue one read per appointment (#2940).
func (s *service) recipientsByAppointment(ctx context.Context, appointmentIDs []int64) (map[int64][]*calModels.AppointmentRecipient, error) {
	recipients, err := s.cfg.Appointments.FindAppointmentRecipientsByAppointmentIDs(ctx, appointmentIDs)
	if err != nil {
		return nil, err
	}
	byAppointment := make(map[int64][]*calModels.AppointmentRecipient, len(appointmentIDs))
	for _, recipient := range recipients {
		byAppointment[recipient.AppointmentID] = append(byAppointment[recipient.AppointmentID], recipient)
	}
	return byAppointment, nil
}

func (s *service) expandAppointmentEvents(ctx context.Context, appointments []*calModels.Appointment, staffID int64, from, to timezone.Date) ([]Event, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.Appointments.FindRecurrenceRules(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, from, to)
	overrides, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, ids, toCalendarDates(occurrenceDates))
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[fmt.Sprintf("%d:%s", override.AppointmentID, override.OccurrenceDate.String())] = override
	}

	recipientsByAppointment, err := s.recipientsByAppointment(ctx, ids)
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		status, recipientID := staffRecipientStatus(recipientsByAppointment[appointment.ID], staffID)
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			if !dateRangesOverlap(toTimezoneDate(appointment.StartDate), toTimezoneDate(appointment.EndDate), from, to) {
				continue
			}
			event := appointmentEvent(appointment, toTimezoneDate(appointment.StartDate), status, recipientID, staffID)
			events = append(events, event)
			continue
		}
		for _, occurrence := range expandOccurrences(appointment, recurrence, from, to) {
			override := overrideByAppointmentDate[fmt.Sprintf("%d:%s", appointment.ID, occurrence.String())]
			if override != nil && override.Cancelled {
				continue
			}
			event := appointmentEvent(appointment, occurrence, status, recipientID, staffID)
			event.Recurring = true
			applyOverride(&event, override)
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *service) expandGuardianAppointmentEvents(ctx context.Context, appointments []*calModels.Appointment, guardianProfileIDs []int64, studentIDs []int64, from, to timezone.Date) ([]Event, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.Appointments.FindRecurrenceRules(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, from, to)
	overrides, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, ids, toCalendarDates(occurrenceDates))
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[fmt.Sprintf("%d:%s", override.AppointmentID, override.OccurrenceDate.String())] = override
	}

	recipientsByAppointment, err := s.recipientsByAppointment(ctx, ids)
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		status, recipientID, err := s.guardianRecipientStatusForStudents(ctx, recipientsByAppointment[appointment.ID], guardianProfileIDs, studentIDs)
		if err != nil {
			return nil, err
		}
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			if !dateRangesOverlap(toTimezoneDate(appointment.StartDate), toTimezoneDate(appointment.EndDate), from, to) {
				continue
			}
			event := appointmentEvent(appointment, toTimezoneDate(appointment.StartDate), status, recipientID, 0)
			event.CanViewOverview = canParentViewOverview(appointment, recipientID != nil)
			events = append(events, event)
			continue
		}
		for _, occurrence := range expandOccurrences(appointment, recurrence, from, to) {
			override := overrideByAppointmentDate[fmt.Sprintf("%d:%s", appointment.ID, occurrence.String())]
			if override != nil && override.Cancelled {
				continue
			}
			event := appointmentEvent(appointment, occurrence, status, recipientID, 0)
			event.CanViewOverview = canParentViewOverview(appointment, recipientID != nil)
			event.Recurring = true
			applyOverride(&event, override)
			events = append(events, event)
		}
	}
	return events, nil
}

// staffTimetableAssignment pairs a timetable instance with the staff row that
// put this staff member on it — the row carries the per-person room override.
type staffTimetableAssignment struct {
	instance   *scheduleModels.ActivityInstance
	assignment *scheduleModels.InstanceStaff
}

// timetableRoomID resolves the room the staff member is actually in: the
// per-person override on instance_staff wins over the instance's own room
// (the Lernzeit split pattern), matching the Dienstplan overview in
// services/schedule/staff_schedule_overview.go.
func timetableRoomID(instance *scheduleModels.ActivityInstance, assignment *scheduleModels.InstanceStaff) int64 {
	if assignment != nil && assignment.RoomID != nil {
		return *assignment.RoomID
	}
	if instance == nil {
		return 0
	}
	return instance.RoomID
}

func distinctTimetableRoomIDs(assigned []staffTimetableAssignment) []int64 {
	seen := make(map[int64]struct{}, len(assigned))
	roomIDs := make([]int64, 0, len(assigned))
	for _, entry := range assigned {
		roomID := timetableRoomID(entry.instance, entry.assignment)
		if roomID <= 0 {
			continue
		}
		if _, ok := seen[roomID]; ok {
			continue
		}
		seen[roomID] = struct{}{}
		roomIDs = append(roomIDs, roomID)
	}
	return roomIDs
}

// timetableRoomNames resolves every room in the window with one query so the
// room name does not cost one read per event (#2078). An unwired room
// repository yields an empty map: events keep an empty Location instead of
// failing the calendar, mirroring the ShiftTypeRepo guard in staffShiftEvents.
func (s *service) timetableRoomNames(ctx context.Context, roomIDs []int64) (map[int64]string, error) {
	names := make(map[int64]string, len(roomIDs))
	if s.cfg.RoomRepo == nil || len(roomIDs) == 0 {
		return names, nil
	}
	rooms, err := s.cfg.RoomRepo.FindByIDs(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		if room != nil {
			names[room.ID] = room.Name
		}
	}
	return names, nil
}

// collectStaffTimetableAssignments resolves the staff member's assignments and
// their instances in two range queries. It keeps the first assignment per
// instance. Interactive reads drop cancelled instances; subscription feeds
// retain them so external calendars receive STATUS:CANCELLED.
func (s *service) collectStaffTimetableAssignments(ctx context.Context, staffID int64, from, to timezone.Date, includeCancelled bool) ([]staffTimetableAssignment, error) {
	assignments, err := s.cfg.InstanceStaffRepo.FindByStaffAndDateRange(ctx, staffID, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return nil, err
	}
	instanceIDs := make([]int64, 0, len(assignments))
	seen := make(map[int64]struct{})
	for _, assignment := range assignments {
		if _, ok := seen[assignment.InstanceID]; ok {
			continue
		}
		seen[assignment.InstanceID] = struct{}{}
		instanceIDs = append(instanceIDs, assignment.InstanceID)
	}
	instances, err := s.cfg.ActivityInstanceRepo.FindByIDs(ctx, instanceIDs)
	if err != nil {
		return nil, err
	}
	instancesByID := make(map[int64]*scheduleModels.ActivityInstance, len(instances))
	for _, instance := range instances {
		instancesByID[instance.ID] = instance
	}

	collected := make([]staffTimetableAssignment, 0, len(instanceIDs))
	seen = make(map[int64]struct{})
	for _, assignment := range assignments {
		if _, ok := seen[assignment.InstanceID]; ok {
			continue
		}
		seen[assignment.InstanceID] = struct{}{}
		instance := instancesByID[assignment.InstanceID]
		if instance == nil || (!includeCancelled && instance.Status == scheduleModels.InstanceStatusCancelled) {
			continue
		}
		collected = append(collected, staffTimetableAssignment{instance: instance, assignment: assignment})
	}
	return collected, nil
}

func (s *service) staffTimetableEvents(ctx context.Context, staffID int64, from, to timezone.Date) ([]Event, error) {
	return s.staffTimetableEventsWithCancelled(ctx, staffID, from, to, false)
}

func (s *service) staffTimetableFeedEvents(ctx context.Context, staffID int64, from, to timezone.Date) ([]Event, error) {
	return s.staffTimetableEventsWithCancelled(ctx, staffID, from, to, true)
}

func (s *service) staffTimetableEventsWithCancelled(ctx context.Context, staffID int64, from, to timezone.Date, includeCancelled bool) ([]Event, error) {
	assigned, err := s.collectStaffTimetableAssignments(ctx, staffID, from, to, includeCancelled)
	if err != nil {
		return nil, err
	}
	roomNames, err := s.timetableRoomNames(ctx, distinctTimetableRoomIDs(assigned))
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(assigned))
	for _, entry := range assigned {
		instance := entry.instance
		id := formatID(instance.ID)
		modifiedAt := instance.UpdatedAt
		if entry.assignment.UpdatedAt.After(modifiedAt) {
			modifiedAt = entry.assignment.UpdatedAt
		}
		event := Event{
			ID:          fmt.Sprintf("timetable:%d", instance.ID),
			Source:      EventSourceTimetable,
			TimetableID: &id,
			Title:       instance.Title,
			Description: instance.Description,
			StartDate:   instance.Date.String(),
			EndDate:     instance.Date.String(),
			StartTime:   formatClock(instance.StartTime),
			EndTime:     formatClock(instance.EndTime),
			AllDay:      false,
			Cancelled:   instance.Status == scheduleModels.InstanceStatusCancelled,
			ModifiedAt:  modifiedAt,
		}
		// A room deleted between assignment and this read misses the map;
		// Location then stays nil rather than becoming an empty string.
		if name := roomNames[timetableRoomID(instance, entry.assignment)]; name != "" {
			event.Location = &name
		}
		events = append(events, event)
	}
	return events, nil
}

// shiftsReferenceTypes reports whether any shift carries a ShiftTypeID —
// the guard that keeps windows without typed shifts free of the ListAll query.
func shiftsReferenceTypes(shifts []*scheduleModels.StaffShift) bool {
	for _, shift := range shifts {
		if shift.ShiftTypeID != nil {
			return true
		}
	}
	return false
}

// staffShiftEvents maps the staff member's Dienstplan shifts
// (schedule.staff_shifts) in the window to calendar events. Cancelled shifts
// stay hidden, mirroring how cancelled timetable instances are skipped. The
// range finder does not load the ShiftType relation, so names come from one
// batch ListAll (the tenant's shift-type table is small); a type missing from
// the map (concurrently deleted) falls back to the generic title.
// Location stays deliberately empty: schedule.staff_shifts carries no room
// column, so there is nothing to resolve (#2078).
func (s *service) staffShiftEvents(ctx context.Context, staffID int64, from, to timezone.Date) ([]Event, error) {
	return s.staffShiftEventsWithCancelled(ctx, staffID, from, to, false)
}

func (s *service) staffShiftFeedEvents(ctx context.Context, staffID int64, from, to timezone.Date) ([]Event, error) {
	return s.staffShiftEventsWithCancelled(ctx, staffID, from, to, true)
}

func (s *service) staffShiftEventsWithCancelled(ctx context.Context, staffID int64, from, to timezone.Date, includeCancelled bool) ([]Event, error) {
	if s.cfg.StaffShiftRepo == nil {
		return []Event{}, nil
	}
	shifts, err := s.cfg.StaffShiftRepo.FindByStaffAndDateRange(ctx, staffID, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return nil, err
	}
	typeNames := map[int64]string{}
	if s.cfg.ShiftTypeRepo != nil && shiftsReferenceTypes(shifts) {
		shiftTypes, err := s.cfg.ShiftTypeRepo.ListAll(ctx)
		if err != nil {
			return nil, err
		}
		for _, shiftType := range shiftTypes {
			typeNames[shiftType.ID] = shiftType.Name
		}
	}
	events := []Event{}
	for _, shift := range shifts {
		if shift.Cancelled && !includeCancelled {
			continue
		}
		title := "Dienst"
		if shift.ShiftTypeID != nil {
			if name := typeNames[*shift.ShiftTypeID]; name != "" {
				title = name
			}
		}
		event := Event{
			ID:         fmt.Sprintf("shift:%d", shift.ID),
			Source:     EventSourceShift,
			Title:      title,
			StartDate:  shift.Date.String(),
			EndDate:    shift.Date.String(),
			StartTime:  formatClock(shift.StartTime),
			EndTime:    formatClock(shift.EndTime),
			AllDay:     false,
			Cancelled:  shift.Cancelled,
			ModifiedAt: shift.UpdatedAt,
		}
		if shift.Notes != "" {
			notes := shift.Notes
			event.Description = &notes
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *service) resolveTargets(ctx context.Context, deliveryMode string, targets []AppointmentTarget) ([]appointments.AppointmentRecipientFields, []*calModels.AppointmentTarget, error) {
	status := calModels.ResponseStatusPending
	if deliveryMode == calModels.DeliveryModeInformational {
		status = calModels.ResponseStatusInfo
	}
	readSet, err := s.loadTargetResolutionReadSet(ctx, targets)
	if err != nil {
		return nil, nil, err
	}

	staffIDs := map[int64]struct{}{}
	guardianStudents := map[int64]map[int64]struct{}{}
	targetRows := make([]*calModels.AppointmentTarget, 0, len(targets))
	guardianCanReceive := func(guardianProfileID int64) (bool, error) {
		return readSet.activeGuardians[guardianProfileID], nil
	}
	addGuardian := func(guardianProfileID int64, studentID *int64) (bool, error) {
		if guardianProfileID <= 0 {
			return false, nil
		}
		active, err := guardianCanReceive(guardianProfileID)
		if err != nil {
			return false, err
		}
		if !active {
			return false, nil
		}
		if _, ok := guardianStudents[guardianProfileID]; !ok {
			guardianStudents[guardianProfileID] = map[int64]struct{}{}
		}
		if studentID != nil && *studentID > 0 {
			guardianStudents[guardianProfileID][*studentID] = struct{}{}
		}
		return true, nil
	}
	addStudentGuardians := func(studentID int64) (int, error) {
		added := 0
		for _, link := range readSet.linksByStudent[studentID] {
			if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
				ok, err := addGuardian(link.GuardianProfileID, &studentID)
				if err != nil {
					return 0, err
				}
				if ok {
					added++
				}
			}
		}
		return added, nil
	}
	// All guardian links and active profiles were loaded once before this loop;
	// target expansion below is now pure in-memory grouping.
	addStudentsGuardians := func(studentIDs []int64) (int, error) {
		added := 0
		for _, studentID := range studentIDs {
			count, err := addStudentGuardians(studentID)
			if err != nil {
				return 0, err
			}
			added += count
		}
		return added, nil
	}

	for _, target := range targets {
		targetRows = append(targetRows, &calModels.AppointmentTarget{
			TargetType:  target.Type,
			TargetID:    target.ID,
			TargetValue: target.Value,
		})
		switch target.Type {
		case calModels.TargetTypeAllStaff:
			// Only invite staff who can actually use the calendar (active
			// account + calendar:own); unreachable staff would leave RSVP
			// appointments permanently pending and skew attendee counts.
			for staffID := range readSet.reachableStaff {
				staffIDs[staffID] = struct{}{}
			}
		case calModels.TargetTypeStaff:
			if !readSet.reachableStaff[*target.ID] {
				return nil, nil, fmt.Errorf("%w: staff target is not available", ErrInvalidRequest)
			}
			staffIDs[*target.ID] = struct{}{}
		case calModels.TargetTypeGuardianProfile:
			if !readSet.activeGuardians[*target.ID] {
				return nil, nil, fmt.Errorf("%w: guardian target is not available", ErrInvalidRequest)
			}
			visible := false
			for _, link := range readSet.linksByGuardian[*target.ID] {
				if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
					visible = true
					studentID := link.StudentID
					if _, err := addGuardian(*target.ID, &studentID); err != nil {
						return nil, nil, err
					}
				}
			}
			if !visible {
				return nil, nil, fmt.Errorf("%w: guardian target is not portal-visible", ErrInvalidRequest)
			}
		case calModels.TargetTypeAllSchoolParents:
			// Every portal-active guardian of the school's ACTIVE students. Resolve
			// in bulk so a school-wide appointment stays a couple of queries, not one
			// per student. Filter to active students at the DB so pending or inactive
			// (e.g. former) families never receive school-wide appointments.
			studentIDs := activeStudentIDs(readSet.allSchoolStudents)
			added, err := addStudentsGuardians(studentIDs)
			if err != nil {
				return nil, nil, err
			}
			if added == 0 {
				return nil, nil, fmt.Errorf("%w: no reachable guardians at this school", ErrInvalidRequest)
			}
		case calModels.TargetTypeParentsByStudent:
			added, err := addStudentGuardians(*target.ID)
			if err != nil {
				return nil, nil, err
			}
			if added == 0 {
				return nil, nil, fmt.Errorf("%w: parent target has no reachable guardians", ErrInvalidRequest)
			}
		case calModels.TargetTypeParentsByGroup:
			// Only active students' guardians — a former student still assigned to
			// the group must not receive the group-wide appointment.
			studentIDs := activeStudentIDs(readSet.studentsByGroup[*target.ID])
			added, err := addStudentsGuardians(studentIDs)
			if err != nil {
				return nil, nil, err
			}
			if added == 0 {
				return nil, nil, fmt.Errorf("%w: parent target has no reachable guardians", ErrInvalidRequest)
			}
		case calModels.TargetTypeParentsByClass:
			// Only active students' guardians — a former student still tagged with
			// the class must not receive the class-wide appointment.
			studentIDs := activeStudentIDs(readSet.studentsByClass[normalizeCalendarClass(*target.Value)])
			added, err := addStudentsGuardians(studentIDs)
			if err != nil {
				return nil, nil, err
			}
			if added == 0 {
				return nil, nil, fmt.Errorf("%w: parent target has no reachable guardians", ErrInvalidRequest)
			}
		}
	}

	recipients := make([]appointments.AppointmentRecipientFields, 0, len(staffIDs)+len(guardianStudents))
	for staffID := range staffIDs {
		id := staffID
		recipients = append(recipients, appointments.AppointmentRecipientFields{
			RecipientType: calModels.RecipientTypeStaff,
			StaffID:       &id,
			Status:        status,
		})
	}
	for guardianProfileID, studentIDs := range guardianStudents {
		id := guardianProfileID
		studentList := make([]int64, 0, len(studentIDs))
		for studentID := range studentIDs {
			studentList = append(studentList, studentID)
		}
		recipients = append(recipients, appointments.AppointmentRecipientFields{
			RecipientType:     calModels.RecipientTypeGuardianProfile,
			GuardianProfileID: &id,
			Status:            status,
			StudentIDs:        studentList,
		})
	}
	return recipients, targetRows, nil
}

// activeStudentIDs returns the IDs of the students that are currently active. A
// bulk parent target (whole-school, a group, a class) must not fan out to the
// guardians of pending or inactive (e.g. former) students, who would otherwise
// receive appointment details and notifications for a school they left.
func activeStudentIDs(students []*userModels.Student) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.Status == userModels.StudentStatusActive {
			ids = append(ids, student.ID)
		}
	}
	return ids
}

func (s *service) guardianHasPortalVisibleStudent(ctx context.Context, guardianProfileID int64) (bool, error) {
	links, err := s.cfg.StudentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return false, err
	}
	for _, link := range links {
		if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) buildAppointmentOverview(ctx context.Context, appointment *calModels.Appointment, recipients []*calModels.AppointmentRecipient) (*AppointmentOverview, error) {
	staffIDs := make([]int64, 0)
	guardianIDs := make([]int64, 0)
	for _, recipient := range recipients {
		if recipient.StaffID != nil {
			staffIDs = append(staffIDs, *recipient.StaffID)
		}
		if recipient.GuardianProfileID != nil {
			guardianIDs = append(guardianIDs, *recipient.GuardianProfileID)
		}
	}

	staffByID, err := s.cfg.StaffRepo.FindWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	guardiansByID, err := s.cfg.GuardianProfileRepo.FindByIDs(ctx, guardianIDs)
	if err != nil {
		return nil, err
	}

	attendees := make([]AppointmentAttendee, 0, len(recipients))
	for _, recipient := range recipients {
		name := ""
		if recipient.StaffID != nil {
			name = staffName(staffByID[*recipient.StaffID])
		}
		if recipient.GuardianProfileID != nil {
			name = guardianName(guardiansByID[*recipient.GuardianProfileID])
		}
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("Empfänger %d", recipient.ID)
		}
		attendees = append(attendees, AppointmentAttendee{
			RecipientID:   formatID(recipient.ID),
			RecipientType: recipient.RecipientType,
			Name:          name,
			Status:        recipient.Status,
			RespondedAt:   recipient.RespondedAt,
		})
	}
	sort.SliceStable(attendees, func(i, j int) bool {
		if attendees[i].RecipientType != attendees[j].RecipientType {
			return attendees[i].RecipientType < attendees[j].RecipientType
		}
		return strings.ToLower(attendees[i].Name) < strings.ToLower(attendees[j].Name)
	})

	return &AppointmentOverview{
		AppointmentID:      formatID(appointment.ID),
		DeliveryMode:       appointment.DeliveryMode,
		OverviewVisibility: appointment.OverviewVisibility,
		Attendees:          attendees,
	}, nil
}

func canStaffViewOverview(appointment *calModels.Appointment, staffID int64, isRecipient bool) bool {
	if appointment == nil {
		return false
	}
	if appointment.OrganizerStaffID == staffID {
		return true
	}
	switch appointment.OverviewVisibility {
	case calModels.OverviewVisibilityStaff, calModels.OverviewVisibilityAll:
		return isRecipient
	default:
		return false
	}
}

func canParentViewOverview(appointment *calModels.Appointment, isRecipient bool) bool {
	return appointment != nil && isRecipient && appointment.OverviewVisibility == calModels.OverviewVisibilityAll
}

func appointmentEvent(appointment *calModels.Appointment, occurrenceDate timezone.Date, responseStatus *string, recipientID *int64, staffID int64) Event {
	appointmentID := formatID(appointment.ID)
	deliveryMode := appointment.DeliveryMode
	occurrence := occurrenceDate.String()
	endDate := occurrenceDate.AddDays(appointment.StartDate.DaysUntil(appointment.EndDate))
	isStaffRecipient := recipientID != nil
	var recipientIDString *string
	if recipientID != nil {
		value := formatID(*recipientID)
		recipientIDString = &value
	}
	organizerStaffID := formatID(appointment.OrganizerStaffID)
	return Event{
		ID:               fmt.Sprintf("appointment:%d:%s", appointment.ID, occurrence),
		Source:           EventSourceAppointment,
		AppointmentID:    &appointmentID,
		OccurrenceDate:   &occurrence,
		Title:            appointment.Title,
		Description:      appointment.Description,
		Location:         appointment.Location,
		StartDate:        occurrenceDate.String(),
		EndDate:          endDate.String(),
		StartTime:        formatClock(appointment.StartTime),
		EndTime:          formatClock(appointment.EndTime),
		AllDay:           appointment.AllDay,
		Cancelled:        appointment.CancelledAt != nil,
		DeliveryMode:     &deliveryMode,
		ResponseStatus:   responseStatus,
		RecipientID:      recipientIDString,
		OrganizerStaffID: &organizerStaffID,
		// Stay respondable for any real (non-informational) recipient, including
		// already accepted/declined ones — the respond endpoints allow changing
		// an existing RSVP, so users can correct an accidental answer. Only
		// informational recipients (and non-recipients) cannot respond, and a
		// cancelled appointment freezes RSVP entirely (matching the server-side
		// rejection in RespondTo*Invitation).
		CanRespond:      appointment.CancelledAt == nil && recipientID != nil && responseStatus != nil && *responseStatus != calModels.ResponseStatusInfo,
		CanEdit:         appointment.OrganizerStaffID == staffID,
		CanViewOverview: canStaffViewOverview(appointment, staffID, isStaffRecipient),
	}
}

func applyOverride(event *Event, override *calModels.AppointmentOccurrenceOverride) {
	if override == nil {
		return
	}
	if override.Title != nil {
		event.Title = *override.Title
	}
	if override.Description != nil {
		event.Description = override.Description
	}
	if override.Location != nil {
		event.Location = override.Location
	}
	if override.StartDate != nil {
		event.StartDate = override.StartDate.String()
	}
	if override.EndDate != nil {
		event.EndDate = override.EndDate.String()
	}
	if override.StartTime != nil {
		event.StartTime = formatClock(*override.StartTime)
	}
	if override.EndTime != nil {
		event.EndTime = formatClock(*override.EndTime)
	}
	if override.AllDay != nil {
		event.AllDay = *override.AllDay
	}
}

func staffRecipientStatus(recipients []*calModels.AppointmentRecipient, staffID int64) (*string, *int64) {
	for _, recipient := range recipients {
		if recipient.StaffID != nil && *recipient.StaffID == staffID {
			status := recipient.Status
			id := recipient.ID
			return &status, &id
		}
	}
	return nil, nil
}

func (s *service) guardianRecipientStatusForStudents(ctx context.Context, recipients []*calModels.AppointmentRecipient, guardianProfileIDs []int64, studentIDs []int64) (*string, *int64, error) {
	allowedGuardians := int64Set(guardianProfileIDs)
	allowedStudents := int64Set(studentIDs)
	recipientIDs := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		if recipient.GuardianProfileID == nil {
			continue
		}
		if _, ok := allowedGuardians[*recipient.GuardianProfileID]; ok {
			recipientIDs = append(recipientIDs, recipient.ID)
		}
	}
	links, err := s.cfg.Appointments.FindAppointmentRecipientStudents(ctx, recipientIDs)
	if err != nil {
		return nil, nil, err
	}
	visibleRecipients := map[int64]struct{}{}
	for _, link := range links {
		if _, ok := allowedStudents[link.StudentID]; ok {
			visibleRecipients[link.RecipientID] = struct{}{}
		}
	}
	for _, recipient := range recipients {
		if recipient.GuardianProfileID == nil {
			continue
		}
		if _, ok := allowedGuardians[*recipient.GuardianProfileID]; !ok {
			continue
		}
		if _, ok := visibleRecipients[recipient.ID]; ok {
			status := recipient.Status
			id := recipient.ID
			return &status, &id, nil
		}
	}
	return nil, nil, nil
}

func (s *service) recipientHasVisibleStudent(ctx context.Context, recipientID int64, allowedStudentIDs map[int64]struct{}) (bool, error) {
	links, err := s.cfg.Appointments.FindAppointmentRecipientStudents(ctx, []int64{recipientID})
	if err != nil {
		return false, err
	}
	for _, link := range links {
		if _, ok := allowedStudentIDs[link.StudentID]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) parentChildren(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	var children []*parentModels.ChildSummary
	if err := tenant.WithAdminTx(ctx, s.cfg.DB, func(adminCtx context.Context, _ bun.Tx) error {
		rows, err := s.cfg.ChildRepo.ListByAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		children = rows
		return nil
	}); err != nil {
		return nil, err
	}
	return children, nil
}

func groupChildrenByTenant(children []*parentModels.ChildSummary) map[int64][]*parentModels.ChildSummary {
	out := make(map[int64][]*parentModels.ChildSummary)
	for _, child := range children {
		out[child.TenantID] = append(out[child.TenantID], child)
	}
	return out
}

func distinctGuardianProfileIDs(children []*parentModels.ChildSummary) []int64 {
	seen := map[int64]struct{}{}
	out := []int64{}
	for _, child := range children {
		if _, ok := seen[child.GuardianProfileID]; ok {
			continue
		}
		seen[child.GuardianProfileID] = struct{}{}
		out = append(out, child.GuardianProfileID)
	}
	return out
}

func distinctChildStudentIDs(children []*parentModels.ChildSummary) []int64 {
	seen := map[int64]struct{}{}
	out := []int64{}
	for _, child := range children {
		if _, ok := seen[child.StudentID]; ok {
			continue
		}
		seen[child.StudentID] = struct{}{}
		out = append(out, child.StudentID)
	}
	return out
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func int64Set(values []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func occurrenceDatesForAppointments(appointments []*calModels.Appointment, recurrenceByAppointment map[int64]*calModels.RecurrenceRule, from, to timezone.Date) []timezone.Date {
	seen := map[string]struct{}{}
	dates := []timezone.Date{}
	for _, appointment := range appointments {
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			continue
		}
		for _, occurrence := range expandOccurrences(appointment, recurrence, from, to) {
			key := occurrence.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			dates = append(dates, occurrence)
		}
	}
	return dates
}

func normalizeWeekdays(days []string) []string {
	out := make([]string, 0, len(days))
	seen := make(map[string]bool, len(days))
	for _, day := range days {
		normalized := strings.ToLower(strings.TrimSpace(day))
		// De-duplicate: a repeated weekday would make the count-bounded expansion
		// emit that date twice and exhaust occurrence_count early.
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func recurrenceRuleFromRequest(req *RecurrenceRequest) *calModels.RecurrenceRule {
	if req == nil {
		return nil
	}
	rule := &calModels.RecurrenceRule{
		Frequency:       req.Frequency,
		IntervalCount:   req.IntervalCount,
		Weekdays:        normalizeWeekdays(req.Weekdays),
		MonthDays:       req.MonthDays,
		EndsOn:          toCalendarDatePtr(req.EndsOn),
		OccurrenceCount: req.OccurrenceCount,
	}
	if rule.IntervalCount == 0 {
		rule.IntervalCount = 1
	}
	return rule
}

func dateRangesOverlap(aStart, aEnd, bStart, bEnd timezone.Date) bool {
	return !aEnd.Before(bStart) && !aStart.After(bEnd)
}

func sortEvents(events []Event) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].StartDate != events[j].StartDate {
			return events[i].StartDate < events[j].StartDate
		}
		if events[i].StartTime != events[j].StartTime {
			return events[i].StartTime < events[j].StartTime
		}
		return events[i].ID < events[j].ID
	})
}

func formatClock(t time.Time) string {
	return timezone.NormalizeWallClock(t).Format("15:04")
}

func staffName(staff *userModels.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName)
}

func guardianName(parent *userModels.GuardianProfile) string {
	if parent == nil {
		return ""
	}
	name := strings.TrimSpace(parent.FirstName + " " + parent.LastName)
	if name == "" && parent.Email != nil {
		return *parent.Email
	}
	return name
}

func studentDisplayName(student *userModels.Student) string {
	if student == nil {
		return ""
	}
	if student.Person == nil {
		return fmt.Sprintf("Kind %d", student.ID)
	}
	return strings.TrimSpace(student.Person.FirstName + " " + student.Person.LastName)
}
