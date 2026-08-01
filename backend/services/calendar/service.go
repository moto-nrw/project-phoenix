package calendar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
	RespondToStaffInvitation(ctx context.Context, recipientID int64, status string) error
	RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error
	RecipientOptions(ctx context.Context, query string, limit int) (*RecipientOptions, error)
	// EnqueueDueAppointmentReminders queues the guardian reminder for every
	// occurrence starting in [from, to). Driven by the scheduler, per tenant.
	EnqueueDueAppointmentReminders(ctx context.Context, from, to time.Time) (int, error)
}

type Config struct {
	AppointmentRepo      calModels.AppointmentRepository
	RecurrenceRepo       calModels.RecurrenceRuleRepository
	RecipientRepo        calModels.AppointmentRecipientRepository
	RecipientStudentRepo calModels.AppointmentRecipientStudentRepository
	TargetRepo           calModels.AppointmentTargetRepository
	OverrideRepo         calModels.AppointmentOccurrenceOverrideRepository
	StaffRepo            userModels.StaffRepository
	StudentRepo          userModels.StudentRepository
	GuardianProfileRepo  userModels.GuardianProfileRepository
	StudentGuardianRepo  userModels.StudentGuardianRepository
	ChildRepo            parentModels.ChildRepository
	GroupRepo            educationModels.GroupRepository
	InstanceStaffRepo    scheduleModels.InstanceStaffRepository
	InstanceStudentRepo  scheduleModels.InstanceStudentRepository
	ActivityInstanceRepo scheduleModels.ActivityInstanceRepository
	// RoomRepo resolves room names for timetable events in one batch per
	// window (#2078). Optional: nil leaves Location empty instead of failing
	// the whole calendar, mirroring StaffShiftRepo/ShiftTypeRepo below.
	RoomRepo       facilitiesModels.RoomRepository
	StaffShiftRepo scheduleModels.StaffShiftRepository
	ShiftTypeRepo  scheduleModels.ShiftTypeRepository
	// CareDays judges unfinished timetable blocks against the care plan; a
	// completed block is read from its persisted marker instead (#1747).
	CareDays    scheduleSvc.CareDayService
	UserContext usercontext.UserContextService
	DB          *bun.DB

	// Notification dependencies (all optional — nil disables e-mail; the in-app
	// calendar is unaffected).
	Outbox      OutboxEnqueuer
	SchoolRepo  platformModels.SchoolRepository
	Settings    LogoResolver
	AccountRepo FeedAccountRepo
	ParentsURL  string

	// Notifier and Preferences drive the guardian push/in-app notification that
	// accompanies the appointment e-mails (#1671). Both optional and both
	// required together: without consent there is nobody to address, so a nil
	// Preferences disables the push rather than broadcasting past consent.
	Notifier    notifications.Service
	Preferences notifications.PreferenceService

	// Logger is nil-safe (see service.logger()); notification dispatch is
	// fire-and-forget and reports its failures here instead of to the caller.
	Logger *slog.Logger
}

type service struct {
	cfg Config
}

func NewService(cfg Config) Service {
	return &service{cfg: cfg}
}

type Event struct {
	ID               string  `json:"id"`
	Source           string  `json:"source"`
	AppointmentID    *string `json:"appointment_id,omitempty"`
	OccurrenceDate   *string `json:"occurrence_date,omitempty"`
	TimetableID      *string `json:"timetable_id,omitempty"`
	StudentID        *string `json:"student_id,omitempty"`
	StudentName      *string `json:"student_name,omitempty"`
	TenantID         *string `json:"tenant_id,omitempty"`
	SchoolName       *string `json:"school_name,omitempty"`
	Title            string  `json:"title"`
	Description      *string `json:"description,omitempty"`
	Location         *string `json:"location,omitempty"`
	StartDate        string  `json:"start_date"`
	EndDate          string  `json:"end_date"`
	StartTime        string  `json:"start_time"`
	EndTime          string  `json:"end_time"`
	AllDay           bool    `json:"all_day"`
	Cancelled        bool    `json:"cancelled"`
	Recurring        bool    `json:"recurring"`
	DeliveryMode     *string `json:"delivery_mode,omitempty"`
	ResponseStatus   *string `json:"response_status,omitempty"`
	RecipientID      *string `json:"recipient_id,omitempty"`
	OrganizerStaffID *string `json:"organizer_staff_id,omitempty"`
	CanRespond       bool    `json:"can_respond"`
	CanEdit          bool    `json:"can_edit"`
	CanViewOverview  bool    `json:"can_view_overview"`
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

	appointments, err := s.cfg.AppointmentRepo.ListVisibleForStaff(ctx, staff.ID, from, to)
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
			appointments, err := s.cfg.AppointmentRepo.ListVisibleForGuardianProfiles(txCtx, guardianProfileIDs, studentIDs, from, to)
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
			timetableEvents, err := s.parentTimetableEvents(txCtx, tenantChildren, from, to)
			if err != nil {
				return err
			}
			events = append(events, appointmentEvents...)
			events = append(events, timetableEvents...)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sortEvents(events)
	return events, nil
}

