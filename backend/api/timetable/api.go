package timetable

import (
	"errors"
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
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const dateLayout = "2006-01-02"

// Resource defines the timetable API resource
type Resource struct {
	calendarPeriodService scheduleSvc.CalendarPeriodService
	db                    *bun.DB
}

// NewResource creates a new timetable resource
func NewResource(calendarPeriodService scheduleSvc.CalendarPeriodService, db *bun.DB) *Resource {
	return &Resource{
		calendarPeriodService: calendarPeriodService,
		db:                    db,
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
	if req.PeriodType == "" {
		return errors.New("period_type is required")
	}
	if req.StartDate == "" {
		return errors.New("start_date is required")
	}
	if req.EndDate == "" {
		return errors.New("end_date is required")
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
		common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
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

	startDate, err := time.Parse(dateLayout, req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return
	}

	endDate, err := time.Parse(dateLayout, req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return
	}

	period := &schedule.CalendarPeriod{
		Name:            req.Name,
		PeriodType:      req.PeriodType,
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCycleLength: req.WeekCycleLength,
		IsActive:        req.IsActive,
	}

	if req.WeekCycleAnchor != nil {
		anchor, err := time.Parse(dateLayout, *req.WeekCycleAnchor)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid week_cycle_anchor format, expected YYYY-MM-DD")))
			return
		}
		period.WeekCycleAnchor = &anchor
	}

	if period.WeekCycleLength == 0 {
		period.WeekCycleLength = 1
	}

	if err := rs.calendarPeriodService.CreatePeriod(r.Context(), period); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		return
	}

	req := &CalendarPeriodRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	startDate, err := time.Parse(dateLayout, req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return
	}

	endDate, err := time.Parse(dateLayout, req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return
	}

	existing.Name = req.Name
	existing.PeriodType = req.PeriodType
	existing.StartDate = startDate
	existing.EndDate = endDate
	existing.WeekCycleLength = req.WeekCycleLength
	existing.IsActive = req.IsActive

	if req.WeekCycleAnchor != nil {
		anchor, err := time.Parse(dateLayout, *req.WeekCycleAnchor)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid week_cycle_anchor format, expected YYYY-MM-DD")))
			return
		}
		existing.WeekCycleAnchor = &anchor
	} else {
		existing.WeekCycleAnchor = nil
	}

	if existing.WeekCycleLength == 0 {
		existing.WeekCycleLength = 1
	}

	if err := rs.calendarPeriodService.UpdatePeriod(r.Context(), existing); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

	if err := rs.calendarPeriodService.DeletePeriod(r.Context(), id); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Calendar period deleted successfully")
}
