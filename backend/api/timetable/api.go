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
	"github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const dateLayout = "2006-01-02"

// Resource defines the timetable API resource.
//
// instanceService, personService, and logger are optional at construction
// time: tests that only exercise /periods or /materialize can pass nil and
// will get a 500 on the WP-B9 routes instead of a crash. Production wiring
// must supply all of them via NewResource.
type Resource struct {
	calendarPeriodService  scheduleSvc.CalendarPeriodService
	materializationService scheduleSvc.MaterializationService
	instanceService        scheduleSvc.InstanceService
	personService          userSvc.PersonService
	logger                 *slog.Logger
	db                     *bun.DB
}

// NewResource creates a new timetable resource. Optional services (see the
// Resource doc) may be nil; passing nil for the materialization or instance
// service makes the corresponding routes return 500 instead of silently
// misbehaving.
func NewResource(
	calendarPeriodService scheduleSvc.CalendarPeriodService,
	materializationService scheduleSvc.MaterializationService,
	instanceService scheduleSvc.InstanceService,
	personService userSvc.PersonService,
	logger *slog.Logger,
	db *bun.DB,
) *Resource {
	return &Resource{
		calendarPeriodService:  calendarPeriodService,
		materializationService: materializationService,
		instanceService:        instanceService,
		personService:          personService,
		logger:                 logger,
		db:                     db,
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
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/re-plan-week", rs.replanWeek)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/start", rs.startInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/complete", rs.completeInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/cancel", rs.cancelInstance)
		})
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
