//go:build testhooks

package api

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func NewAbsenceTypesRouterForBudgetTest(types workforce.AbsenceTypeAdministration, resolveActor func(context.Context) (int64, error)) chi.Router {
	return newAbsenceTypesResource(types, resolveActor).Router()
}