func (s *service) CreateStaffAppointment(ctx context.Context, req CreateAppointmentRequest) (*AppointmentDetail, error) {
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
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		StartTime:          timezone.WallClock(req.StartTime),
		EndTime:            timezone.WallClock(req.EndTime),
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

	recipients, recipientStudents, targets, err := s.resolveTargets(ctx, appointment.DeliveryMode, req.Targets)
	if err != nil {
		return nil, err
	}

	if err := s.cfg.AppointmentRepo.Create(ctx, appointment); err != nil {
		return nil, err
	}

	if recurrence != nil {
		recurrence.AppointmentID = appointment.ID
		if err := s.cfg.RecurrenceRepo.Create(ctx, recurrence); err != nil {
			return nil, err
		}
	}

	for _, recipient := range recipients {
		recipient.AppointmentID = appointment.ID
	}
	if err := s.cfg.RecipientRepo.CreateMany(ctx, recipients); err != nil {
		return nil, err
	}
	recipientIDByKey := make(map[string]int64, len(recipients))
	for _, recipient := range recipients {
		recipientIDByKey[recipientKey(recipient.RecipientType, recipient.StaffID, recipient.GuardianProfileID)] = recipient.ID
	}
	for _, link := range recipientStudents {
		key := recipientKey(calModels.RecipientTypeGuardianProfile, nil, &link.RecipientID)
		if recipientID, ok := recipientIDByKey[key]; ok {
			link.RecipientID = recipientID
		}
	}
	if err := s.cfg.RecipientStudentRepo.CreateMany(ctx, recipientStudents); err != nil {
		return nil, err
	}

	for _, target := range targets {
		target.AppointmentID = appointment.ID
	}
	if err := s.cfg.TargetRepo.ReplaceForAppointment(ctx, appointment.ID, targets); err != nil {
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
	appointment, err := s.cfg.AppointmentRepo.FindByID(ctx, appointmentID)
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

// appointmentDetail reloads the full detail (recurrence, recipients, targets)
// for an appointment. Used by the lifecycle operations so callers (and the
// notification layer in Phase B) get the same shape as CreateStaffAppointment.
func (s *service) appointmentDetail(ctx context.Context, appointment *calModels.Appointment) (*AppointmentDetail, error) {
	recurrence, err := s.cfg.RecurrenceRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	targets, err := s.cfg.TargetRepo.FindByAppointmentID(ctx, appointment.ID)
	if err != nil {
		return nil, err
	}
	return &AppointmentDetail{
		Appointment: appointment,
		Recurrence:  recurrence,
		Recipients:  recipients,
		Targets:     targets,
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
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
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
	appointment.StartDate = req.StartDate
	appointment.EndDate = req.EndDate
	appointment.StartTime = timezone.WallClock(req.StartTime)
	appointment.EndTime = timezone.WallClock(req.EndTime)
	appointment.AllDay = req.AllDay
	appointment.OverviewVisibility = req.OverviewVisibility
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

	if err := s.cfg.AppointmentRepo.Update(ctx, appointment); err != nil {
		// A concurrent cancel/delete transitioned the appointment between load and
		// write, so the conditional update matched nothing. Abort before touching
		// recurrence/overrides or sending an "updated" notice; the tenant tx rolls
		// back. Surface a conflict rather than a bogus success.
		if errors.Is(err, calModels.ErrAppointmentLifecycleConflict) {
			return nil, fmt.Errorf("%w: appointment was cancelled or deleted", ErrConflict)
		}
		return nil, err
	}
	// Replace the recurrence rule wholesale: the DB enforces one rule per
	// appointment, so drop the old row and recreate if the edit still recurs.
	if err := s.cfg.RecurrenceRepo.DeleteByAppointmentID(ctx, appointment.ID); err != nil {
		return nil, err
	}
	if recurrence != nil {
		if err := s.cfg.RecurrenceRepo.Create(ctx, recurrence); err != nil {
			return nil, err
		}
	}
	// Editing the series is a whole-series operation, so per-occurrence
	// cancellations ("Nur diesen Termin") from the old cadence no longer apply.
	// Drop them; otherwise a date reused by the new recurrence would be silently
	// suppressed (and stale EXDATEs would leak into the subscription feed/ICS).
	if err := s.cfg.OverrideRepo.DeleteByAppointmentID(ctx, appointment.ID); err != nil {
		return nil, err
	}

	// Kill any not-yet-sent notice queued by a prior create/update so the worker
	// can't deliver stale title/date/location (or a create notice the edit turned
	// off), then optionally enqueue a fresh update notice — mirrors cancel/delete.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment updated"); err != nil {
		return nil, err
	}
	if req.SendEmail {
		if err := s.notifyGuardians(ctx, appointment, platformModels.EmailKindAppointmentUpdated); err != nil {
			return nil, err
		}
		s.notifyGuardianDevices(ctx, appointment, platformModels.EmailKindAppointmentUpdated)
	}

	return s.appointmentDetail(ctx, appointment)
}

func (s *service) CancelStaffAppointment(ctx context.Context, appointmentID int64) (*AppointmentDetail, error) {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	if appointment.CancelledAt == nil {
		// Cancel via the dedicated conditional update (not Update): it only writes
		// cancelled_at/revision, so it can't clobber a concurrent edit and — being
		// WHERE cancelled_at IS NULL AND deleted_at IS NULL — matches nothing once a
		// concurrent cancel or delete has won. `transitioned` is true only for the
		// caller that actually flipped the row.
		transitioned, err := s.cfg.AppointmentRepo.Cancel(ctx, appointment.ID)
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
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return err
	}
	// Deleting is silent (no notice); kill any queued e-mails first so the worker
	// doesn't send a mail for a row we're about to remove or tombstone.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "appointment deleted"); err != nil {
		return err
	}
	// A subscribed external calendar (a parent's .ics subscription) only drops an
	// appointment when it re-receives the SAME UID with STATUS:CANCELLED at a
	// higher SEQUENCE. Hard-deleting a feed-visible appointment makes it vanish
	// from the feed, which many clients keep rather than purge — the stale event
	// then lingers in the parent's calendar forever. So a feed-visible appointment
	// (one with guardian recipients) is SOFT-deleted: it disappears from every
	// interactive staff/parent calendar (those queries filter deleted_at IS NULL),
	// yet the subscription feed re-exports it as a durable STATUS:CANCELLED
	// tombstone (retained by deletion time, independent of the date lookback) with
	// a bumped SEQUENCE so even long-offline subscribers eventually purge it.
	// Purely staff-internal appointments never reach a subscription feed, so they
	// are removed outright.
	feedVisible, err := s.appointmentHasGuardianRecipients(ctx, appointment.ID)
	if err != nil {
		return err
	}
	if feedVisible {
		return s.cfg.AppointmentRepo.SoftDelete(ctx, appointment.ID)
	}
	// Child rows (recurrence, recipients, recipient-students, targets, occurrence
	// overrides) are removed by ON DELETE CASCADE on the appointment FK.
	return s.cfg.AppointmentRepo.Delete(ctx, appointment.ID)
}

// appointmentHasGuardianRecipients reports whether the appointment has any
// materialized guardian-profile recipient — the marker that it can appear in a
// parent's iCalendar subscription feed. Guardian-facing targets (a class, a
// group, whole-school parents) are expanded into concrete guardian_profile
// recipient rows at create/update time, so checking the recipients is sufficient.
func (s *service) appointmentHasGuardianRecipients(ctx context.Context, appointmentID int64) (bool, error) {
	recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointmentID)
	if err != nil {
		return false, err
	}
	for _, recipient := range recipients {
		if recipient.RecipientType == calModels.RecipientTypeGuardianProfile {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) CancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate timezone.Date) error {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
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
	recurrence, err := s.cfg.RecurrenceRepo.FindByAppointmentID(ctx, appointment.ID)
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
	existing, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(ctx, []int64{appointment.ID}, []timezone.Date{occurrenceDate})
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
	if err := s.cfg.OverrideRepo.CancelOccurrence(ctx, appointment.ID, occurrenceDate); err != nil {
		return err
	}
	// A queued create/update notice announces the appointment's first occurrence;
	// removing an occurrence (possibly that first one) could otherwise deliver a
	// mail for a date parents will no longer see. Kill any pending notice, matching
	// the cancel/delete/update stale-notification cleanup.
	if err := s.cancelPendingNotifications(ctx, appointment.ID, "occurrence cancelled"); err != nil {
		return err
	}
	// Bump the parent revision so the feed re-exports with a higher SEQUENCE and
	// subscribers honour the new EXDATE.
	return s.cfg.AppointmentRepo.BumpRevision(ctx, appointment.ID)
}

// occurrenceExists reports whether occurrenceDate is one of the dates the
// recurrence generates. Membership is decided arithmetically via matchesRule
// (O(1): frequency/interval/weekday/month-day and occurrenceDate >= StartDate)
// plus the EndsOn bound, so cancelling a far-future date never walks every day
// since the series start. Only a count-bounded rule needs expansion, and that is
// bounded by the (small) occurrence count rather than the distance to the date.
func occurrenceExists(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, occurrenceDate timezone.Date) bool {
	if !matchesRule(appointment.StartDate, occurrenceDate, rule) {
		return false
	}
	if rule.EndsOn != nil && occurrenceDate.After(*rule.EndsOn) {
		return false
	}
	if rule.OccurrenceCount == nil {
		return true
	}
	// Count-bounded: the date matches the pattern and lies within EndsOn, so the
	// only remaining question is whether it falls within the first N occurrences.
	// expandOccurrences enumerates those by period (never day-by-day), so this
	// stays cheap even for a far-future date on an old series.
	for _, occ := range expandOccurrences(appointment, rule, appointment.StartDate, occurrenceDate) {
		if occ == occurrenceDate {
			return true
		}
	}
	return false
}

func (s *service) GetStaffAppointmentOverview(ctx context.Context, appointmentID int64) (*AppointmentOverview, error) {
	if appointmentID <= 0 {
		return nil, fmt.Errorf("%w: appointment id is required", ErrInvalidRequest)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	appointment, err := s.cfg.AppointmentRepo.FindByID(ctx, appointmentID)
	if err != nil {
		return nil, err
	}
	// A soft-deleted appointment is gone from every interactive surface, so its
	// overview is not reachable even with a retained appointment ID.
	if appointment == nil || appointment.DeletedAt != nil {
		return nil, ErrNotFound
	}
	recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
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
			appointment, err := s.cfg.AppointmentRepo.FindByID(txCtx, appointmentID)
			if err != nil {
				return err
			}
			// A soft-deleted appointment is not viewable by parents either.
			if appointment == nil || appointment.DeletedAt != nil {
				return nil
			}
			recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(txCtx, appointment.ID)
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

	recipient, err := s.cfg.RecipientRepo.FindByID(ctx, recipientID)
	if err != nil {
		return err
	}
	if recipient == nil || recipient.StaffID == nil || *recipient.StaffID != staff.ID {
		return ErrNotFound
	}
	if recipient.Status == calModels.ResponseStatusInfo {
		return fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
	}
	appointment, err := s.cfg.AppointmentRepo.FindByID(ctx, recipient.AppointmentID)
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
	return s.cfg.RecipientRepo.UpdateResponse(ctx, recipientID, status)
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
			recipient, err := s.cfg.RecipientRepo.FindByID(txCtx, recipientID)
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
			appointment, err := s.cfg.AppointmentRepo.FindByID(txCtx, recipient.AppointmentID)
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
			if err := s.cfg.RecipientRepo.UpdateResponse(txCtx, recipientID, status); err != nil {
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

func (s *service) expandAppointmentEvents(ctx context.Context, appointments []*calModels.Appointment, staffID int64, from, to timezone.Date) ([]Event, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, from, to)
	overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(ctx, ids, occurrenceDates)
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[fmt.Sprintf("%d:%s", override.AppointmentID, override.OccurrenceDate.String())] = override
	}

	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
		if err != nil {
			return nil, err
		}
		status, recipientID := staffRecipientStatus(recipients, staffID)
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			if !dateRangesOverlap(appointment.StartDate, appointment.EndDate, from, to) {
				continue
			}
			event := appointmentEvent(appointment, appointment.StartDate, status, recipientID, staffID)
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
	recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, from, to)
	overrides, err := s.cfg.OverrideRepo.FindByAppointmentIDsAndOccurrenceDates(ctx, ids, occurrenceDates)
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*calModels.AppointmentOccurrenceOverride, len(overrides))
	for _, override := range overrides {
		overrideByAppointmentDate[fmt.Sprintf("%d:%s", override.AppointmentID, override.OccurrenceDate.String())] = override
	}

	events := make([]Event, 0, len(appointments))
	for _, appointment := range appointments {
		recipients, err := s.cfg.RecipientRepo.FindByAppointmentID(ctx, appointment.ID)
		if err != nil {
			return nil, err
		}
		status, recipientID, err := s.guardianRecipientStatusForStudents(ctx, recipients, guardianProfileIDs, studentIDs)
		if err != nil {
			return nil, err
		}
		recurrence := recurrenceByAppointment[appointment.ID]
		if recurrence == nil {
			if !dateRangesOverlap(appointment.StartDate, appointment.EndDate, from, to) {
				continue
			}
			event := appointmentEvent(appointment, appointment.StartDate, status, recipientID, 0)
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

// collectStaffTimetableAssignments walks the window day by day and keeps the
// first assignment per instance; cancelled instances drop out here, so their
// rooms never reach the batch lookup.
func (s *service) collectStaffTimetableAssignments(ctx context.Context, staffID int64, from, to timezone.Date) ([]staffTimetableAssignment, error) {
	collected := []staffTimetableAssignment{}
	seen := make(map[int64]struct{})
	for d := from; !d.After(to); d = d.AddDays(1) {
		assignments, err := s.cfg.InstanceStaffRepo.FindByStaffAndDate(ctx, staffID, d)
		if err != nil {
			return nil, err
		}
		for _, assignment := range assignments {
			if _, ok := seen[assignment.InstanceID]; ok {
				continue
			}
			seen[assignment.InstanceID] = struct{}{}
			instance, err := s.cfg.ActivityInstanceRepo.FindByID(ctx, assignment.InstanceID)
			if err != nil {
				return nil, err
			}
			if instance.Status == scheduleModels.InstanceStatusCancelled {
				continue
			}
			collected = append(collected, staffTimetableAssignment{instance: instance, assignment: assignment})
		}
	}
	return collected, nil
}

func (s *service) staffTimetableEvents(ctx context.Context, staffID int64, from, to timezone.Date) ([]Event, error) {
	assigned, err := s.collectStaffTimetableAssignments(ctx, staffID, from, to)
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
	if s.cfg.StaffShiftRepo == nil {
		return []Event{}, nil
	}
	shifts, err := s.cfg.StaffShiftRepo.FindByStaffAndDateRange(ctx, staffID, from, to)
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
		if shift.Cancelled {
			continue
		}
		title := "Dienst"
		if shift.ShiftTypeID != nil {
			if name := typeNames[*shift.ShiftTypeID]; name != "" {
				title = name
			}
		}
		event := Event{
			ID:        fmt.Sprintf("shift:%d", shift.ID),
			Source:    EventSourceShift,
			Title:     title,
			StartDate: shift.Date.String(),
			EndDate:   shift.Date.String(),
			StartTime: formatClock(shift.StartTime),
			EndTime:   formatClock(shift.EndTime),
			AllDay:    false,
		}
		if shift.Notes != "" {
			notes := shift.Notes
			event.Description = &notes
		}
		events = append(events, event)
	}
	return events, nil
}

// parentCareDays resolves the care-plan verdict for every child in the window,
// so an unfinished block can be judged against the plan. Returns an empty map
// when no care-day service is wired, which keeps the previous behaviour (show
// the event) instead of silently hiding care from parents.
func (s *service) parentCareDays(
	ctx context.Context, children []*parentModels.ChildSummary, from, to timezone.Date,
) (map[int64]map[timezone.Date]scheduleSvc.CareDayStatus, error) {
	empty := map[int64]map[timezone.Date]scheduleSvc.CareDayStatus{}
	if s.cfg.CareDays == nil {
		return empty, nil
	}
	studentIDs := distinctChildStudentIDs(children)
	if len(studentIDs) == 0 {
		return empty, nil
	}
	careDays, err := s.cfg.CareDays.ResolveForRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return careDays, nil
}

// notScheduledForParent reports whether a timetable block must stay out of the
// parent calendar because the care plan does not book the child that day
// (#1747). Showing a care event for a day their child was never booked into is
// the same false claim the attendance history refuses to make.
//
// The verdict has two sources, mirroring instanceStudentCareDay in
// api/timetable: a completed block is frozen — ending it wrote the marker, and
// a later care-plan edit must not rewrite what a finished day meant — while a
// planned or running block carries no marker yet and is judged against the
// current plan.
//
// Only an undecided row can be hidden: a check-in or a hand-made decision tells
// its own story and outranks the plan. A row a broad day status flipped to
// 'absent' is NOT such a decision — sick/excused/class trip lands on every
// expected row of the day, including days the child was never booked into care,
// and it keeps its provenance in student_status_day_id (a manual PATCH and a
// check-in both clear that column). Those rows stay subject to the care-day
// filter, or reporting a child sick would resurrect a parent-calendar event on
// a weekday they were never booked for.
//
// A cancelled care day ("kommt heute nicht") is deliberately NOT hidden. The
// day was booked, so the block still exists for that child — same reason
// ending a block still writes their absence (CareDayStatus.ExemptFromAbsence).
func notScheduledForParent(
	instance *scheduleModels.ActivityInstance,
	row *scheduleModels.InstanceStudent,
	careDays map[int64]map[timezone.Date]scheduleSvc.CareDayStatus,
) bool {
	if row.Status != scheduleModels.AttendanceStatusExpected && row.StudentStatusDayID == nil {
		return false
	}
	// Staff can set an unbooked slot back to 'expected' — "the plan is wrong,
	// this child is coming". That IS the hand-made decision, even though it
	// lands on the same status the automatic state carries, so it has to be
	// recognized by its own stamp (#1747 review).
	if row.ManualStatusAt != nil {
		return false
	}
	if instance.Status == scheduleModels.InstanceStatusCompleted {
		return row.NotScheduled
	}
	return careDays[row.StudentID][instance.Date] == scheduleSvc.CareDayNotScheduled
}

func (s *service) parentTimetableEvents(ctx context.Context, children []*parentModels.ChildSummary, from, to timezone.Date) ([]Event, error) {
	events := []Event{}
	seen := make(map[string]struct{})
	careDays, err := s.parentCareDays(ctx, children, from, to)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		rows, err := s.cfg.InstanceStudentRepo.FindByStudentAndDateRange(ctx, child.StudentID, from, to)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			key := fmt.Sprintf("%d:%d", child.StudentID, row.InstanceID)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			instance, err := s.cfg.ActivityInstanceRepo.FindByID(ctx, row.InstanceID)
			if err != nil {
				return nil, err
			}
			if instance.Status == scheduleModels.InstanceStatusCancelled {
				continue
			}
			if notScheduledForParent(instance, row, careDays) {
				continue
			}
			timetableID := formatID(instance.ID)
			studentID := formatID(child.StudentID)
			tenantID := formatID(child.TenantID)
			studentName := strings.TrimSpace(child.FirstName + " " + child.LastName)
			schoolName := child.SchoolName
			events = append(events, Event{
				ID:          fmt.Sprintf("timetable:%d:%d", child.StudentID, instance.ID),
				Source:      EventSourceTimetable,
				TimetableID: &timetableID,
				StudentID:   &studentID,
				StudentName: &studentName,
				TenantID:    &tenantID,
				SchoolName:  &schoolName,
				Title:       instance.Title,
				Description: instance.Description,
				StartDate:   instance.Date.String(),
				EndDate:     instance.Date.String(),
				StartTime:   formatClock(instance.StartTime),
				EndTime:     formatClock(instance.EndTime),
				AllDay:      false,
			})
		}
	}
	return events, nil
}

