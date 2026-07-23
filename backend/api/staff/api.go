package staff

import (
	"cmp"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the staff API resource
type Resource struct {
	PersonService           usersSvc.PersonService
	StaffOffboardingService usersSvc.StaffOffboardingService
	EducationService        educationSvc.Service
	AuthService             authSvc.AuthService
	WorkSessionService      activeSvc.WorkSessionService
	StaffAbsenceService     activeSvc.StaffAbsenceService
	WorkTimeMonthService    activeSvc.WorkTimeMonthService
	db                      *bun.DB
	logger                  *slog.Logger
}

// NewResource creates a new staff resource
func NewResource(
	personService usersSvc.PersonService,
	staffOffboardingService usersSvc.StaffOffboardingService,
	educationService educationSvc.Service,
	authService authSvc.AuthService,
	workSessionService activeSvc.WorkSessionService,
	staffAbsenceService activeSvc.StaffAbsenceService,
	workTimeMonthService activeSvc.WorkTimeMonthService,
	db *bun.DB,
	logger *slog.Logger,
) *Resource {
	return &Resource{
		PersonService:           personService,
		StaffOffboardingService: staffOffboardingService,
		EducationService:        educationService,
		AuthService:             authService,
		WorkSessionService:      workSessionService,
		StaffAbsenceService:     staffAbsenceService,
		WorkTimeMonthService:    workTimeMonthService,
		db:                      db,
		logger:                  logger,
	}
}

// getLogger returns the injected logger, falling back to slog.Default()
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.logger, slog.Default())
}

// Router returns a configured router for staff endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Staff profile reads are also needed by the absence-management view.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listStaff)
		r.With(authorize.RequiresAnyPermission(permissions.UsersRead, permissions.TimeTrackingManage), withTx).Get("/{id}", rs.getStaff)

		// Other staff reads require users:read permission.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/avatar", rs.serveStaffAvatar)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/groups", rs.getStaffGroups)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/substitutions", rs.getStaffSubstitutions)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/available", rs.getAvailableStaff)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/available-for-substitution", rs.getAvailableForSubstitution)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/by-role", rs.getStaffByRole)

		// Write operations require users:create, users:update, or users:delete permission
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createStaff)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateStaff)
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStaff)

		// Work schedule endpoints expose contractual target hours.
		r.With(authorize.RequiresAnyPermission(permissions.TimeTrackingManage, permissions.TimeTrackingOwn), withTx).Get("/{id}/schedule", rs.getSchedule)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/schedule", rs.updateSchedule)

		// Time tracking history for a specific staff member (admin read)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/history", rs.getStaffHistory)

		// Monatskarte (#1842)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/month-summary", rs.getStaffMonthSummary)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/schedule-targets", rs.getStaffScheduleTargets)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/export", rs.exportStaffSessions)

		// Vacation workflow admin-side (Tranche 4)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Get("/absences/pending", rs.listPendingAbsenceRequests)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/approve", rs.approveAbsence)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/deny", rs.denyAbsence)
		r.With(authorize.RequiresPermission(permissions.VacationApprove), withTx).Post("/absences/{absenceId}/question", rs.questionAbsence)
		r.With(authorize.RequiresAnyPermission(permissions.VacationApprove, permissions.TimeTrackingManage), withTx).Get("/{id}/vacation/quota", rs.getStaffVacationQuota)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}/vacation/quota", rs.setStaffVacationQuota)
		// Audit trail of a single work session, admin-facing. The MA-side
		// /api/time-tracking/{id}/edits enforces session-staff ownership
		// against the JWT subject; here the route guarantees the session
		// belongs to the staff named in the URL instead.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/{id}/time-tracking/sessions/{sessionId}/edits", rs.getStaffSessionEdits)

		// Admin cross-staff corrections. time_tracking:manage is the gate.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/time-tracking/sessions/{sessionId}", rs.adminUpdateStaffSession)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/time-tracking/sessions", rs.adminCreateStaffSession)

		// Absences for a specific staff member (manager read). The MA-Sicht uses
		// /api/time-tracking/absences which is scoped to the caller; this route
		// lets vacation and time-tracking managers see any staff member in the
		// same tenant.
		r.With(authorize.RequiresAnyPermission(permissions.VacationApprove, permissions.TimeTrackingManage), withTx).Get("/{id}/absences", rs.getStaffAbsences)

		// Admin absence writes (#1843): file or delete an absence on a staff
		// member's behalf; sick reports cascade into the plans in the same tx.
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/{id}/absences", rs.adminCreateStaffAbsence)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Delete("/{id}/absences/{absenceId}", rs.adminDeleteStaffAbsence)

		// PIN management endpoints - staff can manage their own PIN
		r.With(withTx).Get("/pin", rs.getPINStatus)
		r.With(withTx).Put("/pin", rs.updatePIN)
	})

	return r
}
