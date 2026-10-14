package httpadapter

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/inbound"
)

// AbsenceTypesRoutes serves /api/absence-types over the absence-type
// administration capability. resolveActor identifies the staff member behind
// an allowance change. The root mounts the routes and does not import the
// inbound adapter (#4003).
func AbsenceTypesRoutes(types workforce.AbsenceTypeAdministration, resolveActor func(context.Context) (int64, error)) chi.Router {
	return inbound.NewAbsenceTypesResource(types, resolveActor).Router()
}

// SubstitutionsRoutes serves /api/substitutions over the substitution
// operations capability (#4003).
func SubstitutionsRoutes(substitutions workforce.Substitutions) chi.Router {
	return inbound.NewSubstitutionsResource(substitutions).Router()
}
