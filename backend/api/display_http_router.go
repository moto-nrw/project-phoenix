package api

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// NewDisplayHTTPRouter composes the display routes over the owner capability.
func NewDisplayHTTPRouter(fleet devicefleet.Capability, enabled func(context.Context) (bool, error)) chi.Router {
	return newDisplayHTTPResource(fleet, enabled).Router()
}
