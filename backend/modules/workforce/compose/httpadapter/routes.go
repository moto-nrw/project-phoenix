package httpadapter

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/inbound"
)

// SubstitutionsRoutes serves /api/substitutions over the substitution
// operations capability (#4003).
func SubstitutionsRoutes(substitutions workforce.Substitutions) chi.Router {
	return inbound.NewSubstitutionsResource(substitutions).Router()
}
