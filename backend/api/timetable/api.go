package timetable

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const dateLayout = "2006-01-02"

// Resource defines the timetable API resource.
//
// Optional services may be nil in tests that only exercise a subset of routes
// — the dependent handler will return 500 instead of panicking. Production
// wiring must populate every field.
type Resource struct {
	calendarPeriodService  scheduleSvc.CalendarPeriodService
	materializationService scheduleSvc.MaterializationService
	instanceService        scheduleSvc.InstanceService
	operationsService      scheduleSvc.TimetableOperationsService
	personService          userSvc.PersonService
	instanceStudentRepo    schedule.InstanceStudentRepository
	activityInstanceRepo   schedule.ActivityInstanceRepository
	activityExceptionRepo  schedule.ActivityExceptionRepository
	activityScheduleRepo   activities.ScheduleRepository
	instanceStaffRepo      schedule.InstanceStaffRepository
	activeGroupRepo        active.GroupRepository
	supervisorRepo         active.GroupSupervisorRepository
	arrivalScheduleRepo    schedule.StudentArrivalScheduleRepository
	arrivalExceptionRepo   schedule.StudentArrivalExceptionRepository
	pickupScheduleRepo     schedule.StudentPickupScheduleRepository
	pickupExceptionRepo    schedule.StudentPickupExceptionRepository
	visitRepo              active.VisitRepository
	studentRepo            users.StudentRepository
	staffRepo              users.StaffRepository
	roomRepo               facilities.RoomRepository
	activityGroupRepo      activities.GroupRepository
	activitySupervisorRepo activities.SupervisorPlannedRepository
	studentEnrollmentRepo  activities.StudentEnrollmentRepository
	timeframeRepo          schedule.TimeframeRepository
	userContextService     usercontextSvc.UserContextService
	settingsService        configSvc.SettingsService
	broadcaster            realtime.Broadcaster
	logger                 *slog.Logger
	db                     *bun.DB
}

// Dependencies bundles everything NewResource needs. Using a struct instead
// of a positional signature keeps tests — which frequently only populate a
// subset — readable and lets us add future deps without churning every call
// site.
type Dependencies struct {
	CalendarPeriodService  scheduleSvc.CalendarPeriodService
	MaterializationService scheduleSvc.MaterializationService
	InstanceService        scheduleSvc.InstanceService
	OperationsService      scheduleSvc.TimetableOperationsService
	PersonService          userSvc.PersonService
	InstanceStudentRepo    schedule.InstanceStudentRepository
	ActivityInstanceRepo   schedule.ActivityInstanceRepository
	ActivityExceptionRepo  schedule.ActivityExceptionRepository
	ActivityScheduleRepo   activities.ScheduleRepository
	InstanceStaffRepo      schedule.InstanceStaffRepository
	ActiveGroupRepo        active.GroupRepository
	SupervisorRepo         active.GroupSupervisorRepository
	ArrivalScheduleRepo    schedule.StudentArrivalScheduleRepository
	ArrivalExceptionRepo   schedule.StudentArrivalExceptionRepository
	PickupScheduleRepo     schedule.StudentPickupScheduleRepository
	PickupExceptionRepo    schedule.StudentPickupExceptionRepository
	VisitRepo              active.VisitRepository
	StudentRepo            users.StudentRepository
	StaffRepo              users.StaffRepository
	RoomRepo               facilities.RoomRepository
	ActivityGroupRepo      activities.GroupRepository
	ActivitySupervisorRepo activities.SupervisorPlannedRepository
	StudentEnrollmentRepo  activities.StudentEnrollmentRepository
	TimeframeRepo          schedule.TimeframeRepository
	UserContextService     usercontextSvc.UserContextService
	SettingsService        configSvc.SettingsService
	Broadcaster            realtime.Broadcaster
	Logger                 *slog.Logger
	DB                     *bun.DB
}

