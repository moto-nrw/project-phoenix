package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
)

// mealBillingErrorRules name the refusals of the Mensa-Monatsfreigabe
// (#4686) before the rules of the Meal Plan it reads.
var mealBillingErrorRules = append([]apiCommon.ErrorRule{
	{Target: mealplanModule.ErrBillingMonthRunning, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorConflictWithCode, errors.New(mealplanModule.ErrBillingMonthRunning.Detail), apiCommon.CodeMealsBillingMonthRunning)},
	{Target: mealplanModule.ErrBillingMonthReleased, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorConflictWithCode, errors.New(mealplanModule.ErrBillingMonthReleased.Detail), apiCommon.CodeMealsBillingMonthReleased)},
	{Target: mealplanModule.ErrBillingMonthChanged, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorConflictWithCode, errors.New(mealplanModule.ErrBillingMonthChanged.Detail), apiCommon.CodeMealsBillingMonthChanged)},
	{Target: mealplanModule.ErrBillingMonthNoTraeger, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorConflictWithCode, errors.New(mealplanModule.ErrBillingMonthNoTraeger.Detail), apiCommon.CodeMealsBillingMonthNoTraeger)},
	{Target: mealplanModule.ErrInvalidBillingMonth, Render: apiCommon.FixedCodedRenderer(apiCommon.ErrorInvalidRequestWithCode, errors.New(mealplanModule.ErrInvalidBillingMonth.Detail), apiCommon.CodeMealsBillingMonthInvalid)},
}, mealPlanErrorRules...)

// newMealBillingMonthResource serves the Mensa-Monatsfreigabe at
// /meal-billing-months in the school transaction.
func newMealBillingMonthResource(months mealplanModule.BillingMonths) *mealplanAPI.BillingMonthResource {
	return mealplanAPI.NewBillingMonthResource(months, mealplanAPI.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, mealplanAPI.Middleware)) {
			ProtectedTenantRoutes(router, register)
		},
		Permission: func(access mealplanAPI.Access) mealplanAPI.Middleware { return requireMealPlanAccess(access) },
		Success:    apiCommon.Respond,
		InvalidRequest: func(w http.ResponseWriter, r *http.Request, err error) {
			apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
		},
		ModuleFailure: func(w http.ResponseWriter, r *http.Request, err error, internalMessage string) {
			renderer := apiCommon.RulesRenderer(mealBillingErrorRules, apiCommon.ErrorInternalServerRenderer(internalMessage))
			apiCommon.RenderError(w, r, renderer(err))
		},
		CurrentAccountID: func(ctx context.Context) int64 { return int64(sessiontoken.ClaimsFromCtx(ctx).ID) },
	})
}
