package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
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
	RoomRepo         ports.Rooms
	StaffShiftRepo   ports.Shifts
	ShiftTypeRepo    ports.ShiftTypes
	UserContext      ports.Identity
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

func (s *service) ListMyStaffEvents(ctx context.Context, from, to appointmentcap.Date) ([]Event, error) {
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
	childrenByTenant := groupChildrenByTenant(children)
	events := []Event{}
	for tenantID, tenantChildren := range childrenByTenant {
		tenantID := tenantID
		tenantChildren := tenantChildren
		if err := s.cfg.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
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
		req.DeliveryMode = appointmentcap.DeliveryModeRSVPRequired
	}
	if req.OverviewVisibility == "" {
		req.OverviewVisibility = appointmentcap.OverviewVisibilityOrganizer
	}
	if req.EndDate.IsZero() {
		req.EndDate = req.StartDate
	}

	appointment := &appointmentcap.Appointment{
		OrganizerStaffID:   staff.ID,
		Title:              req.Title,
		Description:        req.Description,
		Location:           req.Location,
		StartDate:          toCalendarDate(req.StartDate),
		EndDate:            toCalendarDate(req.EndDate),
		StartTime:          normalizeWallClock(req.StartTime),
		EndTime:            normalizeWallClock(req.EndTime),
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

	created, persistedTargets, err := s.cfg.Appointments.CreateAppointment(ctx, appointmentcap.CreateAppointment{
		AppointmentFields: appointmentCapabilityFields(appointment),
		Targets:           appointmentTargetFields(targets),
	})
	if err != nil {
		return nil, err
	}
	appointment = created
	targets = persistedTargets

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
		if err := s.notifyGuardians(ctx, appointment, ports.EmailKindAppointmentPublished); err != nil {
			return nil, err
		}
		s.notifyGuardianDevices(ctx, appointment, ports.EmailKindAppointmentPublished)
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
func (s *service) loadOrganizedAppointment(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
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

func (s *service) findAppointment(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
	appointment, err := s.cfg.Appointments.FindAppointment(ctx, appointmentID)
	if errors.Is(err, appointmentcap.ErrAppointmentNotFound) {
		return nil, nil
	}
	return appointment, err
}

func (s *service) findAppointmentForUpdate(ctx context.Context, appointmentID int64) (*appointmentcap.Appointment, error) {
	appointment, err := s.cfg.Appointments.FindAppointmentForUpdate(ctx, appointmentID)
	if errors.Is(err, appointmentcap.ErrAppointmentNotFound) {
		return nil, fmt.Errorf("appointment %d not found: %w", appointmentID, s.cfg.MissingRecordCause)
	}
	return appointment, err
}

func appointmentCapabilityFields(value *appointmentcap.Appointment) appointmentcap.AppointmentFields {
	return appointmentcap.AppointmentFields{
		OrganizerStaffID: value.OrganizerStaffID, Title: value.Title, Description: value.Description,
		Location: value.Location, StartDate: value.StartDate, EndDate: value.EndDate,
		StartTime: value.StartTime, EndTime: value.EndTime, AllDay: value.AllDay,
		DeliveryMode: value.DeliveryMode, OverviewVisibility: value.OverviewVisibility,
		NotifyGuardians: value.NotifyGuardians,
	}
}

func appointmentTargetFields(values []*appointmentcap.AppointmentTarget) []appointmentcap.AppointmentTargetFields {
	result := make([]appointmentcap.AppointmentTargetFields, 0, len(values))
	for _, value := range values {
		result = append(result, appointmentcap.AppointmentTargetFields{
			TargetType: value.TargetType, TargetID: value.TargetID, TargetValue: value.TargetValue,
		})
	}
	return result
}

func (s *service) listAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]*appointmentcap.Appointment, error) {
	values, err := s.cfg.Appointments.ListAppointmentsVisibleToStaff(ctx, staffID, from, to)
	return values, err
}

func (s *service) listStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) ([]*appointmentcap.Appointment, error) {
	values, err := s.cfg.Appointments.ListStaffCancellationTombstones(ctx, staffID, since)
	return values, err
}

