package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CareOfferingResponse is the wire shape for a single offering. IDs
// stringified so the frontend keeps its int64-as-string convention.
type CareOfferingResponse struct {
	ID                     string     `json:"id"`
	CalendarPeriodID       *string    `json:"calendar_period_id,omitempty"`
	ActivityGroupID        *string    `json:"activity_group_id,omitempty"`
	Name                   string     `json:"name"`
	Description            *string    `json:"description,omitempty"`
	DaysOfWeekMode         string     `json:"days_of_week_mode"`
	AvailableDays          []string   `json:"available_days"`
	IncludesHolidayCare    bool       `json:"includes_holiday_care"`
	IncludesLunch          bool       `json:"includes_lunch"`
	Capacity               *int       `json:"capacity,omitempty"`
	PriceCents             *int       `json:"price_cents,omitempty"`
	ApplicationWindowStart *time.Time `json:"application_window_start,omitempty"`
	ApplicationWindowEnd   *time.Time `json:"application_window_end,omitempty"`
	ServiceStartDate       *time.Time `json:"service_start_date,omitempty"`
	ServiceEndDate         *time.Time `json:"service_end_date,omitempty"`
	IsActive               bool       `json:"is_active"`
	SortOrder              int        `json:"sort_order"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

func toCareOfferingResponse(o *enrollmentModels.CareOffering) CareOfferingResponse {
	resp := CareOfferingResponse{
		ID:                     strconv.FormatInt(o.ID, 10),
		Name:                   o.Name,
		Description:            o.Description,
		DaysOfWeekMode:         o.DaysOfWeekMode,
		AvailableDays:          o.AvailableDays,
		IncludesHolidayCare:    o.IncludesHolidayCare,
		IncludesLunch:          o.IncludesLunch,
		Capacity:               o.Capacity,
		PriceCents:             o.PriceCents,
		ApplicationWindowStart: o.ApplicationWindowStart,
		ApplicationWindowEnd:   o.ApplicationWindowEnd,
		ServiceStartDate:       o.ServiceStartDate,
		ServiceEndDate:         o.ServiceEndDate,
		IsActive:               o.IsActive,
		SortOrder:              o.SortOrder,
		CreatedAt:              o.CreatedAt,
		UpdatedAt:              o.UpdatedAt,
	}
	if o.CalendarPeriodID != nil {
		s := strconv.FormatInt(*o.CalendarPeriodID, 10)
		resp.CalendarPeriodID = &s
	}
	if o.ActivityGroupID != nil {
		s := strconv.FormatInt(*o.ActivityGroupID, 10)
		resp.ActivityGroupID = &s
	}
	return resp
}

// CareOfferingRequest is the wire shape POST + PUT accept.
type CareOfferingRequest struct {
	CalendarPeriodID       *int64     `json:"calendar_period_id,omitempty"`
	ActivityGroupID        *int64     `json:"activity_group_id,omitempty"`
	Name                   string     `json:"name"`
	Description            *string    `json:"description,omitempty"`
	DaysOfWeekMode         string     `json:"days_of_week_mode"`
	AvailableDays          []string   `json:"available_days"`
	IncludesHolidayCare    bool       `json:"includes_holiday_care"`
	IncludesLunch          bool       `json:"includes_lunch"`
	Capacity               *int       `json:"capacity,omitempty"`
	PriceCents             *int       `json:"price_cents,omitempty"`
	ApplicationWindowStart *time.Time `json:"application_window_start,omitempty"`
	ApplicationWindowEnd   *time.Time `json:"application_window_end,omitempty"`
	ServiceStartDate       *time.Time `json:"service_start_date,omitempty"`
	ServiceEndDate         *time.Time `json:"service_end_date,omitempty"`
	IsActive               bool       `json:"is_active"`
	SortOrder              int        `json:"sort_order"`
}

// Bind satisfies render.Binder. Field-level validation runs in the
// model's Validate (called inside Repository.Create/Update).
func (req *CareOfferingRequest) Bind(_ *http.Request) error {
	if req.AvailableDays == nil {
		req.AvailableDays = []string{}
	}
	return nil
}

func (req *CareOfferingRequest) toModel(existingID int64) *enrollmentModels.CareOffering {
	o := &enrollmentModels.CareOffering{
		CalendarPeriodID:       req.CalendarPeriodID,
		ActivityGroupID:        req.ActivityGroupID,
		Name:                   req.Name,
		Description:            req.Description,
		DaysOfWeekMode:         req.DaysOfWeekMode,
		AvailableDays:          req.AvailableDays,
		IncludesHolidayCare:    req.IncludesHolidayCare,
		IncludesLunch:          req.IncludesLunch,
		Capacity:               req.Capacity,
		PriceCents:             req.PriceCents,
		ApplicationWindowStart: req.ApplicationWindowStart,
		ApplicationWindowEnd:   req.ApplicationWindowEnd,
		ServiceStartDate:       req.ServiceStartDate,
		ServiceEndDate:         req.ServiceEndDate,
		IsActive:               req.IsActive,
		SortOrder:              req.SortOrder,
	}
	o.ID = existingID
	return o
}

// CloneCareOfferingRequest is the body POST /{id}/clone accepts.
type CloneCareOfferingRequest struct {
	TargetCalendarPeriodID int64 `json:"target_calendar_period_id"`
	DaysOffset             int   `json:"days_offset"`
}

func (req *CloneCareOfferingRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) listCareOfferings(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}

	periodFilter := r.URL.Query().Get("calendar_period_id")
	var (
		offerings []*enrollmentModels.CareOffering
		err       error
	)
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		var listErr error
		if periodFilter != "" {
			id, parseErr := strconv.ParseInt(periodFilter, 10, 64)
			if parseErr != nil || id <= 0 {
				return errors.New("invalid calendar_period_id")
			}
			offerings, listErr = rs.CareOfferingService.ListByCalendarPeriod(ctx, id)
			return listErr
		}
		offerings, listErr = rs.CareOfferingService.List(ctx)
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	out := make([]CareOfferingResponse, 0, len(offerings))
	for _, o := range offerings {
		out = append(out, toCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, out, "Care offerings retrieved")
}

func (rs *Resource) getCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}
	var offering *enrollmentModels.CareOffering
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		o, e := rs.CareOfferingService.GetByID(ctx, id)
		offering = o
		return e
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareOfferingResponse(offering), "Care offering retrieved")
}

func (rs *Resource) createCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	req := &CareOfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var offering *enrollmentModels.CareOffering
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		o, e := rs.CareOfferingService.Create(ctx, req.toModel(0))
		offering = o
		return e
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareOfferingResponse(offering), "Care offering created")
}

func (rs *Resource) updateCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}
	req := &CareOfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.CareOfferingService.Update(ctx, req.toModel(id))
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Refetch so the response reflects DB-side timestamps.
	var refreshed *enrollmentModels.CareOffering
	if fetchErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		o, e := rs.CareOfferingService.GetByID(ctx, id)
		refreshed = o
		return e
	}); fetchErr != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fetchErr))
		return
	}
	common.Respond(w, r, http.StatusOK, toCareOfferingResponse(refreshed), "Care offering updated")
}

func (rs *Resource) deleteCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.CareOfferingService.Delete(ctx, id)
	})
	if err != nil {
		// FK violation when request_child_offerings already references
		// this offering — admin should soft-delete (is_active=false).
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.RespondNoContent(w, r)
}

func (rs *Resource) cloneCareOffering(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	sourceID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || sourceID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}
	req := &CloneCareOfferingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var clone *enrollmentModels.CareOffering
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		c, e := rs.CareOfferingService.Clone(ctx, sourceID, req.TargetCalendarPeriodID, req.DaysOffset)
		clone = c
		return e
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareOfferingResponse(clone), "Care offering cloned")
}

// listPublicCareOfferings is the parent-facing endpoint. No JWT — the
// {tenantSlug} path param resolves to a tenant whose context we set
// for the lookup. Window-filtered to active + currently-open offerings.
func (rs *Resource) listPublicCareOfferings(w http.ResponseWriter, r *http.Request) {
	if rs.CareOfferingService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("care offering service not configured")))
		return
	}
	if rs.SchoolRepo == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("public endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	var offerings []*enrollmentModels.CareOffering
	err := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, schoolErr := rs.SchoolRepo.FindBySlug(adminCtx, slug)
		if schoolErr != nil {
			return errors.New("tenant not found")
		}
		if school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		// Switch into the resolved tenant context for the query so
		// RLS narrows to the tenant's offerings.
		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)
		return tenant.WithTenantTx(tenantCtx, rs.db, school.ID, func(txCtx context.Context, _ bun.Tx) error {
			list, listErr := rs.CareOfferingService.ListPublic(txCtx, time.Now())
			offerings = list
			return listErr
		})
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}

	out := make([]CareOfferingResponse, 0, len(offerings))
	for _, o := range offerings {
		out = append(out, toCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, out, "Public care offerings retrieved")
}

// We deliberately don't expose enrollmentService here — it is already
// referenced via *Resource.CareOfferingService.
var _ = enrollmentService.ErrCareOfferingNotFound