// NewResource creates a new timetable resource from the given Dependencies.
// Nil deps are tolerated at construction time; the dependent handler returns
// 500 at request time if one of its deps is unset.
func NewResource(deps Dependencies) *Resource {
	return &Resource{
		calendarPeriodService:  deps.CalendarPeriodService,
		materializationService: deps.MaterializationService,
		instanceService:        deps.InstanceService,
		operationsService:      deps.OperationsService,
		personService:          deps.PersonService,
		instanceStudentRepo:    deps.InstanceStudentRepo,
		activityInstanceRepo:   deps.ActivityInstanceRepo,
		activityExceptionRepo:  deps.ActivityExceptionRepo,
		activityScheduleRepo:   deps.ActivityScheduleRepo,
		instanceStaffRepo:      deps.InstanceStaffRepo,
		activeGroupRepo:        deps.ActiveGroupRepo,
		supervisorRepo:         deps.SupervisorRepo,
		arrivalScheduleRepo:    deps.ArrivalScheduleRepo,
		arrivalExceptionRepo:   deps.ArrivalExceptionRepo,
		pickupScheduleRepo:     deps.PickupScheduleRepo,
		pickupExceptionRepo:    deps.PickupExceptionRepo,
		visitRepo:              deps.VisitRepo,
		studentRepo:            deps.StudentRepo,
		staffRepo:              deps.StaffRepo,
		roomRepo:               deps.RoomRepo,
		activityGroupRepo:      deps.ActivityGroupRepo,
		activitySupervisorRepo: deps.ActivitySupervisorRepo,
		studentEnrollmentRepo:  deps.StudentEnrollmentRepo,
		timeframeRepo:          deps.TimeframeRepo,
		userContextService:     deps.UserContextService,
		settingsService:        deps.SettingsService,
		broadcaster:            deps.Broadcaster,
		logger:                 deps.Logger,
		db:                     deps.DB,
	}
}

// Router returns a configured router for timetable endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	tokenAuth := jwt.MustNewTokenAuth()

	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		r.Route("/periods", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).Get("/", rs.listPeriods)
			r.With(authorize.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/", rs.createPeriod)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).Get("/{id}", rs.getPeriod)
			r.With(authorize.RequiresPermission(permissions.SchedulesUpdate), withTx).Put("/{id}", rs.updatePeriod)
			r.With(authorize.RequiresPermission(permissions.SchedulesDelete), withTx).Delete("/{id}", rs.deletePeriod)
		})

		// WP-B8: manual materialization. Admin-only — reuses SchedulesManage
		// as the rough "you can do anything with the schedule" permission.
		// The scheduler job runs the same service; this endpoint exists so
		// admins can re-run ad hoc without waiting for the weekly cadence.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/materialize", rs.materialize)

		// WP-B9: instance lifecycle + re-plan-week. All four routes gated on
		// SchedulesManage. They share the tenant tx so start/complete/cancel
		// are atomic end-to-end (no dangling bridge rows on rollback).
		r.Route("/instances", func(r chi.Router) {
			// WP-F2 backend prerequisite: list instances in a date window for
			// the admin weekly planner. Read-only, gated on SchedulesRead so
			// office staff with view-only permissions can browse the plan
			// without being able to mutate it.
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/", rs.listInstances)
			// Spontaneous (and template-bound out-of-cycle) create. Returns
			// the same enriched shape as the list endpoint so the frontend
			// can splice the fresh row into its SWR cache.
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/", rs.createInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Put("/{id}", rs.updateInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Delete("/{id}", rs.deleteInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/re-plan-week", rs.replanWeek)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/start", rs.startInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/complete", rs.completeInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/cancel", rs.cancelInstance)

			// WP-B10: three-field attendance PATCH. Gated on SchedulesManage
			// like the lifecycle routes. Path params are {instance_id} and
			// {student_id} — distinct from the {id} param above so they live
			// in a sibling route subtree.
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Patch("/{instance_id}/students/{student_id}", rs.patchInstanceStudent)
		})

		// WP-B11: per-student day and week read endpoints. Both gated on
		// SchedulesRead; handler-level auth (authorize.CanReadStudent) adds
		// the per-student check.
		r.Route("/student/{id}", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/day", rs.getStudentDay)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/week", rs.getStudentWeek)
		})

		// WP-B12: gap detection + one-click substitute. Gaps is SchedulesRead
		// (information view), substitute is SchedulesManage (mutation).
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/gaps", rs.getGaps)
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/substitute", rs.substitute)

		// WP-B13: exception-conflict warnings (planning-only, read-only).
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/exception-conflicts", rs.getExceptionConflicts)

		r.Route("/operations", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/planned-now", rs.operationsPlannedNow)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/instances/{id}/roster", rs.operationsRoster)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/active-groups/{id}/roster", rs.operationsRosterByActiveGroup)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Post("/instances/{id}/start", rs.operationsStart)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Post("/instances/{id}/complete", rs.operationsComplete)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Post("/instances/{id}/students/{student_id}/check-in", rs.operationsCheckInStudent)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Post("/instances/{id}/students/{student_id}/check-out", rs.operationsCheckOutStudent)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Patch("/instances/{id}/students/{student_id}/attendance", rs.operationsPatchAttendance)
		})

		// Templates — admin shortcut to add a recurring activity (Mensa,
		// Lernzeit, AGs) without leaving the planner. Bundles timeframe +
		// activities.groups + activities.schedules into one HTTP call and
		// optionally materializes the visible week so the new instances
		// appear immediately on the grid.
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/templates", rs.listTemplates)
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/templates/{id}", rs.getTemplate)
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/templates", rs.createTemplate)
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Put("/templates/{id}", rs.updateTemplate)
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Delete("/templates/{id}", rs.archiveTemplate)
	})

	return r
}

