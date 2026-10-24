package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	settings "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// moduleRequirementKey carries the module a request path belongs to from
// the router-wide classification to the tenant scope gate.
type moduleRequirementKey struct{}

type moduleRequirement struct {
	module  settings.ModuleKey
	booking *settings.ModuleBooking
}

// moduleRoutesMiddleware classifies every request by moduleRouteRules
// (#4893). It runs router-wide before any session is authenticated, so it
// decides nothing itself: a request of an additional module only carries the
// requirement on, and TenantScopeMiddleware checks it once the school of the
// session is bound. Basis routes and paths no rule covers pass unmarked.
func moduleRoutesMiddleware(booking *settings.ModuleBooking) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			module, ok := moduleForRoute(r.URL.Path)
			if !ok || module == moduleRouteBasis {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), moduleRequirementKey{}, moduleRequirement{module: module, booking: booking})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireBookedModule refuses a request whose module the bound school has
// not booked with 403 settings.module_not_booked; the module key travels in
// details so the client can explain the missing module. A failed read is a
// server error, never a booking decision. Without a requirement (Basis, or a
// router without the classification) and without a bound school (platform
// tokens) the request passes.
func requireBookedModule(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requirement, ok := r.Context().Value(moduleRequirementKey{}).(moduleRequirement)
		tenantID := tenant.FromContext(r.Context())
		if !ok || tenantID <= 0 {
			next.ServeHTTP(w, r)
			return
		}
		// The gate runs before the request's tenant transaction, so it reads
		// the switch in a tenant transaction of its own; the request cache
		// still serves every later read of the same switch.
		err := requirement.booking.RequireForTenant(r.Context(), tenantID, requirement.module)
		var notBooked *settings.ModuleNotBookedError
		switch {
		case errors.As(err, &notBooked):
			common.RenderError(w, r, moduleNotBookedResponse(notBooked))
		case err != nil:
			common.RenderError(w, r, common.ErrorInternalServerWrap("module booking could not be read", err))
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func moduleNotBookedResponse(notBooked *settings.ModuleNotBookedError) render.Renderer {
	details := map[string]any{"module": string(notBooked.Module)}
	if definition, ok := settings.LookupModule(notBooked.Module); ok {
		details["module_label"] = definition.Label
	}
	return common.ErrorForbiddenWithDetails(notBooked, common.CodeSettingsModuleNotBooked, details)
}