func (s *service) resolveTargets(ctx context.Context, deliveryMode string, targets []AppointmentTarget) ([]*calModels.AppointmentRecipient, []*calModels.AppointmentRecipientStudent, []*calModels.AppointmentTarget, error) {
	status := calModels.ResponseStatusPending
	if deliveryMode == calModels.DeliveryModeInformational {
		status = calModels.ResponseStatusInfo
	}

	staffIDs := map[int64]struct{}{}
	guardianStudents := map[int64]map[int64]struct{}{}
	activeGuardianCache := map[int64]bool{}
	targetRows := make([]*calModels.AppointmentTarget, 0, len(targets))
	guardianCanReceive := func(guardianProfileID int64) (bool, error) {
		if active, ok := activeGuardianCache[guardianProfileID]; ok {
			return active, nil
		}
		profiles, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, []int64{guardianProfileID})
		if err != nil {
			return false, err
		}
		_, active := profiles[guardianProfileID]
		activeGuardianCache[guardianProfileID] = active
		return active, nil
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
		links, err := s.cfg.StudentGuardianRepo.FindByStudentID(ctx, studentID)
		if err != nil {
			return 0, err
		}
		added := 0
		for _, link := range links {
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
	// addStudentsGuardians is the bulk equivalent for multi-student targets
	// (whole-school / group / class). It resolves every student's guardian links
	// AND their active-portal status in two queries total, seeding
	// activeGuardianCache so the per-guardian add below never re-queries — a
	// school-wide appointment would otherwise fan out into thousands of queries.
	addStudentsGuardians := func(studentIDs []int64) (int, error) {
		if len(studentIDs) == 0 {
			return 0, nil
		}
		links, err := s.cfg.StudentGuardianRepo.FindByStudentIDs(ctx, studentIDs)
		if err != nil {
			return 0, err
		}
		pending := make([]int64, 0, len(links))
		seenPending := map[int64]struct{}{}
		for _, link := range links {
			if !authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
				continue
			}
			if _, cached := activeGuardianCache[link.GuardianProfileID]; cached {
				continue
			}
			if _, dup := seenPending[link.GuardianProfileID]; dup {
				continue
			}
			seenPending[link.GuardianProfileID] = struct{}{}
			pending = append(pending, link.GuardianProfileID)
		}
		if len(pending) > 0 {
			active, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, pending)
			if err != nil {
				return 0, err
			}
			for _, id := range pending {
				_, ok := active[id]
				activeGuardianCache[id] = ok
			}
		}
		added := 0
		for _, link := range links {
			if !authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
				continue
			}
			studentID := link.StudentID
			ok, err := addGuardian(link.GuardianProfileID, &studentID)
			if err != nil {
				return 0, err
			}
			if ok {
				added++
			}
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
			reachable, err := s.cfg.StaffRepo.FindReachableCalendarStaffIDs(ctx, nil)
			if err != nil {
				return nil, nil, nil, err
			}
			for staffID := range reachable {
				staffIDs[staffID] = struct{}{}
			}
		case calModels.TargetTypeStaff:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: staff target requires id", ErrInvalidRequest)
			}
			reachable, err := s.cfg.StaffRepo.FindReachableCalendarStaffIDs(ctx, []int64{*target.ID})
			if err != nil {
				return nil, nil, nil, err
			}
			if !reachable[*target.ID] {
				return nil, nil, nil, fmt.Errorf("%w: staff target is not available", ErrInvalidRequest)
			}
			staffIDs[*target.ID] = struct{}{}
		case calModels.TargetTypeGuardianProfile:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: guardian target requires id", ErrInvalidRequest)
			}
			profiles, err := s.cfg.GuardianProfileRepo.FindActivePortalProfilesByIDs(ctx, []int64{*target.ID})
			if err != nil {
				return nil, nil, nil, err
			}
			if _, ok := profiles[*target.ID]; !ok {
				return nil, nil, nil, fmt.Errorf("%w: guardian target is not available", ErrInvalidRequest)
			}
			links, err := s.cfg.StudentGuardianRepo.FindByGuardianProfileID(ctx, *target.ID)
			if err != nil {
				return nil, nil, nil, err
			}
			visible := false
			for _, link := range links {
				if authorize.StudentGuardianHasPermission(link, authorize.GuardianPermissionPortalAccess) {
					visible = true
					studentID := link.StudentID
					if _, err := addGuardian(*target.ID, &studentID); err != nil {
						return nil, nil, nil, err
					}
				}
			}
			if !visible {
				return nil, nil, nil, fmt.Errorf("%w: guardian target is not portal-visible", ErrInvalidRequest)
			}
		case calModels.TargetTypeAllSchoolParents:
			// Every portal-active guardian of the school's ACTIVE students. Resolve
			// in bulk so a school-wide appointment stays a couple of queries, not one
			// per student. Filter to active students at the DB so pending or inactive
			// (e.g. former) families never receive school-wide appointments.
			students, err := s.cfg.StudentRepo.List(ctx, map[string]any{"status": string(userModels.StudentStatusActive)})
			if err != nil {
				return nil, nil, nil, err
			}
			studentIDs := activeStudentIDs(students)
			added, err := addStudentsGuardians(studentIDs)
			if err != nil {
				return nil, nil, nil, err
			}
			if added == 0 {
				return nil, nil, nil, fmt.Errorf("%w: no reachable guardians at this school", ErrInvalidRequest)
			}
		case calModels.TargetTypeParentsByStudent:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: student target requires id", ErrInvalidRequest)
			}
			added, err := addStudentGuardians(*target.ID)
			if err != nil {
				return nil, nil, nil, err
			}
			if added == 0 {
				return nil, nil, nil, fmt.Errorf("%w: parent target has no reachable guardians", ErrInvalidRequest)
			}
		case calModels.TargetTypeParentsByGroup:
			if target.ID == nil || *target.ID <= 0 {
				return nil, nil, nil, fmt.Errorf("%w: group target requires id", ErrInvalidRequest)
			}
			students, err := s.cfg.StudentRepo.FindByGroupID(ctx, *target.ID)
			if err != nil {
				return nil, nil, nil, err
			}
			// Only active students' guardians — a former student still assigned to
			// the group must not receive the group-wide appointment.
			studentIDs := activeStudentIDs(students)
			added, err := addStudentsGuardians(studentIDs)
			if err != nil {
				return nil, nil, nil, err
			}
			if added == 0 {
				return nil, nil, nil, fmt.Errorf("%w: parent target has no reachable guardians", ErrInvalidRequest)
			}
		case calModels.TargetTypeParentsByClass:
			if target.Value == nil || strings.TrimSpace(*target.Value) == "" {
				return nil, nil, nil, fmt.Errorf("%w: class target requires value", ErrInvalidRequest)
			}
			students, err := s.cfg.StudentRepo.FindBySchoolClass(ctx, strings.TrimSpace(*target.Value))
			if err != nil {
				return nil, nil, nil, err
			}
			// Only active students' guardians — a former student still tagged with
			// the class must not receive the class-wide appointment.
			studentIDs := activeStudentIDs(students)
			added, err := addStudentsGuardians(studentIDs)
			if err != nil {
				return nil, nil, nil, err
			}
			if added == 0 {
				return nil, nil, nil, fmt.Errorf("%w: parent target has no reachable guardians", ErrInvalidRequest)
			}
		default:
			return nil, nil, nil, fmt.Errorf("%w: unknown target type %q", ErrInvalidRequest, target.Type)
		}
	}

	recipients := make([]*calModels.AppointmentRecipient, 0, len(staffIDs)+len(guardianStudents))
	for staffID := range staffIDs {
		id := staffID
		recipients = append(recipients, &calModels.AppointmentRecipient{
			RecipientType: calModels.RecipientTypeStaff,
			StaffID:       &id,
			Status:        status,
		})
	}
	recipientStudents := []*calModels.AppointmentRecipientStudent{}
	for guardianProfileID, studentIDs := range guardianStudents {
		id := guardianProfileID
		recipients = append(recipients, &calModels.AppointmentRecipient{
			RecipientType:     calModels.RecipientTypeGuardianProfile,
			GuardianProfileID: &id,
			Status:            status,
		})
		for studentID := range studentIDs {
			recipientStudents = append(recipientStudents, &calModels.AppointmentRecipientStudent{
				RecipientID: guardianProfileID,
				StudentID:   studentID,
			})
		}
	}
	return recipients, recipientStudents, targetRows, nil
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

