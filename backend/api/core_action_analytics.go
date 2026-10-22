package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/analytics"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/spf13/viper"
)

// newProductAnalytics builds the process's product analytics tracker
// (PostHog); it is a no-op without POSTHOG_API_KEY. The root owns its
// lifetime and closes it on shutdown.
func newProductAnalytics(logger *slog.Logger) (analytics.Tracker, error) {
	return analytics.New(
		viper.GetString("posthog_api_key"),
		viper.GetString("posthog_host"),
		analyticsDeployment(viper.GetString("app_env"), viper.GetString("tenant_domain")),
		logger.With("component", "analytics"),
		// The pseudonymous user ID under Analyse-Freigabe (#3603) hashes with
		// the Security Runtime's SHA-256 fingerprint, like the browser.
		analytics.WithFingerprint(securityruntime.Fingerprint),
	)
}

// analyticsDeployment binds the deployment property to the environment.
func analyticsDeployment(appEnv, tenantDomain string) string {
	return analytics.Deployment(IsDemoEnvironment(appEnv), tenantDomain)
}

// analyticsSchoolNames binds the Worker's school names (the usage
// dashboards) to the tracker.
func analyticsSchoolNames(tracker analytics.Tracker) owners.SchoolNamesPublisher {
	return func(schools []organizationModule.School) int {
		named := make([]analytics.School, 0, len(schools))
		for _, school := range schools {
			named = append(named, analytics.School{ID: school.ID, Name: school.Name})
		}
		return analytics.IdentifySchools(tracker, named)
	}
}

// coreActionAnalytics binds the core-action middleware (#3602) to the
// session model: the request's verified session, or the session a login
// response mints. The root mounts it after the session verifier. The school's
// Analyse-Freigabe (#3603, settings compose NewAnalyseFreigabe) decides
// whether an OGS account is a pseudonymous person.
func coreActionAnalytics(tracker analytics.Tracker, sessionAuth *sessiontoken.TokenAuth, analyseFreigabe func(ctx context.Context, schoolID int64) bool) func(http.Handler) http.Handler {
	return analytics.CoreActionMiddleware(analytics.CoreActionConfig{
		Tracker:         tracker,
		AnalyseFreigabe: analyseFreigabe,
		RequestActor: func(r *http.Request) (analytics.Actor, bool) {
			// The root verifier leaves a token only when its signature and
			// expiry hold; ParseClaims rejects MFA interim tokens.
			raw, ok := analytics.VerifiedSessionClaims(r)
			if !ok {
				return analytics.Actor{}, false
			}
			var claims sessiontoken.AppClaims
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

// analyticsActor hands the session facts of the token to the analytics
// rule.
func analyticsActor(claims sessiontoken.AppClaims) (analytics.Actor, bool) {
	return analytics.ActorOf(analytics.SessionFacts{
		Platform: claims.IsPlatformScope(), Parent: claims.IsParentScope(), School: claims.IsSchoolScope(),
		TenantID: claims.TenantID, ReadOnlyPreview: claims.IsReadOnlyPreview(), Admin: claims.IsAdmin, AccountID: int64(claims.ID),
	})
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
		if analytics.IsWritingMethod(method) && isCoreActionPortalRoute(route) {
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
