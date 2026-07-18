package checkin

import (
	"cmp"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	checkinSvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the Check-in API resource for student RFID check-in/checkout
type Resource struct {
	IoTService            iotSvc.Service
	UsersService          usersSvc.PersonService
	ActiveService         activeSvc.Service
	PickupScheduleService scheduleSvc.PickupScheduleService
	SettingsService       configSvc.SettingsService
	UnregisteredTagScans  auditSvc.UnregisteredTagScanService
	// Checkin holds the extracted RFID check-in business logic (issue #575 B8).
	Checkin *checkinSvc.CheckinService
	logger  *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.logger, slog.Default())
}

// NewResource creates a new Check-in resource
func NewResource(
	iotService iotSvc.Service,
	usersService usersSvc.PersonService,
	activeService activeSvc.Service,
	checkinService *checkinSvc.CheckinService,
	pickupScheduleService scheduleSvc.PickupScheduleService,
	settingsService configSvc.SettingsService,
	logger *slog.Logger,
	unregisteredTagScans ...auditSvc.UnregisteredTagScanService,
) *Resource {
	var scanService auditSvc.UnregisteredTagScanService
	if len(unregisteredTagScans) > 0 {
		scanService = unregisteredTagScans[0]
	}
	return &Resource{
		IoTService:            iotService,
		UsersService:          usersService,
		ActiveService:         activeService,
		Checkin:               checkinService,
		PickupScheduleService: pickupScheduleService,
		SettingsService:       settingsService,
		UnregisteredTagScans:  scanService,
		logger:                logger,
	}
}

// Router returns a configured router for check-in endpoints
// This router handles student RFID check-in/checkout workflow
// All routes require device authentication (API key + Staff PIN)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Check-in workflow endpoints
	r.Post("/checkin", rs.deviceCheckin)
	r.Post("/pickup-query", rs.devicePickupQuery)
	r.Post("/ping", rs.devicePing)
	r.Get("/status", rs.deviceStatus)

	return r
}
