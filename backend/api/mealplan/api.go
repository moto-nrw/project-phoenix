// Package mealplan exposes the staff-facing meal plan (Essensplan) HTTP API:
// list a week, upsert one day, delete one day. The feature is gated per tenant
// by the operations.meal_plan_enabled setting. Parents read the same data
// through the parents portal (api/parent), not these routes.
package mealplan

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/mealplan"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	mealplanSvc "github.com/moto-nrw/project-phoenix/services/mealplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource is the meal plan API resource.
type Resource struct {
	MealPlanService mealplanSvc.Service
	SettingsService configService.SettingsService
	db              *bun.DB
}

// NewResource creates a new meal plan resource.
func NewResource(mealPlanService mealplanSvc.Service, settingsService configService.SettingsService, db *bun.DB) *Resource {
	return &Resource{
		MealPlanService: mealPlanService,
		SettingsService: settingsService,
		db:              db,
	}
}

// Router returns the configured router for staff meal plan endpoints.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		r.With(common.RequiresPermission(permissions.ConfigRead), withTx).Get("/", rs.getWeek)
		r.With(common.RequiresPermission(permissions.ConfigUpdate), withTx).Put("/{date}", rs.setDay)
		r.With(common.RequiresPermission(permissions.ConfigUpdate), withTx).Delete("/{date}", rs.deleteDay)
	})

	return r
}

// MealPlanEntryResponse is one dish on a day of the meal plan.
type MealPlanEntryResponse struct {
	Date     string  `json:"date"` // YYYY-MM-DD
	Position int     `json:"position"`
	Dish     string  `json:"dish"`
	Note     *string `json:"note,omitempty"`
}

func newMealPlanEntryResponse(entry *mealplan.MealPlanEntry) MealPlanEntryResponse {
	return MealPlanEntryResponse{
		Date:     entry.Date.String(),
		Position: entry.Position,
		Dish:     entry.Dish,
		Note:     entry.Note,
	}
}

// DishRequest is one dish in a day's submission.
type DishRequest struct {
	Dish string  `json:"dish"`
	Note *string `json:"note"`
}

// SetDayRequest is the body for replacing a day's dishes. Dishes is a pointer so
// a missing field or an explicit null ("{}" / {"dishes":null}) is rejected,
// while an explicit empty array ({"dishes":[]}) is a valid clear-the-day
// operation. Without this distinction a malformed body would silently wipe the
// day's stored dishes.
type SetDayRequest struct {
	Dishes *[]DishRequest `json:"dishes"`
}

// Bind validates the set-day request: the dishes array must be present (use an
// empty array to clear the day). A missing/null array is a malformed request,
// not an intentional clear.
func (req *SetDayRequest) Bind(_ *http.Request) error {
	if req.Dishes == nil {
		return errors.New("dishes is required (use an empty array to clear the day)")
	}
	return nil
}

// featureEnabled reports whether the meal plan is enabled for the current
// tenant, rendering a 403 and returning false when it is not. The meal plan is
// opt-out, so ResolveBool returns the registry default (ON) when a tenant has
// set no override — matching the parent-side ResolveBool path.
//
// It fails CLOSED on a settings lookup error: a transient/RLS/config-table
// failure while reading the override must NOT silently open the endpoints of a
// tenant that explicitly disabled the feature, so the error is rendered (500)
// rather than defaulted to ON. The routes run inside TenantTxMiddleware, so
// ResolveBool resolves against the current tenant.
func (rs *Resource) featureEnabled(w http.ResponseWriter, r *http.Request) bool {
	enabled, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyMealPlanEnabled)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to resolve meal plan setting", err))
		return false
	}
	if enabled {
		return true
	}
	common.RenderError(w, r, common.ErrorForbidden(errors.New("feature_disabled")))
	return false
}

// getWeek lists the Monday-Friday entries for the week containing week_start.
func (rs *Resource) getWeek(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}

	weekStart, err := timezone.ParseDate(r.URL.Query().Get("week_start"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("week_start must be in YYYY-MM-DD format")))
		return
	}

	entries, err := rs.MealPlanService.GetWeek(r.Context(), weekStart)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to load meal plan", err))
		return
	}

	responses := make([]MealPlanEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newMealPlanEntryResponse(entry))
	}
	common.Respond(w, r, http.StatusOK, responses, "Meal plan retrieved successfully")
}

// setDay replaces all dishes for one day.
func (rs *Resource) setDay(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}

	date, err := timezone.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be in YYYY-MM-DD format")))
		return
	}

	req := &SetDayRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	dishes := make([]mealplanSvc.DishInput, 0, len(*req.Dishes))
	for _, d := range *req.Dishes {
		dishes = append(dishes, mealplanSvc.DishInput{Dish: d.Dish, Note: d.Note})
	}

	tenantID := tenant.FromContext(r.Context())
	err = tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.MealPlanService.SetDay(ctx, date, dishes)
	})
	if err != nil {
		// A weekend/invalid-date rejection is a client error, not a 500.
		if errors.Is(err, mealplanSvc.ErrInvalidMealDate) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to save meal", err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Meal plan day saved successfully")
}

// deleteDay removes the meal for one day.
func (rs *Resource) deleteDay(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}

	date, err := timezone.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be in YYYY-MM-DD format")))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	err = tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.MealPlanService.Delete(ctx, date)
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to delete meal", err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Meal deleted successfully")
}
