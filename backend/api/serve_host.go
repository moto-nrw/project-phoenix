package api

import (
	"log/slog"
	"net/http"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/analytics"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// productionObservers record every owner operation the roots observe.
func productionObservers() owners.RootObservers {
	return owners.RootObservers{
		Communication: observeCommunication, AuditAppend: observability.ObserveAuditAppend,
		Delivery: observability.ObserveSynchronousDelivery, DurableDelivery: observability.ObserveDurableDelivery,
		DeviceFleet: observability.ObserveDeviceFleetOperation, IdentityAccess: observability.ObserveIdentityAccessOperation,
		FileStorage: observeFileStorage, SettingsLookup: observability.ObserveSettingsLookup,
		SettingsSideEffectFailure: observability.RecordSettingsSideEffectFailure, CarePlan: observeCarePlan,
	}
}

// newProductionEnvironment reads the process configuration the owners are
// built over; the services keep the real clock.
func newProductionEnvironment(logger *slog.Logger, publicAPIURL string) (owners.RootEnvironment, error) {
	cfg := CurrentServiceConfig()
	cfg.PublicAPIURL = publicAPIURL
	return newRootEnvironment(cfg, logger, nil, productionObservers())
}

// mountServeMiddleware sets up the global middleware chain of the server
// root in its fixed order and returns the session runtime it verifies with.
func mountServeMiddleware(
	router chi.Router, unit tenant.UnitOfWork, logger *slog.Logger, config ServeConfig, tracer *observability.Tracer,
	metrics *httpMetrics, tracker analytics.Tracker, settings settingsCompose.SettingsService,
) (*sessiontoken.TokenAuth, error) {
	router.Use(requestIDs(tracer))
	router.Use(apiCommon.ProblemResponseMiddleware)
	// chi answers an unknown path with a plain-text body; the shared envelope
	// replaces it, so ProblemResponseMiddleware only repairs real handler
	// regressions. chi's 405 has no body and keeps its Allow header.
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		apiCommon.RenderStatus(w, r, http.StatusNotFound, "")
	})
	router.Use(apiCommon.TenantRuntimeMiddleware(unit))
	router.Use(AuthorizationObserverMiddleware(recordAuthorizationEvent))
	router.Use(apiCommon.TenantRuntimeObserverMiddleware(func(observation apiCommon.TenantRuntimeObservation) {
		recordHTTPRuntimeEvent(tracer, observation)
	}))
	router.Use(apiCommon.TenantRequestObserverMiddleware(func(event apiCommon.TenantRequestEvent) {
		observability.ObserveTenantRequest(
			event.TenantID,
			event.Scope,
			event.Request.Method,
			apiCommon.RoutePattern(event.Request),
			event.Status,
			event.Duration,
			event.Outcome,
		)
	}))
	setupBasicMiddleware(router, logger, metrics)

	// Setup CORS, security logging, and rate limiting
	setupCORSIfEnabled(router, config.EnableCORS)
	securityLogger := setupSecurityLogging(router)
	sessionAuth, err := newSessionTokenAuth()
	if err != nil {
		return nil, err
	}
	setupRateLimiting(router, securityLogger, sessionAuth, demoLoopbackExempt())
	// One verifier serves every route: it only parses the presented token, and
	// each protected group still rejects through the Authenticator and its
	// scope gate. A group mounted without it fails closed.
	router.Use(sessionAuth.Verifier())
	// Sentry events name the session they affect (#3643).
	router.Use(SentrySessionContext)
	// Core actions of the portals reach the usage analytics once their
	// response is 2xx (#3602). After the verifier, which names the session.
	router.Use(coreActionAnalytics(tracker, sessionAuth, settingsCompose.NewAnalyseFreigabe(settings, logger)))
	return sessionAuth, nil
}

// registerRuntimeStatsProviders hands the live streams and the PWA usage to
// the metrics endpoint; the shared process setup registers the pool.
func registerRuntimeStatsProviders(hub *realtime.Hub, usage deliveryCompose.PWAUsageService) {
	observability.RegisterSSEStatsProvider(hub)
	observability.RegisterPWAUsageStatsProvider(observability.PWAUsageStatsProviderFunc(func() ([]observability.PWAUsageStat, error) {
		rows, err := usage.SnapshotUsage()
		if err != nil {
			return nil, err
		}
		stats := make([]observability.PWAUsageStat, 0, len(rows))
		for _, row := range rows {
			stats = append(stats, observability.PWAUsageStat{
				TenantID:        row.TenantID,
				Portal:          row.Portal,
				StandaloneUsers: row.StandaloneUsers,
				EligibleUsers:   row.EligibleUsers,
			})
		}
		return stats, nil
	}))
}

// serveHandler is the server root's router behind the CalDAV method
// normalization and the weekend plan of the tenant settings.
type serveHandler struct {
	router   chi.Router
	settings settingsCompose.SettingsService
}

// ServeHTTP routes a request through the composed router. CalDAV extension
// methods reach their handler under a routable stand-in method; a weekend
// that follows Friday's plan reads the tenant setting only for Saturday and
// Sunday dates (#3921).
func (handler *serveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = normalizeCalDAVMethodForRouting(r)
	r = WithWeekendPlan(r, handler.settings)
	handler.router.ServeHTTP(w, r)
}
