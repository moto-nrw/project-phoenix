package students

import (
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/realtime"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	activityService "github.com/moto-nrw/project-phoenix/services/activities"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource defines the students API resource
type Resource struct {
	ResourceConfig
}

// ResourceConfig holds all dependencies for creating a students Resource.
// Using a config struct instead of individual parameters improves maintainability.
type ResourceConfig struct {
	PersonService          userService.PersonService
	GuardianService        *userService.GuardianService
	EducationService       educationService.Service
	UserContextService     userContextService.UserContextService
	ActiveService          activeService.Service
	IoTService             iotSvc.Service
	StaffPINAuthenticator  authService.StaffPINAuthenticator
	PickupScheduleService  scheduleService.PickupScheduleService
	ArrivalScheduleService scheduleService.ArrivalScheduleService
	InstanceService        scheduleService.InstanceService
	// CareDayService gates the day-planning timetable signal on the child's
	// care plan (#1747) — without it a child assigned to a block counts as
	// "kommt heute" on every weekday, including the ones they are not booked
	// for. Optional: nil keeps the unfiltered pre-#1747 behaviour, which is
	// what bare test Resources rely on.
	CareDayService          scheduleService.CareDayService
	SchoolService           platformSvc.SchoolService
	SettingsService         configService.SettingsService
	StudentService          userService.StudentService
	StudentAuditService     userService.StudentAuditService
	MasterDataReviewService userService.MasterDataReviewService
	CareRequestService      scheduleService.CareScheduleRequestService
	ExcusedRequestService   absenceService.ExcusedAbsenceRequestService
	StudentStatusDayService *activeService.StudentStatusDayService
	StudentHistoryService   activeService.StudentHistoryService
	ActivityService         activityService.ActivityService
	EnrollmentDecision      enrollmentService.DecisionService
	EnrollmentFormSchema    enrollmentService.FormSchemaService
	Broadcaster             realtime.Broadcaster
	// ParentEventEmitter wakes a child's guardians (message-independent
	// parent_child_updated SSE fan-out) after staff-side care writes, so an open
	// parents-app tab refetches the child's care state live (#1725). Optional —
	// nil is a no-op (the guardian helper guards on it), so tests that build a
	// bare Resource keep working.
	ParentEventEmitter *parentmessaging.Emitter
	StudentPhotos      userService.StudentPhotoService
	ListExportService  *listexport.RendererService
	Logger             *slog.Logger
	Now                func() time.Time
	DB                 *bun.DB
}

// NewResource creates a new students resource from the provided configuration.
func NewResource(cfg ResourceConfig) *Resource {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Resource{ResourceConfig: cfg}
}

// Router returns a configured router for student endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {

		// Routes requiring users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listStudents)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/school-classes", rs.listSchoolClasses)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Post("/export", rs.exportStudents)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getStudent)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/in-group-room", rs.getStudentInGroupRoom)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-location", rs.getStudentCurrentLocation)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/current-visit", rs.getStudentCurrentVisit)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/visit-history", rs.getStudentVisitHistory)
		// Group day log ("Tagesauswertung", #1456). Static paths take
		// precedence over /{id} in chi; gate + scope enforced in the handlers.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/day-log", rs.getStudentsDayLog)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/day-log/export", rs.exportStudentsDayLog)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history", rs.getStudentAttendanceHistory)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/attendance-history/export", rs.exportStudentAttendanceHistory)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/status-days", rs.getStudentStatusDays)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/enrollment-extra-fields", rs.getStudentEnrollmentExtraFields)
		// Per-child change history (issue #1455). Full access (admin / group
		// supervisor) is enforced inside the handler.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/change-history", rs.getStudentChangeHistory)

		// Parent Stammdaten change-request review queue (Track B). Requests can
		// contain parent-submitted name, birthday, and departure-plan changes.
		// Gated on users:update — the same permission as editing a child directly
		// (PUT /{id}) — because deciding a request is that same write. The service
		// additionally scopes both the list and the decision per child (admin or
		// the child's group supervisor), so a supervisor sees and decides only
		// their own group's requests. Static paths take precedence over the /{id}
		// param route in chi.
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Get("/master-data-change-requests", rs.listMasterDataChangeRequests)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/master-data-change-requests/{requestId}/decide", rs.decideMasterDataChangeRequest)

		// Parent care-schedule change-request review queue (#1803). Decisions
		// rewrite the child's permanent weekly plan, so they share the users:update
		// gate + per-child write scope of the master-data queue — both are decided
		// on the same Änderungsanfragen page.
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Get("/care-schedule-change-requests", rs.listCareScheduleChangeRequests)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/care-schedule-change-requests/{requestId}/decide", rs.decideCareScheduleChangeRequest)

		// Excused-absence approval requests (#1845): staff review queue.
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Get("/excused-absence-requests", rs.listExcusedAbsenceRequests)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/excused-absence-requests/{requestId}/decide", rs.decideExcusedAbsenceRequest)

		// Combined pending count across both review queues, driving the
		// Änderungsanfragen sidebar badge. Same users:update gate + per-child
		// scope as the queues it summarizes (the count sums their scoped lists).
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Get("/change-requests/pending-count", rs.pendingChangeRequestCount)

		// Routes requiring users:create permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStudent)

		// Routes requiring users:update permission
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateStudent)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/status-days/bulk", rs.bulkCreateStudentStatusDays)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/status-days", rs.createStudentStatusDays)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/status-days/{statusDayId}", rs.deleteStudentStatusDay)

		// Routes requiring users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStudent)

		// Privacy consent routes
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/privacy-consent", rs.getStudentPrivacyConsent)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/privacy-consent", rs.updateStudentPrivacyConsent)

		// Departure companions ("läuft mit" / Laufgemeinschaft), read-only.
		// There is deliberately no PUT here: a link is only legal on a weekday
		// the child's departure plan allows "Anderes Kind", so writing it
		// separately from that plan would allow the two to contradict each
		// other. Companions are written through PUT /students/{id}.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/companions", rs.getStudentCompanions)

		// Pickup schedule routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/pickup-schedules", rs.getStudentPickupSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-schedules", rs.updateStudentPickupSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-exceptions", rs.createStudentPickupException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-exceptions/{exceptionId}", rs.updateStudentPickupException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/pickup-exceptions/{exceptionId}", rs.deleteStudentPickupException)

		// Pickup note routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/pickup-notes", rs.createStudentPickupNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/pickup-notes/{noteId}", rs.updateStudentPickupNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/pickup-notes/{noteId}", rs.deleteStudentPickupNote)

		// Bulk pickup times endpoint (returns pickup times for multiple students)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Post("/pickup-times/bulk", rs.getBulkPickupTimes)

		// Arrival schedule routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/arrival-schedules", rs.getStudentArrivalSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-schedules", rs.updateStudentArrivalSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-exceptions", rs.createStudentArrivalException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-exceptions/{exceptionId}", rs.updateStudentArrivalException)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-exceptions/{exceptionId}", rs.deleteStudentArrivalException)

		// Arrival note routes (full access required - checked in handlers)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{id}/arrival-notes", rs.createStudentArrivalNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/arrival-notes/{noteId}", rs.updateStudentArrivalNote)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/arrival-notes/{noteId}", rs.deleteStudentArrivalNote)

		// Bulk arrival schedule and time endpoints
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/arrival-schedules/bulk", rs.bulkUpsertArrivalSchedules)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Post("/arrival-times/bulk", rs.getBulkArrivalTimes)

		// Web-based school check-in/out. Mode-agnostic (writes attendance only).
		// The users:checkin permission is the coarse gate; the
		// attendance.web_checkin_access setting is the fine gate enforced inside
		// the handler (group_supervisors vs all_staff).
		r.With(authorize.RequiresPermission(permissions.UsersCheckin), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).Post("/{id}/school-checkin", rs.schoolCheckinHandler)

		// Student photo (Datenverwaltung). upload + delete: users:update;
		// serve: users:read. Feature gate + consent enforced in photo.go.
		// upload + serve skip withTx so a slow body / file stream doesn't
		// pin a bun pool connection. The handlers open their own short tx.
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Post("/{id}/photo", rs.uploadStudentPhoto)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/photo", rs.deleteStudentPhoto)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/photo/{filename}", rs.serveStudentPhoto)
	})

	// Device-authenticated routes for RFID devices.
	// DeviceAuthenticator validates API key + PIN and sets tenant context,
	// then TenantTxMiddleware wraps each handler in a tenant-scoped transaction
	// (SET LOCAL ROLE phoenix_tenant + set_config) so RLS is enforced.
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.SchoolService, rs.StaffPINAuthenticator, nil))
		r.Use(tenant.TenantTxMiddleware(rs.DB))

		// RFID tag assignment endpoint
		r.Post("/{id}/rfid", rs.assignRFIDTag)
		r.Delete("/{id}/rfid", rs.unassignRFIDTag)
	})

	return r
}
