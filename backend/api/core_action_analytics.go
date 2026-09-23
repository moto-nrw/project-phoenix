package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-nrw/project-phoenix/analytics"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	projectJWT "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// coreActionAnalytics binds the core-action middleware (#3602) to the
// session model: the request's verified session, or the session a login
// response mints. The root mounts it after the session verifier.
func coreActionAnalytics(tracker analytics.Tracker, sessionAuth *projectJWT.TokenAuth) func(http.Handler) http.Handler {
	return analytics.CoreActionMiddleware(analytics.CoreActionConfig{
		Tracker: tracker,
		RequestActor: func(r *http.Request) (analytics.Actor, bool) {
			// The root verifier leaves a token only when its signature and
			// expiry hold; ParseClaims rejects MFA interim tokens.
			token, raw, err := jwtauth.FromContext(r.Context())
			if err != nil || token == nil {
				return analytics.Actor{}, false
			}
			var claims projectJWT.AppClaims
			if claims.ParseClaims(raw) != nil {
				return analytics.Actor{}, false
			}
			return analyticsActor(claims)
		},
		SessionActor: func(accessToken string) (analytics.Actor, bool) {
			claims, err := sessionAuth.ParseAccessJWT(accessToken)
			if err != nil {
				return analytics.Actor{}, false
			}
			return analyticsActor(*claims)
		},
	})
}

// analyticsActor maps the token scope to the analytics surface and role, the
// same values the frontend registers. Role names are school data, so the OGS
// portal reports only the admin flag. Operator tokens are no portal.
func analyticsActor(claims projectJWT.AppClaims) (analytics.Actor, bool) {
	switch {
	case claims.IsPlatformScope():
		return analytics.Actor{}, false
	case claims.Scope == "parent":
		return analytics.Actor{Surface: analytics.SurfaceParents, Role: analytics.RoleGuardian}, true
	case claims.IsSchoolScope():
		return analytics.Actor{Surface: analytics.SurfaceSchool, Role: analytics.RoleLehrkraft, SchoolID: claims.TenantID}, true
	case claims.TenantID == 0:
		return analytics.Actor{}, false
	case claims.IsAdmin || claims.IsReadOnlyPreview():
		return analytics.Actor{Surface: analytics.SurfaceOGS, Role: analytics.RoleAdmin, SchoolID: claims.TenantID}, true
	default:
		return analytics.Actor{Surface: analytics.SurfaceOGS, Role: analytics.RoleStaff, SchoolID: claims.TenantID}, true
	}
}

// requireCoreActionClassification refuses to start a server with a writing
// portal route the usage analytics does not classify (#3602). The route
// table is static, so TestFullProductionRouterGolden, which builds the
// server, fails in CI long before a deploy could.
func requireCoreActionClassification(router chi.Routes) error {
	unclassified, _, err := coreActionRouteGaps(router)
	if err != nil {
		return fmt.Errorf("classify core actions: %w", err)
	}
	if len(unclassified) > 0 {
		return fmt.Errorf("writing routes without a core-action classification in backend/analytics/core_actions.go: %s", strings.Join(unclassified, ", "))
	}
	return nil
}

// coreActionRouteGaps is the route guard of the usage analytics (#3602): it
// lists the writing routes of the portal routers in router that the
// core-action table does not classify, and the table entries whose route is
// gone. The demo routes exist under APP_ENV=demo only, so the guard adds them
// from the demo resource itself, mounted where MountDemoRoutes mounts it.
func coreActionRouteGaps(router chi.Routes) (unclassified, stale []string, err error) {
	demo := chi.NewRouter()
	demo.Mount("/demo", (&authAPI.DemoResource{}).Router())

	routes := map[analytics.RouteKey]bool{}
	for _, source := range []chi.Routes{router, demo} {
		if err := collectWritingPortalRoutes(source, routes); err != nil {
			return nil, nil, err
		}
	}
	table := analytics.CoreActions()
	for route := range routes {
		if _, ok := table[route]; !ok {
			unclassified = append(unclassified, route.Method+" "+route.Pattern)
		}
	}
	for route := range table {
		if !routes[route] {
			stale = append(stale, route.Method+" "+route.Pattern)
		}
	}
	sort.Strings(unclassified)
	sort.Strings(stale)
	return unclassified, stale, nil
}

func collectWritingPortalRoutes(router chi.Routes, into map[analytics.RouteKey]bool) error {
	return chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return nil
		}
		if isCoreActionPortalRoute(route) {
			into[analytics.RouteKey{Method: method, Pattern: route}] = true
		}
		return nil
	})
}

func isCoreActionPortalRoute(route string) bool {
	for _, excluded := range analytics.CoreActionExcludedPrefixes() {
		if strings.HasPrefix(route, excluded) {
			return false
		}
	}
	for _, prefix := range analytics.CoreActionPrefixes() {
		if strings.HasPrefix(route, prefix) {
			return true
		}
	}
	return false
}
