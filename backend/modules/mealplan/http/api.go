// Package mealplan exposes the staff Meal Plan HTTP adapter.
package mealplan

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
)

type Middleware = func(http.Handler) http.Handler

type Access uint8

const (
	AccessRead Access = iota + 1
	AccessWrite
)

type Runtime struct {
	Protected  func(chi.Router, func(chi.Router, Middleware))
	Permission func(Access) Middleware
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Failure    func(http.ResponseWriter, *http.Request, int, error)
}

type Resource struct {
	module  *mealplanModule.Module
	runtime Runtime
}

func NewResource(module *mealplanModule.Module, runtime Runtime) *Resource {
	if module == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.Success == nil || runtime.Failure == nil {
		panic("meal plan HTTP: all dependencies are required")
	}
	return &Resource{module: module, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		protected.With(rs.runtime.Permission(AccessRead), withTx).Get("/", rs.getWeek)
		protected.With(rs.runtime.Permission(AccessWrite), withTx).Put("/{date}", rs.setDay)
		protected.With(rs.runtime.Permission(AccessWrite), withTx).Delete("/{date}", rs.deleteDay)
	})
	return router
}

type MealPlanEntryResponse struct {
	Date     string  `json:"date"`
	Position int     `json:"position"`
	Dish     string  `json:"dish"`
	Note     *string `json:"note,omitempty"`
}

type DishRequest struct {
	Dish string  `json:"dish"`
	Note *string `json:"note"`
}

type SetDayRequest struct {
	Dishes *[]DishRequest `json:"dishes"`
}

func (request *SetDayRequest) Bind(*http.Request) error {
	if request.Dishes == nil {
		return errors.New("dishes is required (use an empty array to clear the day)")
	}
	return nil
}

func (rs *Resource) getWeek(w http.ResponseWriter, r *http.Request) {
	weekStart := r.URL.Query().Get("week_start")
	parsed, err := mealplanModule.ParseDate(weekStart)
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("week_start must be in YYYY-MM-DD format"))
		return
	}
	entries, err := rs.module.Week(r.Context(), parsed)
	if err != nil {
		rs.renderModuleError(w, r, err, "failed to load meal plan")
		return
	}
	responses := make([]MealPlanEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, MealPlanEntryResponse{
			Date: string(entry.Date), Position: entry.Position, Dish: entry.Dish, Note: entry.Note,
		})
	}
	rs.runtime.Success(w, r, http.StatusOK, responses, "Meal plan retrieved successfully")
}

func (rs *Resource) setDay(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	parsed, err := mealplanModule.ParseDate(date)
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("date must be in YYYY-MM-DD format"))
		return
	}
	request := &SetDayRequest{}
	if err := render.Bind(r, request); err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
		return
	}
	dishes := make([]mealplanModule.Dish, 0, len(*request.Dishes))
	for _, dish := range *request.Dishes {
		dishes = append(dishes, mealplanModule.Dish{Dish: dish.Dish, Note: dish.Note})
	}
	if err := rs.module.ReplaceDay(r.Context(), mealplanModule.ReplaceDay{Date: parsed, Dishes: dishes}); err != nil {
		rs.renderModuleError(w, r, err, "failed to save meal")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, nil, "Meal plan day saved successfully")
}

func (rs *Resource) deleteDay(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	parsed, err := mealplanModule.ParseDate(date)
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("date must be in YYYY-MM-DD format"))
		return
	}
	if err := rs.module.ClearDay(r.Context(), parsed); err != nil {
		rs.renderModuleError(w, r, err, "failed to delete meal")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, nil, "Meal deleted successfully")
}

func (rs *Resource) renderModuleError(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
	switch {
	case errors.Is(err, mealplanModule.ErrDisabled):
		rs.runtime.Failure(w, r, http.StatusForbidden, errors.New("feature_disabled"))
	case errors.Is(err, mealplanModule.ErrInvalidMealDate):
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("meal plan covers weekdays only (Monday-Friday)"))
	case errors.Is(err, mealplanModule.ErrInvalidDishes):
		rs.runtime.Failure(w, r, http.StatusBadRequest, err)
	default:
		rs.runtime.Failure(w, r, http.StatusInternalServerError, errors.Join(errors.New(internalMessage), err))
	}
}
