// Package mealplan exposes the staff-facing meal plan (Essensplan) HTTP API:
// list a week, upsert one day, delete one day. The feature is gated per tenant
// by the operations.meal_plan_enabled setting. Parents read the same data
// through the parents portal (api/parent), not these routes.
package mealplan

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
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

	tokenAuth := jwt.MustNewTokenAuth()

	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		r.With(authorize.RequiresPermission(permissions.ConfigRead), withTx).Get("/", rs.getWeek)
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate), withTx).Put("/{date}", rs.setDay)
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate), withTx).Delete("/{date}", rs.deleteDay)
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

// SetDayRequest is the body for replacing a day's dishes.
type SetDayRequest struct {
	Dishes []DishRequest `json:"dishes"`
}

// Bind validates the set-day request.
func (req *SetDayRequest) Bind(_ *http.Request) error {
	return nil
}

// featureEnabled reports whether the meal plan is enabled for the current
// tenant, rendering a 403 and returning false when it is not. The meal plan is
// opt-out, so the fallback when a tenant has set no override is ON — matching
// the registry default and the parent-side ResolveBool path.
func (rs *Resource) featureEnabled(w http.ResponseWriter, r *http.Request) bool {
	if configService.ResolveBoolOrDefault(r.Context(), rs.SettingsService, configModel.KeyMealPlanEnabled, true, slog.Default()) {
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

	dishes := make([]mealplanSvc.DishInput, 0, len(req.Dishes))
	for _, d := range req.Dishes {
		dishes = append(dishes, mealplanSvc.DishInput{Dish: d.Dish, Note: d.Note})
	}

	tenantID := tenant.FromContext(r.Context())
	err = tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.MealPlanService.SetDay(ctx, date, dishes)
	})
	if err != nil {
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
