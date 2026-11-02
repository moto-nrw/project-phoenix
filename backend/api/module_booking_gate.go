package api

import (
	"context"
	"errors"
	"fmt"
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

// parentPortalCheck answers nil while at least one guardian school of the
// account booked Eltern-App, *settings.ModuleNotBookedError for the first
// school otherwise, and any other error for a failed read. The portal spans
// schools, so the lock is the account's, not one school's.
type parentPortalCheck func(ctx context.Context, accountID int64) error

// newParentPortalCheck binds the check to the module switches and the
// guardian schools of Identity & Access. Without either it is nil and the
// lock passes, as in route-sized test routers.
func newParentPortalCheck(booking *settings.ModuleBooking, guardianSchools identityaccess.GuardianSchools) parentPortalCheck {
	if booking == nil || guardianSchools == nil {
		return nil
	}
	return func(ctx context.Context, accountID int64) error {
		schoolIDs, err := guardianSchools.ListGuardianSchoolIDs(ctx, accountID)
		if err != nil {
			return fmt.Errorf("guardian schools could not be read: %w", err)
		}
		var firstRefusal error
		for _, schoolID := range schoolIDs {
			err := booking.RequireForTenant(ctx, schoolID, settings.ModuleElternApp)
			var notBooked *settings.ModuleNotBookedError
			switch {
			case err == nil:
				return nil
			case errors.As(err, &notBooked):
				if firstRefusal == nil {
					firstRefusal = err
				}
			default:
				return err
			}
		}
		return firstRefusal
	}
}

// moduleRoutesMiddleware classifies every request by moduleRouteRules
// (#4893). It runs router-wide before any session is authenticated, so it
// decides nothing itself: a request of an additional module only carries the
// requirement on, and TenantScopeMiddleware checks it once the school of the
// session is bound. Basis routes, paths no rule covers and the surfaces in
// moduleRoutesLockedElsewhere pass unmarked.
func moduleRoutesMiddleware(booking *settings.ModuleBooking, guardianSchools identityaccess.GuardianSchools) func(http.Handler) http.Handler {
	parentCheck := newParentPortalCheck(booking, guardianSchools)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if parentCheck != nil {
				r = r.WithContext(context.WithValue(r.Context(), parentPortalGateKey{}, parentCheck))
			}
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
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		serveBookedModules(w, r, next, requirement.booking, requirement.module)
	})
}

// gateDeviceAuthentication puts the module lock of the kiosk behind a device
// authenticator (#4896). Devices carry no session, so TenantScopeMiddleware
// never sees them; the school is the device's, bound by the authenticator.
// Every kiosk route needs NFC-Anwesenheit, and a route of a further module
// needs that module too: the staff clock needs Zeiterfassung as well. The
// lock runs after authentication, so only a valid device learns about the
// booking of its school.
func gateDeviceAuthentication(authenticator func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return authenticator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requirement, ok := r.Context().Value(moduleRequirementKey{}).(moduleRequirement)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			modules := []settings.ModuleKey{settings.ModuleNFCAnwesenheit}
			if requirement.module != settings.ModuleNFCAnwesenheit {
				modules = append(modules, requirement.module)
			}
			serveBookedModules(w, r, next, requirement.booking, modules...)
		}))
	}
}

// serveBookedModules serves the request once the bound school booked every
// module, and answers module.not_booked for the first one it did not.
func serveBookedModules(w http.ResponseWriter, r *http.Request, next http.Handler, booking *settings.ModuleBooking, modules ...settings.ModuleKey) {
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		next.ServeHTTP(w, r)
		return
	}
	// The gate runs before the request's tenant transaction, so it reads
	// the switch in a tenant transaction of its own; the request cache
	// still serves every later read of the same switch.
	err := booking.RequireForTenant(r.Context(), tenantID, modules...)
	var notBooked *settings.ModuleNotBookedError
	switch {
	case errors.As(err, &notBooked):
		common.RenderError(w, r, moduleNotBookedResponse(notBooked))
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServerWrap("module booking could not be read", err))
	default:
		next.ServeHTTP(w, r)
	}
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
		check, ok := r.Context().Value(parentPortalGateKey{}).(parentPortalCheck)
		claims := sessiontoken.ClaimsFromCtx(r.Context())
		if !ok || check == nil || claims.ID <= 0 {
			next.ServeHTTP(w, r)
			return
		}
		err := check(r.Context(), int64(claims.ID))
		var notBooked *settings.ModuleNotBookedError
		switch {
		case errors.As(err, &notBooked):
			common.RenderError(w, r, moduleNotBookedResponse(notBooked))
		case err != nil:
			common.RenderError(w, r, common.ErrorInternalServerWrap("parents portal booking could not be read", err))
		default:
			next.ServeHTTP(w, r)
		}
	})
}
