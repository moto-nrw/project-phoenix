package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
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

// parentPortalGateKey carries the parents-portal lock (#4895) from the
// router-wide middleware to ParentScopeMiddleware.
type parentPortalGateKey struct{}

// parentPortalGate opens the parents portal while at least one guardian
// school of the account booked Eltern-App. The portal spans schools, so the
// lock is the account's, not one school's.
type parentPortalGate struct {
	booking         *settings.ModuleBooking
	guardianSchools identityaccess.GuardianSchools
}

// moduleRoutesMiddleware classifies every request by moduleRouteRules
// (#4893). It runs router-wide before any session is authenticated, so it
// decides nothing itself: a request of an additional module only carries the
// requirement on, and TenantScopeMiddleware checks it once the school of the
// session is bound. Basis routes, paths no rule covers and the surfaces in
// moduleRoutesLockedElsewhere pass unmarked.
func moduleRoutesMiddleware(booking *settings.ModuleBooking, guardianSchools identityaccess.GuardianSchools) func(http.Handler) http.Handler {
	parentGate := parentPortalGate{booking: booking, guardianSchools: guardianSchools}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), parentPortalGateKey{}, parentGate))
			// The same path chi routes on: the escaped form when the request
			// carries one, so an encoded slash cannot shift the segments.
			path := r.URL.Path
			if r.URL.RawPath != "" {
				path = r.URL.RawPath
			}
			rule, ok := moduleRouteFor(path)
			_, elsewhere := moduleRoutesLockedElsewhere[rule.prefix]
			if !ok || rule.module == moduleRouteBasis || elsewhere {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), moduleRequirementKey{}, moduleRequirement{module: rule.module, booking: booking})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireBookedModule refuses a request whose module the bound school has
// not booked with 403 module.not_booked; the module key travels in
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
	label := ""
	if definition, ok := settings.LookupModule(notBooked.Module); ok {
		label = definition.Label
	}
	return common.ErrorModuleNotBooked(notBooked, string(notBooked.Module), label)
}

// requireParentPortal refuses a parents-portal request once none of the
// account's guardian schools books Eltern-App (#4895), with 403
// module.not_booked for the first of them. Login and refresh refuse the same
// account; this lock ends a session that is still open at the moment the
// moto team cancels the module. A failed read is a server error. Without the
// gate (route-sized test routers) or without a guardian school the request
// passes: the portal's services already answer an account without children.
func requireParentPortal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate, ok := r.Context().Value(parentPortalGateKey{}).(parentPortalGate)
		claims := sessiontoken.ClaimsFromCtx(r.Context())
		if !ok || gate.booking == nil || gate.guardianSchools == nil || claims.ID <= 0 {
			next.ServeHTTP(w, r)
			return
		}
		schoolIDs, err := gate.guardianSchools.ListGuardianSchoolIDs(r.Context(), int64(claims.ID))
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("guardian schools could not be read", err))
			return
		}
		var firstRefusal *settings.ModuleNotBookedError
		for _, schoolID := range schoolIDs {
			err := gate.booking.RequireForTenant(r.Context(), schoolID, settings.ModuleElternApp)
			var notBooked *settings.ModuleNotBookedError
			switch {
			case err == nil:
				next.ServeHTTP(w, r)
				return
			case errors.As(err, &notBooked):
				if firstRefusal == nil {
					firstRefusal = notBooked
				}
			default:
				common.RenderError(w, r, common.ErrorInternalServerWrap("module booking could not be read", err))
				return
			}
		}
		if firstRefusal != nil {
			common.RenderError(w, r, moduleNotBookedResponse(firstRefusal))
			return
		}
		next.ServeHTTP(w, r)
	})
}
