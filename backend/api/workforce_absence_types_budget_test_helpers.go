//go:build testhooks

package api

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// NewAbsenceTypesRouterForBudgetTest exposes the retained root wiring to the
// Workforce adapter's external query-budget test.
func NewAbsenceTypesRouterForBudgetTest(types workforce.AbsenceTypeAdministration, resolveActor func(context.Context) (int64, error)) chi.Router {
	return newAbsenceTypesResource(types, resolveActor).Router()
}
