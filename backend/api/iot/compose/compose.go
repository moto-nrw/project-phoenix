package compose

import (
	"cmp"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot"
	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	devicesAPI "github.com/moto-nrw/project-phoenix/api/iot/devices"
	sessionsAPI "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	staffclockAPI "github.com/moto-nrw/project-phoenix/api/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	"github.com/uptrace/bun"
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
	Administration devicefleet.Administration
	// DeviceScan is the public device-scan workflow the kiosk scans, pickup
	// queries, heartbeats and attendance toggles go through (#2698).
	DeviceScan devicescan.DeviceScan
	// StaffClock is the public device-scan staff clock the kiosk stamps
	// through (#2690).
	StaffClock               devicescan.StaffClock
	Configuration            devicescan.ConfigurationQuery
	Rooms                    devicescan.RoomAvailability
	Directory                devicescan.Directory
	TagAssignments           devicescan.TagAssignments
	FeedbackStudents         devicescan.FeedbackStudents
	FeedbackService          dataAPI.Feedback
	FeedbackResponseObserver func(int, string)
	SchoolName               devicescan.SchoolNameQuery
	// SessionEnd is the application workflow behind POST /session/end
	// (#2697): one UnitOfWork over the Presence and Timetable commands.
	SessionEnd       sessionend.Command
	SessionLifecycle devicescan.SessionLifecycle
	Logger           *slog.Logger
	DB               *bun.DB
	// DeviceAuthenticator and DeviceOnlyAuthenticator guard the kiosk route
	// groups. The Device Fleet composition builds them over one shared
	// last-seen debouncer; this resource only mounts them.
	DeviceAuthenticator     common.Middleware
	DeviceOnlyAuthenticator common.Middleware
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
	return cmp.Or(rs.Logger, slog.Default())
}

// Router returns a configured router for IoT endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {

		// Mount devices sub-router (handles device CRUD and admin operations)
		// All device routes require JWT authentication with IOT permissions
		devicesResource := devicesAPI.NewResource(rs.Administration, devicesRuntime())
		r.With(withTx).Mount("/", devicesResource.Router())
	})

	// Device-only authenticated routes (API key only, no PIN required)
	// DeviceOnlyAuthenticator sets tenant context from device.TenantID,
	// then TenantTxMiddleware wraps the handler in a tenant-scoped transaction
	// so downstream queries run as phoenix_tenant with RLS enforced.
	r.Group(func(r chi.Router) {
		r.Use(device.Required("DeviceOnlyAuthenticator", rs.DeviceOnlyAuthenticator))
		r.Use(iotMetricsMiddleware)
		r.Use(common.TenantTxMiddleware)

		// Mount data sub-router for teachers endpoint (device-only auth)
		dataResource := dataAPI.NewResource(rs.Directory, rs.TagAssignments, rs.Rooms, dataRuntime())
		r.Mount("/teachers", dataResource.TeachersRouter())

		// School name endpoint (device API key → school name)
		info := iotAPI.NewResource(rs.Configuration, rs.SchoolName, infoRuntime())
		r.Get("/school-name", info.Router().ServeHTTP)

		// Device configuration endpoint (checkout buttons, feedback settings)
		r.Get("/config", info.Router().ServeHTTP)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates the device credentials and, when supplied,
	// binds staff identity to a verified account PIN. TenantTxMiddleware then
	// wraps each handler in a tenant-scoped transaction.
	r.Group(func(r chi.Router) {
		r.Use(device.Required("DeviceAuthenticator", rs.DeviceAuthenticator))
		r.Use(iotMetricsMiddleware)
		r.Use(common.TenantTxMiddleware)

		// Feedback endpoint (device-based feedback submission)
		feedbackResource := dataAPI.NewFeedbackResource(rs.FeedbackStudents, rs.FeedbackService, rs.FeedbackResponseObserver, dataRuntime(), rs.getLogger().With(slog.String("sub", "feedback")))
		r.Post("/feedback", delegateHandler(feedbackResource.Router()))

		// Check-in endpoints (student RFID check-in/checkout workflow)
		checkinResource := checkinAPI.NewResource(rs.DeviceScan, checkinRuntime(), rs.getLogger().With(slog.String("sub", "checkin")))
		// Register routes directly instead of mounting at "/" to avoid Chi conflict
		checkinHandler := delegateHandler(checkinResource.Router())
		r.Post("/checkin", checkinHandler)
		r.Post("/pickup-query", checkinHandler)
		r.Post("/ping", checkinHandler)
		r.Get("/status", checkinHandler)

		// Pure staff time tracking, independent of activities or groups.
		staffClockResource := staffclockAPI.NewResource(rs.StaffClock, staffClockRuntime())
		staffClockHandler := delegateHandler(staffClockResource.Router())
		r.Post("/staff-clock", staffClockHandler)
		r.Post("/staff-clock/state", staffClockHandler)

		// Data query endpoints (device + PIN auth)
		dataResourceAuth := dataAPI.NewResource(rs.Directory, rs.TagAssignments, rs.Rooms, dataRuntime())
		dataHandler := delegateHandler(dataResourceAuth.Router())
		r.Get("/students", dataHandler)
		r.Get("/activities", dataHandler)
		r.Get("/rooms/available", dataHandler)
		r.Get("/rfid/{tagId}", dataHandler)

		// Mount attendance sub-router (handles daily attendance tracking)
		attendanceResource := checkinAPI.NewAttendanceResource(rs.DeviceScan, checkinRuntime(), rs.getLogger().With(slog.String("sub", "attendance")))
		r.Mount("/attendance", attendanceResource.Router())

		// Mount sessions sub-router (handles activity session management and timeout)
		sessionsResource := sessionsAPI.NewResource(rs.SessionLifecycle, rs.SessionEnd, sessionRuntime())
		r.Mount("/session", sessionsResource.Router())

		// Mount RFID sub-router (handles RFID tag assignment/unassignment for staff)
		rfidResource := dataAPI.NewRFIDResource(rs.TagAssignments, dataRuntime())
		r.Mount("/staff", rfidResource.Router())
	})

	return r
}
