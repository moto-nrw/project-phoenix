package compose

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// delegateHandler creates an http.HandlerFunc that delegates to a subrouter.
// This avoids Chi's "Mount() on existing path" error while keeping routes organized.
func delegateHandler(router chi.Router) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		router.ServeHTTP(w, req)
	}
}

// ServiceDependencies groups all service dependencies for the IoT resource
type ServiceDependencies struct {
	Administration Administration
	// DeviceScan is the public device-scan workflow the kiosk scans, pickup
	// queries, heartbeats and attendance toggles go through (#2698).
	DeviceScan devicescan.DeviceScan
	// OpenRooms is the destination booking into released rooms behind
	// POST /move-to-room (#3067). Nil leaves the route unmounted.
	OpenRooms devicescan.OpenRoomBooking
	// StaffClock is the public device-scan staff clock the kiosk stamps
	// through (#2690).
	StaffClock               devicescan.StaffClock
	Configuration            devicescan.ConfigurationQuery
	Rooms                    devicescan.RoomAvailability
	Directory                devicescan.Directory
	TagAssignments           devicescan.TagAssignments
	FeedbackStudents         devicescan.FeedbackStudents
	FeedbackService          Feedback
	FeedbackResponseObserver func(int, string)
	SchoolName               devicescan.SchoolNameQuery
	// ErrorReports forwards kiosk Sentry envelopes (#3645); nil when the
	// backend runs without Sentry.
	ErrorReports ErrorReports
	// SessionEnd is the application workflow behind POST /session/end
	// (#2697): one UnitOfWork over the Presence and Timetable commands.
	SessionEnd       EndSession
	SessionLifecycle devicescan.SessionLifecycle
	Logger           *slog.Logger
	Runtime          RouterRuntime

	// DeviceAuthenticator and DeviceOnlyAuthenticator guard the kiosk route
	// groups. The Device Fleet composition builds them over one shared
	// last-seen debouncer; this resource only mounts them.
	DeviceAuthenticator     Middleware
	DeviceOnlyAuthenticator Middleware
}

// Resource defines the IoT API resource
type Resource struct {
	ServiceDependencies
}

// NewResource creates a new IoT resource
func NewResource(deps ServiceDependencies) *Resource {
	return &Resource{ServiceDependencies: deps}
}

// getLogger returns the resource's logger, falling back to slog.Default() if nil.
func (rs *Resource) getLogger() *slog.Logger {
	if rs.Logger == nil {
		return slog.Default()
	}
	return rs.Logger
}