func guardianRecipientStatus(recipients []*calModels.AppointmentRecipient, guardianProfileIDs []int64) (*string, *int64) {
	allowed := int64Set(guardianProfileIDs)
	for _, recipient := range recipients {
		if recipient.GuardianProfileID == nil {
			continue
		}
		if _, ok := allowed[*recipient.GuardianProfileID]; ok {
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
	links, err := s.cfg.RecipientStudentRepo.FindByRecipientIDs(ctx, recipientIDs)
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
	links, err := s.cfg.RecipientStudentRepo.FindByRecipientIDs(ctx, []int64{recipientID})
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

func expandOccurrences(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, from, to timezone.Date) []timezone.Date {
	occurrences := []timezone.Date{}
	if rule.IntervalCount <= 0 {
		rule.IntervalCount = 1
	}
	span := appointment.StartDate.DaysUntil(appointment.EndDate)
	if rule.OccurrenceCount != nil {
		// Count-bounded: enumerate the first N occurrences by PERIOD (never a
		// day-by-day walk of decades of history for a sparse monthly/yearly series),
		// then keep those overlapping [from, to]. N is capped at
		// MaxRecurrenceOccurrenceCount during validation.
		for _, d := range boundedRecurrenceDates(appointment, rule, *rule.OccurrenceCount) {
			if dateRangesOverlap(d, d.AddDays(span), from, to) {
				occurrences = append(occurrences, d)
			}
		}
		return occurrences
	}
	// Open-ended or EndsOn-bounded: only the window matters, so anchor the scan at
	// the window (rewound by the appointment's own span to catch a multi-day
	// occurrence that begins before `from`), never before the series start, and
	// stop at `to` or EndsOn — whichever is earlier.
	scanStart := from.AddDays(-span)
	if scanStart.Before(appointment.StartDate) {
		scanStart = appointment.StartDate
	}
	scanEnd := to
	if rule.EndsOn != nil && rule.EndsOn.Before(scanEnd) {
		scanEnd = *rule.EndsOn
	}
	for d := scanStart; !d.After(scanEnd); d = d.AddDays(1) {
		if matchesRule(appointment.StartDate, d, rule) &&
			dateRangesOverlap(d, d.AddDays(span), from, to) {
			occurrences = append(occurrences, d)
		}
	}
	return occurrences
}

// boundedRecurrenceDates returns up to `limit` occurrence start-dates the rule
// generates on/after the appointment start, honouring EndsOn. It advances one
// interval-PERIOD per step (never day-by-day), so a sparse count-bounded
// monthly/yearly series with a large occurrence count is enumerated without
// scanning decades of history on every feed poll. Dates come back in
// chronological order — identical to the day-walk it replaces. Returns fewer
// than `limit` when the rule is exhausted (EndsOn reached, or no further
// reachable occurrence exists).
func boundedRecurrenceDates(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, limit int) []timezone.Date {
	if rule == nil || limit <= 0 {
		return nil
	}
	interval := rule.IntervalCount
	if interval <= 0 {
		interval = 1
	}
	start := appointment.StartDate
	out := make([]timezone.Date, 0, limit)
	past := func(d timezone.Date) bool {
		return rule.EndsOn != nil && d.After(*rule.EndsOn)
	}

	switch rule.Frequency {
	case calModels.RecurrenceFrequencyDaily:
		for k := 0; len(out) < limit; k++ {
			d := start.AddDays(k * interval)
			if past(d) {
				break
			}
			out = append(out, d)
		}

	case calModels.RecurrenceFrequencyWeekly:
		offsets := weekdayOffsets(normalizeWeekdays(rule.Weekdays))
		weekBase := weekStartMonday(start)
		// startOffset is start's own Monday-based weekday offset, used when no
		// explicit weekdays are given (the occurrence is the start weekday).
		startOffset := (int(start.Weekday()) + 6) % 7
		for w := 0; len(out) < limit; w += interval {
			weekStart := weekBase.AddDays(w * 7)
			if past(weekStart) {
				break
			}
			if len(offsets) == 0 {
				d := weekStart.AddDays(startOffset)
				if !d.Before(start) && !past(d) {
					out = append(out, d)
				}
				continue
			}
			for _, off := range offsets {
				if len(out) >= limit {
					break
				}
				d := weekStart.AddDays(off)
				if d.Before(start) || past(d) {
					continue
				}
				out = append(out, d)
			}
		}

	case calModels.RecurrenceFrequencyMonthly:
		days := monthlyCandidateDays(rule, start)
		// The (month-of-year, leap-phase) pattern of reachable months repeats
		// within this many steps, so this many consecutive empty months proves no
		// further occurrence exists (mirrors firstMonthlyOccurrence's bound).
		maxEmpty := 4*(12/gcdInt(interval, 12)) + 12
		empty := 0
		for k := 0; len(out) < limit; k++ {
			year, month := addMonthsTo(start.Year, int(start.Month), k*interval)
			if past(timezone.NewDate(year, time.Month(month), 1)) {
				break
			}
			dim := daysInMonth(year, month)
			produced := 0
			for _, day := range days {
				if len(out) >= limit {
					break
				}
				if day < 1 || day > dim {
					continue
				}
				d := timezone.NewDate(year, time.Month(month), day)
				if d.Before(start) || past(d) {
					continue
				}
				out = append(out, d)
				produced++
			}
			if produced == 0 {
				empty++
				if empty > maxEmpty {
					break
				}
			} else {
				empty = 0
			}
		}

	case calModels.RecurrenceFrequencyYearly:
		// Only a Feb-29 rule is sparse (the day is absent in non-leap years); the
		// Gregorian leap cycle is 400 years, so this many empty steps proves
		// exhaustion regardless of interval.
		const maxEmpty = 400
		empty := 0
		for k := 0; len(out) < limit; k++ {
			year := start.Year + k*interval
			if daysInMonth(year, int(start.Month)) < start.Day {
				empty++
				if empty > maxEmpty {
					break
				}
				continue
			}
			d := timezone.NewDate(year, start.Month, start.Day)
			if past(d) {
				break
			}
			out = append(out, d)
			empty = 0
		}
	}
	return out
}

// monthlyCandidateDays returns the sorted day-of-month values a monthly rule can
// land on: its explicit month_days, or the start day when none are given (which
// is exactly what matchesRule compares against).
func monthlyCandidateDays(rule *calModels.RecurrenceRule, start timezone.Date) []int {
	if len(rule.MonthDays) == 0 {
		return []int{start.Day}
	}
	// De-duplicate: a repeated month day would emit that occurrence twice and
	// exhaust occurrence_count early.
	seen := make(map[int]bool, len(rule.MonthDays))
	days := make([]int, 0, len(rule.MonthDays))
	for _, day := range rule.MonthDays {
		if seen[day] {
			continue
		}
		seen[day] = true
		days = append(days, day)
	}
	sort.Ints(days)
	return days
}

// weekdayMondayOffset maps a lowercase weekday name to its Monday-based offset
// (Monday=0 … Sunday=6), matching weekStartMonday's anchoring.
var weekdayMondayOffset = map[string]int{
	"monday":    0,
	"tuesday":   1,
	"wednesday": 2,
	"thursday":  3,
	"friday":    4,
	"saturday":  5,
	"sunday":    6,
}

// weekdayOffsets converts normalized weekday names into sorted Monday-based
// offsets, dropping any unrecognised name. Validation guarantees valid names, so
// a non-empty weekly rule yields a non-empty, ascending result.
func weekdayOffsets(weekdays []string) []int {
	offsets := make([]int, 0, len(weekdays))
	for _, weekday := range weekdays {
		if off, ok := weekdayMondayOffset[weekday]; ok {
			offsets = append(offsets, off)
		}
	}
	sort.Ints(offsets)
	return offsets
}

// hasOccurrenceInWindow reports whether the recurrence produces at least one
// occurrence overlapping [from, to] WITHOUT walking every day since the series
// start. For an open-ended or EndsOn-bounded series it scans only the window
// (bounded by the window width plus the appointment's own multi-day span); for a
// count-bounded series it expands the bounded first-N occurrences. Used by the
// subscription feed, which is polled frequently for old series.
func hasOccurrenceInWindow(appointment *calModels.Appointment, rule *calModels.RecurrenceRule, from, to timezone.Date) bool {
	if rule == nil || to.Before(from) {
		return false
	}
	if rule.IntervalCount <= 0 {
		rule.IntervalCount = 1
	}
	if rule.OccurrenceCount != nil {
		// Bounded by the count: expandOccurrences enumerates the first N occurrences
		// by period (not day-by-day), and N is capped at MaxRecurrenceOccurrenceCount
		// during validation, so the scan can never walk unbounded history on a feed
		// poll even for a decades-old sparse monthly/yearly series.
		return len(expandOccurrences(appointment, rule, from, to)) > 0
	}
	// Scan only the window. Start early enough to catch a multi-day occurrence
	// that begins before `from` but reaches into it, but never before the series
	// start; stop at `to` or EndsOn, whichever is earlier.
	duration := appointment.StartDate.DaysUntil(appointment.EndDate)
	start := from.AddDays(-duration)
	if start.Before(appointment.StartDate) {
		start = appointment.StartDate
	}
	end := to
	if rule.EndsOn != nil && rule.EndsOn.Before(end) {
		end = *rule.EndsOn
	}
	for d := start; !d.After(end); d = d.AddDays(1) {
		if matchesRule(appointment.StartDate, d, rule) &&
			dateRangesOverlap(d, d.AddDays(duration), from, to) {
			return true
		}
	}
	return false
}

// firstRecurrenceOccurrence returns the first calendar date on or after the
// appointment's StartDate that satisfies the recurrence rule. ok=false means the
// rule has NO valid occurrence: either it is mathematically impossible (e.g. a
// monthly rule that only ever lands on February asking for day 31), or its first
// occurrence falls after EndsOn. The occurrence is computed analytically per
// frequency — no fixed scan horizon — so a valid but sparse rule (a large
// interval whose first match is many years out) is never mis-rejected nor
// mis-anchored to StartDate.
//
// For a weekly rule whose selected weekdays exclude StartDate's own weekday the
// first occurrence is later than StartDate; the in-app expansion
// (expandOccurrences) starts there, so ICS DTSTART must too.
func firstRecurrenceOccurrence(appointment *calModels.Appointment, rule *calModels.RecurrenceRule) (timezone.Date, bool) {
	if rule == nil {
		return appointment.StartDate, false
	}
	interval := rule.IntervalCount
	if interval <= 0 {
		interval = 1
	}
	first, exists := firstMatchingOccurrence(appointment.StartDate, interval, rule)
	if !exists {
		return appointment.StartDate, false
	}
	if rule.EndsOn != nil && first.After(*rule.EndsOn) {
		return first, false
	}
	return first, true
}

// firstMatchingOccurrence computes the first date on or after start that the rule
// matches, ignoring EndsOn. exists=false only for a genuinely impossible rule
// (monthly month_days that never fall in any reachable month). Daily, weekly
// without weekdays, monthly without month_days, and yearly always match start.
func firstMatchingOccurrence(start timezone.Date, interval int, rule *calModels.RecurrenceRule) (timezone.Date, bool) {
	switch rule.Frequency {
	case calModels.RecurrenceFrequencyDaily, calModels.RecurrenceFrequencyYearly:
		return start, true
	case calModels.RecurrenceFrequencyWeekly:
		weekdays := normalizeWeekdays(rule.Weekdays)
		if len(weekdays) == 0 {
			return start, true
		}
		offsets := weekdayOffsets(weekdays)
		if len(offsets) == 0 {
			return start, false
		}
		// Compute the first occurrence directly instead of scanning ~interval*7
		// days (which an arbitrarily large interval would blow up). The start's own
		// calendar week is interval-week 0 (always valid): the first occurrence is
		// its earliest selected weekday on/after start. If every selected weekday in
		// that week precedes start, the first occurrence is the earliest selected
		// weekday in the next valid week — interval calendar weeks later. This
		// matches matchesRule's calendar-week anchoring exactly.
		weekStart := weekStartMonday(start)
		for _, off := range offsets {
			d := weekStart.AddDays(off)
			if !d.Before(start) {
				return d, true
			}
		}
		return weekStart.AddDays(interval*7 + offsets[0]), true
	case calModels.RecurrenceFrequencyMonthly:
		if len(rule.MonthDays) == 0 {
			return start, true
		}
		return firstMonthlyOccurrence(start, interval, rule.MonthDays)
	default:
		return start, false
	}
}

// firstMonthlyOccurrence finds the first reachable month (start.Month + k·interval)
// that contains a requested month-day valid for that month and on/after start.
// The (month-of-year, leap-phase) pattern of reachable months repeats within a
// bounded number of steps, so scanning that many steps proves impossibility
// without an unbounded loop.
func firstMonthlyOccurrence(start timezone.Date, interval int, monthDays []int) (timezone.Date, bool) {
	days := append([]int(nil), monthDays...)
	sort.Ints(days)
	// 12/gcd distinct months of the year are reached within 12/gcd steps; the ×4
	// (+buffer) covers up to four leap cycles for a February-only day-29 rule.
	maxSteps := 4*(12/gcdInt(interval, 12)) + 12
	for k := 0; k <= maxSteps; k++ {
		year, month := addMonthsTo(start.Year, int(start.Month), k*interval)
		dim := daysInMonth(year, month)
		for _, day := range days {
			if day < 1 || day > dim {
				continue
			}
			candidate := timezone.NewDate(year, time.Month(month), day)
			if candidate.Before(start) {
				continue
			}
			return candidate, true
		}
	}
	return start, false
}

// addMonthsTo returns the year and 1-based month reached by adding delta months.
func addMonthsTo(year, month, delta int) (int, int) {
	total := year*12 + (month - 1) + delta
	return total / 12, total%12 + 1
}

// daysInMonth returns the number of days in the given 1-based month, honouring
// leap years (day 0 of the next month is the last day of this one).
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		a = -a
	}
	if a == 0 {
		return 1
	}
	return a
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

// weekStartMonday returns the Monday of the calendar week containing d. Used to
// anchor weekly-recurrence intervals to calendar weeks rather than to 7-day
// blocks measured from the appointment's start weekday.
func weekStartMonday(d timezone.Date) timezone.Date {
	// time.Weekday: Sunday=0 … Saturday=6; days since Monday = (weekday+6)%7.
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDays(-offset)
}

func matchesRule(start, candidate timezone.Date, rule *calModels.RecurrenceRule) bool {
	if candidate.Before(start) {
		return false
	}
	switch rule.Frequency {
	case calModels.RecurrenceFrequencyDaily:
		return start.DaysUntil(candidate)%rule.IntervalCount == 0
	case calModels.RecurrenceFrequencyWeekly:
		// Anchor the interval to calendar weeks (Mon–Sun), not to 7-day blocks
		// counted from the start weekday. Counting from the start day splits the
		// week on that weekday, so e.g. a biweekly Wed start with Mon+Wed would
		// wrongly include the Monday five days later (still "week 0") and skip
		// the Monday in the next valid calendar week.
		weeks := weekStartMonday(start).DaysUntil(weekStartMonday(candidate)) / 7
		if weeks%rule.IntervalCount != 0 {
			return false
		}
		weekdays := normalizeWeekdays(rule.Weekdays)
		if len(weekdays) == 0 {
			return candidate.Weekday() == start.Weekday()
		}
		return containsString(weekdays, strings.ToLower(candidate.Weekday().String()))
	case calModels.RecurrenceFrequencyMonthly:
		months := (candidate.Year-start.Year)*12 + int(candidate.Month-start.Month)
		if months < 0 || months%rule.IntervalCount != 0 {
			return false
		}
		if len(rule.MonthDays) == 0 {
			return candidate.Day == start.Day
		}
		for _, day := range rule.MonthDays {
			if candidate.Day == day {
				return true
			}
		}
		return false
	case calModels.RecurrenceFrequencyYearly:
		years := candidate.Year - start.Year
		return years >= 0 && years%rule.IntervalCount == 0 && candidate.Month == start.Month && candidate.Day == start.Day
	default:
		return false
	}
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
		EndsOn:          req.EndsOn,
		OccurrenceCount: req.OccurrenceCount,
	}
	if rule.IntervalCount == 0 {
		rule.IntervalCount = 1
	}
	return rule
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
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
	return timezone.WallClock(t).Format("15:04")
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

func recipientKey(recipientType string, staffID, guardianProfileID *int64) string {
	switch recipientType {
	case calModels.RecipientTypeStaff:
		if staffID != nil {
			return fmt.Sprintf("staff:%d", *staffID)
		}
	case calModels.RecipientTypeGuardianProfile:
		if guardianProfileID != nil {
			return fmt.Sprintf("guardian:%d", *guardianProfileID)
		}
	}
	return recipientType + ":0"
}
