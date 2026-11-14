//go:build testhooks

package api

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
)

func NewShiftTypesRouterForBudgetTest(types workforce.ShiftTypeAdministration, references workforce.ShiftPlanReferenceQuery) chi.Router {
	return newShiftTypesResource(types, references).Router()
}

func NewWorkTimeModelsRouterForBudgetTest(models workforceCompose.WorkTime, notifyChanged func(context.Context)) chi.Router {
	return newWorkTimeModelsResource(models, models, notifyChanged).Router()
}