// Router returns a configured router for IoT endpoints
func (rs *Resource) Router() chi.Router {
	if rs.FeedbackStudents == nil || rs.FeedbackService == nil || rs.FeedbackResponseObserver == nil || rs.Runtime.Feedback == nil {
		panic("IoT feedback: all dependencies are required")
	}
	if rs.Runtime.Protected == nil || rs.Runtime.Required == nil || rs.Runtime.Metrics == nil || rs.Runtime.TenantTx == nil || rs.Runtime.Info == nil || rs.Runtime.Data == nil || rs.Runtime.Checkin == nil || rs.Runtime.StaffClock == nil || rs.Runtime.Attendance == nil || rs.Runtime.Session == nil || rs.Runtime.RFID == nil || rs.ErrorReports == nil || (rs.OpenRooms != nil && rs.Runtime.OpenRooms == nil) {
		panic("IoT HTTP: route runtime is required")
	}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	rs.Runtime.Protected(r, func(r chi.Router, withTx Middleware) {

		// Mount devices sub-router (handles device CRUD and admin operations)
		// All device routes require JWT authentication with IOT permissions
		if rs.Administration != nil {
			rs.Administration(r, withTx)
		}
	})

	// Device-only authenticated routes (API key only, no PIN required)
	// DeviceOnlyAuthenticator sets tenant context from device.TenantID,
	// then TenantTxMiddleware wraps the handler in a tenant-scoped transaction
	// so downstream queries run as phoenix_tenant with RLS enforced.
	r.Group(func(r chi.Router) {
		r.Use(rs.Runtime.Required("DeviceOnlyAuthenticator", rs.DeviceOnlyAuthenticator))
		r.Use(rs.Runtime.Metrics)
		r.Use(rs.Runtime.TenantTx)

		// Mount data sub-router for teachers endpoint (device-only auth)
		dataResource := rs.Runtime.Data(rs.Directory, rs.TagAssignments, rs.Rooms)
		r.Mount("/teachers", dataResource.TeachersRouter())

		// School name endpoint (device API key → school name)
		info := rs.Runtime.Info(rs.Configuration, rs.SchoolName)
		r.Get("/school-name", info.Router().ServeHTTP)

		// Device configuration endpoint (checkout buttons, feedback settings)
		r.Get("/config", info.Router().ServeHTTP)
	})

	// Sentry tunnel of the kiosks: API key only, like the group above, but
	// without a tenant transaction. The relay reads no table and must not
	// hold a database connection while it waits for Sentry.
	r.Group(func(r chi.Router) {
		r.Use(rs.Runtime.Required("DeviceOnlyAuthenticator", rs.DeviceOnlyAuthenticator))
		r.Use(rs.Runtime.Metrics)

		errorReports := rs.ErrorReports(rs.getLogger().With(slog.String("sub", "error-reports")))
		r.Post("/error-reports", errorReports.Post)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates the device credentials and, when supplied,
	// binds staff identity to a verified account PIN. TenantTxMiddleware then
	// wraps each handler in a tenant-scoped transaction.
	r.Group(func(r chi.Router) {
		r.Use(rs.Runtime.Required("DeviceAuthenticator", rs.DeviceAuthenticator))
		r.Use(rs.Runtime.Metrics)
		r.Use(rs.Runtime.TenantTx)

		// Feedback endpoint (device-based feedback submission)
		feedbackResource := rs.Runtime.Feedback(rs.FeedbackStudents, rs.FeedbackService, rs.FeedbackResponseObserver, rs.getLogger().With(slog.String("sub", "feedback")))
		r.Post("/feedback", delegateHandler(feedbackResource.Router()))

		// Check-in endpoints (student RFID check-in/checkout workflow)
		checkinResource := rs.Runtime.Checkin(rs.DeviceScan, rs.getLogger().With(slog.String("sub", "checkin")))
		// Register routes directly instead of mounting at "/" to avoid Chi conflict
		checkinHandler := delegateHandler(checkinResource.Router())
		r.Post("/checkin", checkinHandler)
		r.Post("/pickup-query", checkinHandler)
		r.Post("/ping", checkinHandler)
		r.Get("/status", checkinHandler)

		// Destination booking into a released room, chosen at the device
		// the child leaves (#3067).
		if rs.OpenRooms != nil {
			openRoomResource := rs.Runtime.OpenRooms(rs.OpenRooms, rs.getLogger().With(slog.String("sub", "open-room")))
			r.Post("/move-to-room", delegateHandler(openRoomResource.Router()))
		}

		// Pure staff time tracking, independent of activities or groups.
		staffClockResource := rs.Runtime.StaffClock(rs.StaffClock)
		staffClockHandler := delegateHandler(staffClockResource.Router())
		r.Post("/staff-clock", staffClockHandler)
		r.Post("/staff-clock/state", staffClockHandler)

		// Data query endpoints (device + PIN auth)
		dataResourceAuth := rs.Runtime.Data(rs.Directory, rs.TagAssignments, rs.Rooms)
		dataHandler := delegateHandler(dataResourceAuth.Router())
		r.Get("/students", dataHandler)
		r.Get("/activities", dataHandler)
		r.Get("/rooms/available", dataHandler)
		r.Get("/rfid/{tagId}", dataHandler)

		// Mount attendance sub-router (handles daily attendance tracking)
		attendanceResource := rs.Runtime.Attendance(rs.DeviceScan, rs.getLogger().With(slog.String("sub", "attendance")))
		r.Mount("/attendance", attendanceResource.Router())

		// Mount sessions sub-router (handles activity session management and timeout)
		sessionsResource := rs.Runtime.Session(rs.SessionLifecycle, rs.SessionEnd)
		r.Mount("/session", sessionsResource.Router())

		// Mount RFID sub-router (handles RFID tag assignment/unassignment for staff)
		rfidResource := rs.Runtime.RFID(rs.TagAssignments)
		r.Mount("/staff", rfidResource.Router())
	})

	return r
}
