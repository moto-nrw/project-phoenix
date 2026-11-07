//go:build testhooks

package api

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func NewShiftTypesRouterForBudgetTest(types workforce.ShiftTypeAdministration) chi.Router {
	return newShiftTypesResource(types).Router()
}

func NewWorkTimeModelsRouterForBudgetTest(models workforce.Capability, notifyChanged func(context.Context)) chi.Router {
	return newWorkTimeModelsResource(models, notifyChanged).Router()
}
