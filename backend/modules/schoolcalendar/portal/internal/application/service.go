package application

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
	"golang.org/x/sync/singleflight"
)

const (
	EventSourceAppointment      = portal.EventSourceAppointment
	EventSourceTimetable        = portal.EventSourceTimetable
	EventSourceShift            = portal.EventSourceShift
	maxCalendarWindowDays       = 92
	parentSearchCandidateFactor = 5
)

var (
	ErrInvalidRequest = portal.ErrInvalidRequest
	ErrForbidden      = portal.ErrForbidden
	ErrNotFound       = portal.ErrNotFound
	ErrConflict       = portal.ErrConflict
)

type Service = portal.Service

type FeedCleanupService = portal.FeedCleanupService

type FullService = portal.FullService

// StaffCalDAVService keeps the CalDAV protocol adapter behind one deep
// calendar interface: callers receive either the UI credentials or one fully
// authorised, privacy-filtered calendar snapshot.
type StaffCalDAVService = portal.StaffCalDAVService

type Config struct {
	Now                func() time.Time
	NewFeedToken       func() (string, error)
	Digest             func([]byte) string
	MissingRecord      func(error) bool
	MissingRecordCause error
	NotificationCopy   func(kind, locale string) (string, string)
	MotoLogoURL        func(string) string
	SchoolLogoURL      func(string, string) string
	PushSuppressed     func(error) bool
	NoDelivery         func(error) bool

	Appointments         appointmentcap.Capability
	StaffRepo            ports.StaffDirectory
	StudentRepo          ports.StudentDirectory
	GuardianProfileRepo  ports.GuardianDirectory
	StudentGuardianRepo  ports.GuardianRelationships
	ChildRepo            ports.Children
	GroupRepo            ports.Groups
	InstanceStaffRepo    ports.Assignments
	ActivityInstanceRepo ports.Instances
	// RoomRepo resolves room names for timetable events in one batch per
	// window (#2078). Optional: nil leaves Location empty instead of failing
	// the whole calendar, mirroring StaffShiftRepo/ShiftTypeRepo below.
	RoomRepo       ports.Rooms
	StaffShiftRepo ports.Shifts
	ShiftTypeRepo  ports.ShiftTypes
	UserContext    ports.Identity
	// SchoolDays are the closing days and holiday periods the moto schule
	// calendar shows (#4798).
	SchoolDays       ports.SchoolDays
	Runtime          ports.Runtime
	CalendarRenderer CalendarRenderer

	// Notification dependencies (all optional — nil disables e-mail; the in-app
	// calendar is unaffected).
	Outbox                 OutboxEnqueuer
	PushOutbox             PushOutboxCanceller
	SchoolRepo             ports.Schools
	Settings               LogoResolver
	CalDAVPolicy           CalDAVPolicy
	AccountRepo            FeedAccountRepo
	StaffFeedRepo          ports.StaffFeeds
	StaffFeedTombstoneRepo portal.FeedHistory
	PersonRepo             ports.Persons
	ParentsURL             string
	FrontendURL            string
	CalDAVURL              string
	// Today is the Berlin calendar day the subscription feeds count their
	// window from. Nil means the real day; tests pin it to fixed dates.
	Today func() appointmentcap.Date

	// Notifier and Preferences drive the guardian push/in-app notification that
	// accompanies the appointment e-mails (#1671). Both optional and both
	// required together: without consent there is nobody to address, so a nil
	// Preferences disables the push rather than broadcasting past consent.
	Notifier ports.Notifier
	// ReminderNotifier waits for Web Push acceptance so the scheduler can retry
	// a transient failure without permanently claiming the reminder delivery.
	ReminderNotifier ports.SynchronousNotifier
	Preferences      ports.Preferences

	// Logger is nil-safe (see service.logger()); notification dispatch is
	// fire-and-forget and reports its failures here instead of to the caller.
	Logger *slog.Logger
}

type CalendarRecurrence = portal.CalendarRecurrence

type CalendarEvent = portal.CalendarEvent

type CalendarRenderer = portal.CalendarRenderer

type CalDAVPolicy = portal.CalDAVPolicy

type StaffCalendarAccessInfo = portal.StaffCalendarAccessInfo

type StaffCalDAVCredentials = portal.StaffCalDAVCredentials

type StaffCalDAVCalendar = portal.StaffCalDAVCalendar

type StaffCalDAVItem = portal.StaffCalDAVItem

type service struct {
	cfg          Config
	feedCreation singleflight.Group
}

func NewService(cfg Config) *service {
	return &service{cfg: cfg}
}

func (s *service) withinAppointmentWrite(ctx context.Context, command func(context.Context) error) error {
	return s.cfg.Runtime.WithinAppointmentWrite(ctx, command)
}

type Event = portal.Event

type AppointmentDetail = portal.AppointmentDetail

type CreateAppointmentRequest = portal.CreateAppointmentRequest

// UpdateAppointmentRequest carries the editable fields of an existing
// appointment. Targeting (recipients) and delivery_mode are intentionally
// immutable after creation: re-resolving the audience on every edit would wipe
// the RSVP responses already collected. Changing the audience means cancelling
// and re-creating the appointment.
type UpdateAppointmentRequest = portal.UpdateAppointmentRequest

type AppointmentOverview = portal.AppointmentOverview

type AppointmentAttendee = portal.AppointmentAttendee

type RecurrenceRequest = portal.RecurrenceRequest

type AppointmentTarget = portal.AppointmentTarget

type RecipientOptions = portal.RecipientOptions

type StaffOption = portal.StaffOption

type ParentOption = portal.ParentOption