func (s *service) listAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to appointmentcap.Date) ([]*appointmentcap.Appointment, error) {
	values, err := s.cfg.Appointments.ListAppointmentsVisibleToGuardians(ctx, guardianIDs, studentIDs, from, to)
	return values, err
}

func (s *service) listGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) ([]*appointmentcap.Appointment, error) {
	values, err := s.cfg.Appointments.ListGuardianCancellationTombstones(ctx, guardianIDs, studentIDs, since)
	return values, err
}

// appointmentDetail reloads the full detail (recurrence, recipients, targets)
// for an appointment. Used by the lifecycle operations so callers (and the
// notification layer in Phase B) get the same shape as CreateStaffAppointment.
func (s *service) appointmentDetail(ctx context.Context, appointment *appointmentcap.Appointment) (*AppointmentDetail, error) {
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
	if s.cfg.MissingRecord(err) {
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
	appointment.StartTime = normalizeWallClock(req.StartTime)
	appointment.EndTime = normalizeWallClock(req.EndTime)
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

	updated, err := s.cfg.Appointments.UpdateAppointment(ctx, appointmentcap.UpdateAppointment{
		ID: appointment.ID, AppointmentFields: appointmentCapabilityFields(appointment),
	})
	if err != nil {
		// A concurrent cancel/delete transitioned the appointment between load and
		// write, so the conditional update matched nothing. Abort before touching
		// recurrence/overrides or sending an "updated" notice; the tenant tx rolls
		// back. Surface a conflict rather than a bogus success.
		if errors.Is(err, appointmentcap.ErrAppointmentLifecycleConflict) {
			return nil, fmt.Errorf("%w: appointment was cancelled or deleted", ErrConflict)
		}
		return nil, err
	}
	appointment = updated
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
		if err := s.notifyGuardians(ctx, appointment, ports.EmailKindAppointmentUpdated); err != nil {
			return nil, err
		}
		s.notifyGuardianDevices(ctx, appointment, ports.EmailKindAppointmentUpdated)
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
	if s.cfg.MissingRecord(err) {
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
			if err := s.notifyGuardians(ctx, appointment, ports.EmailKindAppointmentCancelled); err != nil {
				return nil, err
			}
			s.notifyGuardianDevices(ctx, appointment, ports.EmailKindAppointmentCancelled)
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
	if s.cfg.MissingRecord(err) {
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

func (s *service) CancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate appointmentcap.Date) error {
	return s.withinAppointmentWrite(ctx, func(txCtx context.Context) error {
		return s.cancelStaffAppointmentOccurrence(txCtx, appointmentID, occurrenceDate)
	})
}

func (s *service) cancelStaffAppointmentOccurrence(ctx context.Context, appointmentID int64, occurrenceDate appointmentcap.Date) error {
	appointment, err := s.loadOrganizedAppointment(ctx, appointmentID)
	if err != nil {
		return err
	}
	appointment, err = s.findAppointmentForUpdate(ctx, appointment.ID)
	if s.cfg.MissingRecord(err) {
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
	existing, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, []int64{appointment.ID}, []appointmentcap.Date{toCalendarDate(occurrenceDate)})
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
		err := s.cfg.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
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
	if status != appointmentcap.ResponseStatusAccepted && status != appointmentcap.ResponseStatusDeclined {
		return fmt.Errorf("%w: response status must be accepted or declined", ErrInvalidRequest)
	}

	recipient, err := s.cfg.Appointments.FindAppointmentRecipient(ctx, recipientID)
	if err != nil {
		return err
	}
	if recipient == nil || recipient.StaffID == nil || *recipient.StaffID != staff.ID {
		return ErrNotFound
	}
	if recipient.Status == appointmentcap.ResponseStatusInfo {
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
		allowedProfiles := int64Set(distinctGuardianProfileIDs(tenantChildren))
		allowedStudentIDs := int64Set(distinctChildStudentIDs(tenantChildren))
		var updated bool
		if err := s.cfg.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
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
			if recipient.Status == appointmentcap.ResponseStatusInfo {
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

	groups, err := s.cfg.GroupRepo.List(ctx)
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
		name := studentDisplayName(row)
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

func validateWindow(from, to appointmentcap.Date) error {
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
func (s *service) recipientsByAppointment(ctx context.Context, appointmentIDs []int64) (map[int64][]*appointmentcap.AppointmentRecipient, error) {
	recipients, err := s.cfg.Appointments.FindAppointmentRecipientsByAppointmentIDs(ctx, appointmentIDs)
	if err != nil {
		return nil, err
	}
	byAppointment := make(map[int64][]*appointmentcap.AppointmentRecipient, len(appointmentIDs))
	for _, recipient := range recipients {
		byAppointment[recipient.AppointmentID] = append(byAppointment[recipient.AppointmentID], recipient)
	}
	return byAppointment, nil
}

func (s *service) expandAppointmentEvents(ctx context.Context, appointments []*appointmentcap.Appointment, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.Appointments.FindRecurrenceRules(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*appointmentcap.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, from, to)
	overrides, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, ids, toCalendarDates(occurrenceDates))
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*appointmentcap.AppointmentOccurrenceOverride, len(overrides))
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

func (s *service) expandGuardianAppointmentEvents(ctx context.Context, appointments []*appointmentcap.Appointment, guardianProfileIDs []int64, studentIDs []int64, from, to appointmentcap.Date) ([]Event, error) {
	ids := make([]int64, 0, len(appointments))
	for _, appointment := range appointments {
		ids = append(ids, appointment.ID)
	}
	recurrences, err := s.cfg.Appointments.FindRecurrenceRules(ctx, ids)
	if err != nil {
		return nil, err
	}
	recurrenceByAppointment := make(map[int64]*appointmentcap.RecurrenceRule, len(recurrences))
	for _, recurrence := range recurrences {
		recurrenceByAppointment[recurrence.AppointmentID] = recurrence
	}
	occurrenceDates := occurrenceDatesForAppointments(appointments, recurrenceByAppointment, from, to)
	overrides, err := s.cfg.Appointments.FindOccurrenceOverrides(ctx, ids, toCalendarDates(occurrenceDates))
	if err != nil {
		return nil, err
	}
	overrideByAppointmentDate := make(map[string]*appointmentcap.AppointmentOccurrenceOverride, len(overrides))
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
	instance   *ports.ActivityInstance
	assignment *ports.InstanceStaff
}

// timetableRoomID resolves the room the staff member is actually in: the
// per-person override on instance_staff wins over the instance's own room
// (the Lernzeit split pattern), matching the Dienstplan overview in
// services/schedule/staff_schedule_overview.go.
func timetableRoomID(instance *ports.ActivityInstance, assignment *ports.InstanceStaff) int64 {
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
func (s *service) collectStaffTimetableAssignments(ctx context.Context, staffID int64, from, to appointmentcap.Date, includeCancelled bool) ([]staffTimetableAssignment, error) {
	assignments, err := s.cfg.InstanceStaffRepo.FindByStaffAndDateRange(ctx, staffID, appointmentcap.Date(from), appointmentcap.Date(to))
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
	instancesByID := make(map[int64]*ports.ActivityInstance, len(instances))
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
		if instance == nil || (!includeCancelled && instance.Status == "cancelled") {
			continue
		}
		collected = append(collected, staffTimetableAssignment{instance: instance, assignment: assignment})
	}
	return collected, nil
}

func (s *service) staffTimetableEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffTimetableEventsWithCancelled(ctx, staffID, from, to, false)
}

func (s *service) staffTimetableFeedEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffTimetableEventsWithCancelled(ctx, staffID, from, to, true)
}

func (s *service) staffTimetableEventsWithCancelled(ctx context.Context, staffID int64, from, to appointmentcap.Date, includeCancelled bool) ([]Event, error) {
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
			Cancelled:   instance.Status == "cancelled",
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
func shiftsReferenceTypes(shifts []*ports.StaffShift) bool {
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
func (s *service) staffShiftEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffShiftEventsWithCancelled(ctx, staffID, from, to, false)
}

func (s *service) staffShiftFeedEvents(ctx context.Context, staffID int64, from, to appointmentcap.Date) ([]Event, error) {
	return s.staffShiftEventsWithCancelled(ctx, staffID, from, to, true)
}

func (s *service) staffShiftEventsWithCancelled(ctx context.Context, staffID int64, from, to appointmentcap.Date, includeCancelled bool) ([]Event, error) {
	if s.cfg.StaffShiftRepo == nil {
		return []Event{}, nil
	}
	shifts, err := s.cfg.StaffShiftRepo.FindByStaffAndDateRange(ctx, staffID, appointmentcap.Date(from), appointmentcap.Date(to))
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

func (s *service) resolveTargets(ctx context.Context, deliveryMode string, targets []AppointmentTarget) ([]appointmentcap.AppointmentRecipientFields, []*appointmentcap.AppointmentTarget, error) {
	status := appointmentcap.ResponseStatusPending
	if deliveryMode == appointmentcap.DeliveryModeInformational {
		status = appointmentcap.ResponseStatusInfo
	}
	readSet, err := s.loadTargetResolutionReadSet(ctx, targets)
	if err != nil {
		return nil, nil, err
	}

	staffIDs := map[int64]struct{}{}
	guardianStudents := map[int64]map[int64]struct{}{}
	targetRows := make([]*appointmentcap.AppointmentTarget, 0, len(targets))
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
			if link.PortalAccess {
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
		targetRows = append(targetRows, &appointmentcap.AppointmentTarget{
			TargetType:  target.Type,
			TargetID:    target.ID,
			TargetValue: target.Value,
		})
		switch target.Type {
		case appointmentcap.TargetTypeAllStaff:
			// Only invite staff who can actually use the calendar (active
			// account + calendar:own); unreachable staff would leave RSVP
			// appointments permanently pending and skew attendee counts.
			for staffID := range readSet.reachableStaff {
				staffIDs[staffID] = struct{}{}
			}
		case appointmentcap.TargetTypeStaff:
			if !readSet.reachableStaff[*target.ID] {
				return nil, nil, fmt.Errorf("%w: staff target is not available", ErrInvalidRequest)
			}
			staffIDs[*target.ID] = struct{}{}
		case appointmentcap.TargetTypeGuardianProfile:
			if !readSet.activeGuardians[*target.ID] {
				return nil, nil, fmt.Errorf("%w: guardian target is not available", ErrInvalidRequest)
			}
			visible := false
			for _, link := range readSet.linksByGuardian[*target.ID] {
				if link.PortalAccess {
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
		case appointmentcap.TargetTypeAllSchoolParents:
			// Every portal-active guardian of the school's ACTIVE students. Resolve
			// in bulk so a school-wide appointment stays a couple of queries, not one
			// per student. Filter to active students at the DB so pending or inactive
			// (e.g. former) families never receive school-wide appointmentcap.
			studentIDs := activeStudentIDs(readSet.allSchoolStudents)
			added, err := addStudentsGuardians(studentIDs)
			if err != nil {
				return nil, nil, err
			}
			if added == 0 {
				return nil, nil, fmt.Errorf("%w: no reachable guardians at this school", ErrInvalidRequest)
			}
		case appointmentcap.TargetTypeParentsByStudent:
			added, err := addStudentGuardians(*target.ID)
			if err != nil {
				return nil, nil, err
			}
			if added == 0 {
				return nil, nil, fmt.Errorf("%w: parent target has no reachable guardians", ErrInvalidRequest)
			}
		case appointmentcap.TargetTypeParentsByGroup:
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
		case appointmentcap.TargetTypeParentsByClass:
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

	recipients := make([]appointmentcap.AppointmentRecipientFields, 0, len(staffIDs)+len(guardianStudents))
	for staffID := range staffIDs {
		id := staffID
		recipients = append(recipients, appointmentcap.AppointmentRecipientFields{
			RecipientType: appointmentcap.RecipientTypeStaff,
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
		recipients = append(recipients, appointmentcap.AppointmentRecipientFields{
			RecipientType:     appointmentcap.RecipientTypeGuardianProfile,
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
func activeStudentIDs(students []*ports.Student) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if student.Status == "active" {
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
		if link.PortalAccess {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) buildAppointmentOverview(ctx context.Context, appointment *appointmentcap.Appointment, recipients []*appointmentcap.AppointmentRecipient) (*AppointmentOverview, error) {
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

func canStaffViewOverview(appointment *appointmentcap.Appointment, staffID int64, isRecipient bool) bool {
	if appointment == nil {
		return false
	}
	if appointment.OrganizerStaffID == staffID {
		return true
	}
	switch appointment.OverviewVisibility {
	case appointmentcap.OverviewVisibilityStaff, appointmentcap.OverviewVisibilityAll:
		return isRecipient
	default:
		return false
	}
}

func canParentViewOverview(appointment *appointmentcap.Appointment, isRecipient bool) bool {
	return appointment != nil && isRecipient && appointment.OverviewVisibility == appointmentcap.OverviewVisibilityAll
}

func appointmentEvent(appointment *appointmentcap.Appointment, occurrenceDate appointmentcap.Date, responseStatus *string, recipientID *int64, staffID int64) Event {
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
		CanRespond:      appointment.CancelledAt == nil && recipientID != nil && responseStatus != nil && *responseStatus != appointmentcap.ResponseStatusInfo,
		CanEdit:         appointment.OrganizerStaffID == staffID,
		CanViewOverview: canStaffViewOverview(appointment, staffID, isStaffRecipient),
	}
}

func applyOverride(event *Event, override *appointmentcap.AppointmentOccurrenceOverride) {
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

func staffRecipientStatus(recipients []*appointmentcap.AppointmentRecipient, staffID int64) (*string, *int64) {
	for _, recipient := range recipients {
		if recipient.StaffID != nil && *recipient.StaffID == staffID {
			status := recipient.Status
			id := recipient.ID
			return &status, &id
		}
	}
	return nil, nil
}

func (s *service) guardianRecipientStatusForStudents(ctx context.Context, recipients []*appointmentcap.AppointmentRecipient, guardianProfileIDs []int64, studentIDs []int64) (*string, *int64, error) {
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

func groupChildrenByTenant(children []*ports.ChildSummary) map[int64][]*ports.ChildSummary {
	out := make(map[int64][]*ports.ChildSummary)
	for _, child := range children {
		out[child.TenantID] = append(out[child.TenantID], child)
	}
	return out
}

func distinctGuardianProfileIDs(children []*ports.ChildSummary) []int64 {
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

func distinctChildStudentIDs(children []*ports.ChildSummary) []int64 {
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

func occurrenceDatesForAppointments(appointments []*appointmentcap.Appointment, recurrenceByAppointment map[int64]*appointmentcap.RecurrenceRule, from, to appointmentcap.Date) []appointmentcap.Date {
	seen := map[string]struct{}{}
	dates := []appointmentcap.Date{}
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

func recurrenceRuleFromRequest(req *RecurrenceRequest) *appointmentcap.RecurrenceRule {
	if req == nil {
		return nil
	}
	rule := &appointmentcap.RecurrenceRule{
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

func dateRangesOverlap(aStart, aEnd, bStart, bEnd appointmentcap.Date) bool {
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
	return normalizeWallClock(t).Format("15:04")
}

func staffName(staff *ports.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName)
}

func guardianName(parent *ports.GuardianProfile) string {
	if parent == nil {
		return ""
	}
	name := strings.TrimSpace(parent.FirstName + " " + parent.LastName)
	if name == "" && parent.Email != nil {
		return *parent.Email
	}
	return name
}

func studentDisplayName(student *ports.Student) string {
	if student == nil {
		return ""
	}
	if student.Person == nil {
		return fmt.Sprintf("Kind %d", student.ID)
	}
	return strings.TrimSpace(student.Person.FirstName + " " + student.Person.LastName)
}
