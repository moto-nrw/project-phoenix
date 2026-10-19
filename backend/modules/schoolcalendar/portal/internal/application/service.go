package application

import (
	"context"
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