type GroupOption = portal.GroupOption

type StudentOption = portal.StudentOption

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

func formatClock(t time.Time) string {
	return normalizeWallClock(t).Format("15:04")
}

func (s *service) parentChildren(ctx context.Context, accountID int64) ([]*ports.ChildSummary, error) {
	var children []*ports.ChildSummary
	if err := s.cfg.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
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

func (s *service) ListMyParentEvents(ctx context.Context, accountID int64, from, to appointmentcap.Date) ([]Event, error) {
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
	events := []Event{}
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		var tenantEvents []Event
		if err := s.cfg.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			var err error
			tenantEvents, err = s.parentTenantEvents(txCtx, tenantID, tenantChildren, from, to)
			return err
		}); err != nil {
			return nil, err
		}
		events = append(events, tenantEvents...)
	}
	sortEvents(events)
	return events, nil
}

// parentTenantEvents lists the appointment events of one tenant that are
// addressed to the parent through the tenant's children, labelled with the
// tenant and school.
func (s *service) parentTenantEvents(ctx context.Context, tenantID int64, tenantChildren []*ports.ChildSummary, from, to appointmentcap.Date) ([]Event, error) {
	guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
	studentIDs := distinctChildStudentIDs(tenantChildren)
	appointments, err := s.listAppointmentsVisibleToGuardians(ctx, guardianProfileIDs, studentIDs, toCalendarDate(from), toCalendarDate(to))
	if err != nil {
		return nil, err
	}
	appointmentEvents, err := s.expandGuardianAppointmentEvents(ctx, appointments, guardianProfileIDs, studentIDs, from, to)
	if err != nil {
		return nil, err
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
	return appointmentEvents, nil
}

func (s *service) GetParentAppointmentOverview(ctx context.Context, accountID, appointmentID int64) (*AppointmentOverview, error) {
	if err := validateParentAppointmentRequest(accountID, appointmentID); err != nil {
		return nil, err
	}
	children, err := s.parentChildren(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var foundForbidden bool
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		var overview *AppointmentOverview
		var forbidden bool
		if err := s.cfg.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			var err error
			overview, forbidden, err = s.parentTenantAppointmentOverview(txCtx, appointmentID, tenantChildren)
			return err
		}); err != nil {
			return nil, err
		}
		if overview != nil {
			return overview, nil
		}
		foundForbidden = foundForbidden || forbidden
	}
	if foundForbidden {
		return nil, ErrForbidden
	}
	return nil, ErrNotFound
}

// parentTenantAppointmentOverview builds the overview inside one tenant when
// the appointment is addressed to the parent there. It reports forbidden when
// the parent may see the appointment but not its overview.
func (s *service) parentTenantAppointmentOverview(ctx context.Context, appointmentID int64, tenantChildren []*ports.ChildSummary) (*AppointmentOverview, bool, error) {
	// A soft-deleted appointment is not viewable by parents either.
	appointment, recipients, err := s.parentVisibleAppointment(ctx, appointmentID, tenantChildren)
	if err != nil || appointment == nil {
		return nil, false, err
	}
	if !canParentViewOverview(appointment, true) {
		return nil, true, nil
	}
	overview, err := s.buildAppointmentOverview(ctx, appointment, recipients)
	return overview, false, err
}

func (s *service) RespondToParentInvitation(ctx context.Context, accountID, recipientID int64, status string) error {
	if accountID <= 0 {
		return fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	if status != appointmentcap.ResponseStatusAccepted && status != appointmentcap.ResponseStatusDeclined {
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
		var updated bool
		if err := s.cfg.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			var err error
			updated, err = s.respondAsGuardianInTenant(txCtx, recipientID, status, tenantChildren)
			return err
		}); err != nil {
			return err
		}
		if updated {
			return nil
		}
	}
	return ErrNotFound
}

// respondAsGuardianInTenant records the RSVP when the recipient belongs to one
// of the tenant's guardian profiles through a visible child. It reports false
// (and no error) when the recipient is not this parent's in the tenant.
func (s *service) respondAsGuardianInTenant(ctx context.Context, recipientID int64, status string, tenantChildren []*ports.ChildSummary) (bool, error) {
	allowedProfiles := int64Set(distinctGuardianProfileIDs(tenantChildren))
	allowedStudentIDs := int64Set(distinctChildStudentIDs(tenantChildren))
	recipient, err := s.cfg.Appointments.FindAppointmentRecipient(ctx, recipientID)
	if err != nil {
		return false, err
	}
	if recipient == nil || recipient.GuardianProfileID == nil {
		return false, nil
	}
	if _, ok := allowedProfiles[*recipient.GuardianProfileID]; !ok {
		return false, nil
	}
	visible, err := s.recipientHasVisibleStudent(ctx, recipient.ID, allowedStudentIDs)
	if err != nil || !visible {
		return false, err
	}
	if recipient.Status == appointmentcap.ResponseStatusInfo {
		return false, fmt.Errorf("%w: informational appointments cannot be answered", ErrInvalidRequest)
	}
	appointment, err := s.findAppointment(ctx, recipient.AppointmentID)
	if err != nil {
		return false, err
	}
	// A soft-deleted appointment is unanswerable — treat as not found.
	if appointment == nil || appointment.DeletedAt != nil {
		return false, nil
	}
	if appointment.CancelledAt != nil {
		return false, fmt.Errorf("%w: appointment is cancelled", ErrInvalidRequest)
	}
	if err := s.cfg.Appointments.UpdateAppointmentRecipientResponse(ctx, recipientID, status); err != nil {
		return false, err
	}
	return true, nil
}