// Request / Response types

// CalendarPeriodRequest represents a create/update request for a calendar period
type CalendarPeriodRequest struct {
	Name            string  `json:"name"`
	PeriodType      string  `json:"period_type"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	WeekCycleLength int     `json:"week_cycle_length"`
	WeekCycleAnchor *string `json:"week_cycle_anchor,omitempty"`
	IsActive        bool    `json:"is_active"`
}

// Bind validates the request
func (req *CalendarPeriodRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if len(req.Name) > 255 {
		return errors.New("name cannot exceed 255 characters")
	}
	if req.PeriodType == "" {
		return errors.New("period_type is required")
	}
	if !schedule.IsValidPeriodType(req.PeriodType) {
		return errors.New("invalid period_type, must be one of: school_year, semester, holiday, custom")
	}
	if req.StartDate == "" {
		return errors.New("start_date is required")
	}
	if req.EndDate == "" {
		return errors.New("end_date is required")
	}
	if req.WeekCycleLength <= 0 {
		req.WeekCycleLength = 1
	}
	return nil
}

// CalendarPeriodResponse represents a calendar period in API responses
type CalendarPeriodResponse struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	PeriodType      string  `json:"period_type"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	WeekCycleLength int     `json:"week_cycle_length"`
	WeekCycleAnchor *string `json:"week_cycle_anchor,omitempty"`
	IsActive        bool    `json:"is_active"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func mapPeriodToResponse(p *schedule.CalendarPeriod) CalendarPeriodResponse {
	resp := CalendarPeriodResponse{
		ID:              p.ID,
		Name:            p.Name,
		PeriodType:      p.PeriodType,
		StartDate:       p.StartDate.Format(dateLayout),
		EndDate:         p.EndDate.Format(dateLayout),
		WeekCycleLength: p.WeekCycleLength,
		IsActive:        p.IsActive,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.Format(time.RFC3339),
	}
	if p.WeekCycleAnchor != nil {
		anchor := p.WeekCycleAnchor.Format(dateLayout)
		resp.WeekCycleAnchor = &anchor
	}
	return resp
}

// parseDates extracts start_date, end_date, and optional week_cycle_anchor from a request.
// Returns parsed times and true on success, or renders an error and returns false.
func parseDates(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest) (startDate, endDate time.Time, anchor *time.Time, ok bool) {
	var err error
	startDate, err = time.Parse(dateLayout, req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return time.Time{}, time.Time{}, nil, false
	}

	endDate, err = time.Parse(dateLayout, req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return time.Time{}, time.Time{}, nil, false
	}

	if req.WeekCycleAnchor != nil {
		a, err := time.Parse(dateLayout, *req.WeekCycleAnchor)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid week_cycle_anchor format, expected YYYY-MM-DD")))
			return time.Time{}, time.Time{}, nil, false
		}
		anchor = &a
	}

	return startDate, endDate, anchor, true
}

// validatePeriodRules checks business rules after dates have been parsed.
// Returns true on success, or renders an error and returns false.
func validatePeriodRules(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest, startDate, endDate time.Time, anchor *time.Time) bool {
	if !endDate.After(startDate) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("end_date must be after start_date")))
		return false
	}
	if req.WeekCycleLength > 1 && anchor == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("week_cycle_anchor is required when week_cycle_length > 1")))
		return false
	}
	return true
}

// Handlers

func (rs *Resource) listPeriods(w http.ResponseWriter, r *http.Request) {
	periods, err := rs.calendarPeriodService.GetAllPeriods(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]CalendarPeriodResponse, len(periods))
	for i, p := range periods {
		responses[i] = mapPeriodToResponse(p)
	}

	common.Respond(w, r, http.StatusOK, responses, "Calendar periods retrieved successfully")
}

func (rs *Resource) getPeriod(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid period ID")))
		return
	}

	period, err := rs.calendarPeriodService.GetPeriodByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusOK, mapPeriodToResponse(period), "Calendar period retrieved successfully")
}

func (rs *Resource) createPeriod(w http.ResponseWriter, r *http.Request) {
	req := &CalendarPeriodRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	startDate, endDate, anchor, ok := parseDates(w, r, req)
	if !ok {
		return
	}

	if !validatePeriodRules(w, r, req, startDate, endDate, anchor) {
		return
	}

	period := &schedule.CalendarPeriod{
		Name:            req.Name,
		PeriodType:      req.PeriodType,
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCycleLength: req.WeekCycleLength,
		WeekCycleAnchor: anchor,
		IsActive:        req.IsActive,
	}

	if err := rs.calendarPeriodService.CreatePeriod(r.Context(), period); err != nil {
		if errors.Is(err, schedule.ErrCalendarPeriodNameConflict) {
			common.RenderError(w, r, common.ErrorConflict(schedule.ErrCalendarPeriodNameConflict))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, mapPeriodToResponse(period), "Calendar period created successfully")
}

func (rs *Resource) updatePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid period ID")))
		return
	}

	existing, err := rs.calendarPeriodService.GetPeriodByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	req := &CalendarPeriodRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	startDate, endDate, anchor, ok := parseDates(w, r, req)
	if !ok {
		return
	}

	if !validatePeriodRules(w, r, req, startDate, endDate, anchor) {
		return
	}

	existing.Name = req.Name
	existing.PeriodType = req.PeriodType
	existing.StartDate = startDate
	existing.EndDate = endDate
	existing.WeekCycleLength = req.WeekCycleLength
	existing.WeekCycleAnchor = anchor
	existing.IsActive = req.IsActive

	if err := rs.calendarPeriodService.UpdatePeriod(r.Context(), existing); err != nil {
		if errors.Is(err, schedule.ErrCalendarPeriodNameConflict) {
			common.RenderError(w, r, common.ErrorConflict(schedule.ErrCalendarPeriodNameConflict))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusOK, mapPeriodToResponse(existing), "Calendar period updated successfully")
}

func (rs *Resource) deletePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid period ID")))
		return
	}

	if _, err := rs.calendarPeriodService.GetPeriodByID(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	if err := rs.calendarPeriodService.DeletePeriod(r.Context(), id); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Calendar period deleted successfully")
}
