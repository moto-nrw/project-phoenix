package timetable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
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
	templateSplitService   scheduleSvc.TemplateSplitService
	personService          userSvc.PersonService
	timetableData          scheduleSvc.TimetableDataService
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
	TemplateSplitService   scheduleSvc.TemplateSplitService
	PersonService          userSvc.PersonService
	TimetableData          scheduleSvc.TimetableDataService
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
		templateSplitService:   deps.TemplateSplitService,
		personService:          deps.PersonService,
		timetableData:          deps.TimetableData,
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
			// WP-B1: idempotent school-year bootstrap. Same permission and tx
			// middleware as POST /periods — it is just a specialized create.
			r.With(authorize.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/bootstrap", rs.bootstrapPeriods)
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

		// Planning-time conflict probe for a hypothetical slot (read-only,
		// advisory). Same permission + tx middleware as /exception-conflicts.
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/conflicts", rs.getPlannedConflicts)

		r.Route("/operations", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/capabilities", rs.operationsCapabilities)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/planned-now", rs.operationsPlannedNow)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/instances/{id}/roster", rs.operationsRoster)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/active-groups/{id}/roster", rs.operationsRosterByActiveGroup)
			// Operational mutations are available to normal supervisors with
			// SchedulesRead; the service enforces assignment/admin access via
			// requireCanOperate before touching schedule or active state.
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Post("/spontaneous/start", rs.operationsCreateAndStartSpontaneous)
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
		// WP-B3: "Dieser und alle folgenden" — cap the old template at the
		// effective date and continue with a successor template.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/templates/{id}/split", rs.splitTemplate)
		// "Dieser und alle folgenden löschen" — cap the template at the
		// effective date without creating a successor and remove planned
		// future instances.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/templates/{id}/end", rs.endTemplate)
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

// PeriodWarning is an advisory, never-blocking notice attached to a calendar
// period response (WP-B2, QA #1577 H3). One entry is emitted per overlapping
// active period.
type PeriodWarning struct {
	Code                   string   `json:"code"`
	Message                string   `json:"message"`
	OverlappingPeriodIDs   []int64  `json:"overlapping_period_ids"`
	OverlappingPeriodNames []string `json:"overlapping_period_names"`
}

// warningCodeOverlappingActivePeriods is the machine-readable code the
// frontend matches on; the German message is display-only.
const warningCodeOverlappingActivePeriods = "overlapping_active_periods"

// germanDateLayout renders calendar dates as dd.mm.yyyy for user-facing text.
const germanDateLayout = "02.01.2006"

// CalendarPeriodResponse represents a calendar period in API responses
type CalendarPeriodResponse struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	PeriodType      string          `json:"period_type"`
	StartDate       string          `json:"start_date"`
	EndDate         string          `json:"end_date"`
	WeekCycleLength int             `json:"week_cycle_length"`
	WeekCycleAnchor *string         `json:"week_cycle_anchor,omitempty"`
	IsActive        bool            `json:"is_active"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	Warnings        []PeriodWarning `json:"warnings,omitempty"`
}

func mapPeriodToResponse(p *schedule.CalendarPeriod) CalendarPeriodResponse {
	resp := CalendarPeriodResponse{
		ID:              p.ID,
		Name:            p.Name,
		PeriodType:      p.PeriodType,
		StartDate:       p.StartDate.String(),
		EndDate:         p.EndDate.String(),
		WeekCycleLength: p.WeekCycleLength,
		IsActive:        p.IsActive,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.Format(time.RFC3339),
	}
	if p.WeekCycleAnchor != nil {
		anchor := p.WeekCycleAnchor.String()
		resp.WeekCycleAnchor = &anchor
	}
	return resp
}

// parseDates extracts start_date, end_date, and optional week_cycle_anchor from a request.
// Returns parsed calendar dates and true on success, or renders an error and returns false.
func parseDates(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest) (startDate, endDate timezone.Date, anchor *timezone.Date, ok bool) {
	var err error
	startDate, err = timezone.ParseDate(req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, nil, false
	}

	endDate, err = timezone.ParseDate(req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, nil, false
	}

	if req.WeekCycleAnchor != nil {
		a, err := timezone.ParseDate(*req.WeekCycleAnchor)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid week_cycle_anchor format, expected YYYY-MM-DD")))
			return timezone.Date{}, timezone.Date{}, nil, false
		}
		anchor = &a
	}

	return startDate, endDate, anchor, true
}

// validatePeriodRules checks business rules after dates have been parsed.
// Returns true on success, or renders an error and returns false.
func validatePeriodRules(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest, startDate, endDate timezone.Date, anchor *timezone.Date) bool {
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

// attachOverlapWarnings decorates resp with one advisory warning per active
// period overlapping the just-saved period (WP-B2, QA #1577 H3). The check
// runs after a successful save and must never turn the 2xx into an error:
// lookup failures are logged at Warn and the response simply ships without
// warnings.
func (rs *Resource) attachOverlapWarnings(ctx context.Context, period *schedule.CalendarPeriod, resp *CalendarPeriodResponse) {
	overlaps, err := rs.calendarPeriodService.FindActiveOverlaps(ctx, period)
	if err != nil {
		rs.getLogger().Warn("calendar period overlap check failed, omitting warnings",
			slog.Int64("period_id", period.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	for _, o := range overlaps {
		resp.Warnings = append(resp.Warnings, PeriodWarning{
			Code: warningCodeOverlappingActivePeriods,
			Message: fmt.Sprintf(
				"Überschneidet sich mit aktivem Zeitraum „%s“ (%s – %s). Termine werden im Zweifel dem älteren Zeitraum zugeordnet.",
				o.Name,
				o.StartDate.Format(germanDateLayout),
				o.EndDate.Format(germanDateLayout),
			),
			OverlappingPeriodIDs:   []int64{o.ID},
			OverlappingPeriodNames: []string{o.Name},
		})
	}
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

	resp := mapPeriodToResponse(period)
	rs.attachOverlapWarnings(r.Context(), period, &resp)
	common.Respond(w, r, http.StatusCreated, resp, "Calendar period created successfully")
}

// bootstrapPeriodsResponse is the wire shape of POST /periods/bootstrap.
type bootstrapPeriodsResponse struct {
	Periods []CalendarPeriodResponse `json:"periods"`
	Created bool                     `json:"created"`
}

// bootstrapPeriods handles POST /api/timetable/periods/bootstrap (WP-B1).
// It ensures the tenant has at least one calendar period, creating the
// current school year when none exists. Idempotent by design: the response
// is always 200 with the tenant's periods plus a created flag — repeated
// calls and concurrent calls never yield a 409.
func (rs *Resource) bootstrapPeriods(w http.ResponseWriter, r *http.Request) {
	periods, created, err := rs.calendarPeriodService.EnsureDefaultSchoolYear(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]CalendarPeriodResponse, len(periods))
	for i, p := range periods {
		responses[i] = mapPeriodToResponse(p)
	}

	common.Respond(w, r, http.StatusOK, bootstrapPeriodsResponse{
		Periods: responses,
		Created: created,
	}, "Calendar periods bootstrapped successfully")
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

	resp := mapPeriodToResponse(existing)
	rs.attachOverlapWarnings(r.Context(), existing, &resp)
	common.Respond(w, r, http.StatusOK, resp, "Calendar period updated successfully")
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
