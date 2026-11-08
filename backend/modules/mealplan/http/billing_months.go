package mealplan

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
)

// AccessBillingRelease reads and releases the Mensa months (#4686): the
// OGS-Leitung sees the children's names and maintains the Mensa.
const AccessBillingRelease Access = AccessDailyParticipants + 1

// BillingMonthResource serves the Mensa-Monatsfreigabe at
// /meal-billing-months in the school transaction of the session's school.
type BillingMonthResource struct {
	months  mealplanModule.BillingMonths
	runtime Runtime
}

// NewBillingMonthResource builds the resource.
func NewBillingMonthResource(months mealplanModule.BillingMonths, runtime Runtime) *BillingMonthResource {
	if months == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.Success == nil ||
		runtime.InvalidRequest == nil || runtime.ModuleFailure == nil || runtime.CurrentAccountID == nil {
		panic("meal plan HTTP: the billing months and their runtime are required")
	}
	return &BillingMonthResource{months: months, runtime: runtime}
}

// Router returns the routes mounted at /meal-billing-months.
func (rs *BillingMonthResource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		release := protected.With(rs.runtime.Permission(AccessBillingRelease), withTx)
		release.Get("/", rs.overview)
		release.Get("/{month}", rs.month)
		release.Post("/{month}/release", rs.release)
	})
	return router
}

// BillingMonthReleaseRequest names the counts the OGS-Leitung saw.
type BillingMonthReleaseRequest struct {
	ChildCount *int `json:"child_count"`
	MealCount  *int `json:"meal_count"`
}

// Bind requires both counts.
func (req *BillingMonthReleaseRequest) Bind(*http.Request) error {
	if req.ChildCount == nil || req.MealCount == nil {
		return errors.New("child_count and meal_count are required")
	}
	return nil
}

func (rs *BillingMonthResource) overview(w http.ResponseWriter, r *http.Request) {
	months, err := rs.months.BillingMonthOverview(r.Context())
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to read the Mensa months")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, months, "Mensa months retrieved successfully")
}

func (rs *BillingMonthResource) month(w http.ResponseWriter, r *http.Request) {
	month, err := rs.months.BillingMonth(r.Context(), mealplanModule.Date(chi.URLParam(r, "month")))
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to read the Mensa month")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, month, "Mensa month retrieved successfully")
}

func (rs *BillingMonthResource) release(w http.ResponseWriter, r *http.Request) {
	req := &BillingMonthReleaseRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.runtime.InvalidRequest(w, r, err)
		return
	}
	month, err := rs.months.ReleaseBillingMonth(r.Context(), mealplanModule.BillingMonthRelease{
		Month: mealplanModule.Date(chi.URLParam(r, "month")), AccountID: rs.runtime.CurrentAccountID(r.Context()),
		ChildCount: *req.ChildCount, MealCount: *req.MealCount,
	})
	if err != nil {
		rs.runtime.ModuleFailure(w, r, err, "failed to release the Mensa month")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, month, "Mensa month released successfully")
}
