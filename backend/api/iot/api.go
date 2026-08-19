package iot

import (
	"cmp"
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	sessionsAPI "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	staffclockAPI "github.com/moto-nrw/project-phoenix/api/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	checkinSvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
	staffclockSvc "github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	IoTService            iotSvc.Service
	StaffPINAuthenticator authSvc.StaffPINAuthenticator
	CheckinService        *checkinSvc.CheckinService
	StaffClockService     *staffclockSvc.Service
	UsersService          usersSvc.PersonService
	ActiveService         activeSvc.Service
	ActivitiesService     activitiesSvc.ActivityService
	SettingsService       configSvc.SettingsService
	FacilityService       facilitiesSvc.Service
	EducationService      educationSvc.Service
	PickupScheduleService scheduleSvc.PickupScheduleService
	SchoolService         platformSvc.SchoolService
	TimetableDataService  *scheduleSvc.TimetableDataService
	TimetableBridge       *scheduleSvc.TimetableBridgeService
	UnregisteredTagScans  auditSvc.UnregisteredTagScanService
	Broadcaster           realtime.Broadcaster
	Logger                *slog.Logger
	DB                    *bun.DB
}

// Resource defines the IoT API resource
type Resource struct {
	ServiceDependencies
}

// NewResource creates a new IoT resource
func NewResource(deps ServiceDependencies) *Resource {
	return &Resource{ServiceDependencies: deps}
}

// pinResolver returns a PINResolver that reads from the settings service.
// Returns nil if no settings service is available (falls back to env var in device auth).
func (rs *Resource) pinResolver() device.PINResolver {
	if rs.SettingsService == nil {
		return nil
	}
	return func(ctx context.Context, tenantID int64) string {
		pin, err := rs.SettingsService.ResolveStringForTenant(ctx, tenantID, configModel.KeyOGSDevicePIN)
		if err != nil {
			slog.Error("failed to resolve tenant PIN from settings, falling back to env var",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return ""
		}
		return pin
	}
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
		devicesResource := NewDevicesResource(rs.IoTService)
		r.With(withTx).Mount("/", devicesResource.Router())
	})

	// Device-only authenticated routes (API key only, no PIN required)
	// DeviceOnlyAuthenticator sets tenant context from device.TenantID,
	// then TenantTxMiddleware wraps the handler in a tenant-scoped transaction
	// so downstream queries run as phoenix_tenant with RLS enforced.
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceOnlyAuthenticator(rs.IoTService, rs.SchoolService))
		r.Use(iotMetricsMiddleware)
		r.Use(tenant.TenantTxMiddleware(rs.DB))

		// Mount data sub-router for teachers endpoint (device-only auth)
		dataResource := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService, rs.UnregisteredTagScans)
		r.Mount("/teachers", dataResource.TeachersRouter())

		// School name endpoint (device API key → school name)
		r.Get("/school-name", rs.getSchoolName)

		// Device configuration endpoint (checkout buttons, presence mode)
		r.Get("/config", rs.getDeviceConfig)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates the device credentials and, when supplied,
	// binds staff identity to a verified account PIN. TenantTxMiddleware then
	// wraps each handler in a tenant-scoped transaction.
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(
			rs.IoTService,
			rs.SchoolService,
			rs.StaffPINAuthenticator,
			rs.pinResolver(),
		))
		r.Use(iotMetricsMiddleware)
		r.Use(tenant.TenantTxMiddleware(rs.DB))

		// Check-in endpoints (student RFID check-in/checkout workflow)
		checkinResource := checkinAPI.NewResource(
			rs.IoTService,
			rs.UsersService,
			rs.ActiveService,
			rs.CheckinService,
			rs.PickupScheduleService,
			rs.SettingsService,
			rs.getLogger().With(slog.String("sub", "checkin")),
			rs.UnregisteredTagScans,
		)
		// Register routes directly instead of mounting at "/" to avoid Chi conflict
		checkinHandler := delegateHandler(checkinResource.Router())
		r.Post("/checkin", checkinHandler)
		r.Post("/pickup-query", checkinHandler)
		r.Post("/ping", checkinHandler)
		r.Get("/status", checkinHandler)

		// Pure staff time tracking, independent of activities or groups.
		staffClockResource := staffclockAPI.NewResource(rs.StaffClockService)
		staffClockHandler := delegateHandler(staffClockResource.Router())
		r.Post("/staff-clock", staffClockHandler)
		r.Post("/staff-clock/state", staffClockHandler)

		// Data query endpoints (device + PIN auth)
		dataResourceAuth := dataAPI.NewResource(rs.IoTService, rs.UsersService, rs.ActivitiesService, rs.FacilityService, rs.UnregisteredTagScans)
		dataHandler := delegateHandler(dataResourceAuth.Router())
		r.Get("/students", dataHandler)
		r.Get("/activities", dataHandler)
		r.Get("/rooms/available", dataHandler)
		r.Get("/rfid/{tagId}", dataHandler)

		// Mount attendance sub-router (handles daily attendance tracking)
		attendanceResource := checkinAPI.NewAttendanceResource(rs.UsersService, rs.ActiveService, rs.EducationService, rs.SettingsService, rs.UnregisteredTagScans)
		r.Mount("/attendance", attendanceResource.Router())

		// Mount sessions sub-router (handles activity session management and timeout)
		sessionsResource := sessionsAPI.NewResource(
			rs.IoTService,
			rs.UsersService,
			rs.ActiveService,
			rs.ActivitiesService,
			rs.FacilityService,
			rs.EducationService,
		)
		sessionsResource.ConfigureTimetableMirror(
			rs.TimetableDataService,
			rs.TimetableBridge,
			rs.Broadcaster,
		)
		r.Mount("/session", sessionsResource.Router())

		// Mount RFID sub-router (handles RFID tag assignment/unassignment for staff)
		rfidResource := dataAPI.NewRFIDResource(rs.UsersService)
		r.Mount("/staff", rfidResource.Router())
	})

	return r
}

// schoolNameResponse is the payload for GET /school-name.
type schoolNameResponse struct {
	Name string `json:"name"`
}

// getSchoolName returns the school name for the authenticated device.
func (rs *Resource) getSchoolName(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			slog.Error("failed to render device auth error", slog.String("error", err.Error()))
		}
		return
	}

	school, err := rs.SchoolService.GetSchoolByID(r.Context(), deviceCtx.TenantID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, schoolNameResponse{Name: school.Name}, "School name retrieved")
}
