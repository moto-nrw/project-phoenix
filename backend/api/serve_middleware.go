package api

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/observability"
	slogchi "github.com/samber/slog-chi"
	"github.com/spf13/viper"
)

func newRuntimeTracer(logger *slog.Logger) *observability.Tracer {
	return observability.NewTracer(logger, func(entryPoint, _ string, outcome string) {
		observability.RecordTenantRuntimeEvent(entryPoint, outcome)
	})
}

// setupBasicMiddleware configures basic router middleware
func setupBasicMiddleware(router chi.Router, logger *slog.Logger, httpMetrics *httpMetrics) {
	router.Use(middleware.ClientIPFromXFFTrustedProxies(1))
	router.Use(syncClientIPToRemoteAddr)
	if httpMetrics != nil {
		router.Use(httpMetrics.middleware)
	}
	// Redact public calendar- and request-feed tokens (the sole credential for
	// those feeds) from the per-request "path" attribute, and
	// strip query-string values (staff-UI searches carry student names and
	// e-mail addresses as query parameters, issue #2105) so neither lands in
	// access logs.
	requestLogger := slog.New(customMiddleware.NewQueryValueRedactor(
		customMiddleware.NewFeedTokenRedactor(logger.Handler())))
	router.Use(slogchi.NewWithConfig(requestLogger, slogchi.Config{
		DefaultLevel:     slog.LevelInfo,
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
		WithRequestID:    true,
		WithRequestBody:  false,
		WithResponseBody: false,
		WithSpanID:       false,
		WithTraceID:      false,
		Filters: []slogchi.Filter{
			slogchi.IgnorePath("/health"),
			// Bot/scanner probes use HTTP methods our API never serves
			// (CONNECT tunneling, TRACE/XST, PRI HTTP/2 preface). They get a
			// 404/405 but add only WARN-level log noise (issue #850). Drop the
			// log lines; the Prometheus HTTP middleware runs earlier in the
			// chain and still counts them, so volume-based scan alerting is
			// unaffected. Legitimate 4xx on GET/POST/... routes keep logging.
			slogchi.IgnoreMethod(http.MethodConnect, http.MethodTrace, "PRI"),
			keepAccessLogLine,
		},
	}))
	router.Use(middleware.Recoverer)
	// Inside the Recoverer: sentryhttp reports a panic and repanics to it, and
	// every 5xx answer becomes one Sentry event (#3639).
	router.Use(ServerErrorReporting)
	router.Use(customMiddleware.SecurityHeaders)
	// Request-scoped settings memo cache (issue #2065). Router-wide so routes
	// outside ProtectedTenantGroup (/auth incl. /auth/tenant/resolve,
	// /operator, /parent, public enrollment) dedupe their settings lookups
	// too. Idempotent with the group-wide attachment in api/common.
	router.Use(RequestSettingsCacheMiddleware)
	// Request-scoped identity memo cache (issue #2099). Router-wide so routes
	// outside ProtectedTenantGroup (notably /api/sse, which builds its own JWT
	// chain) dedupe their identity-chain lookups too.
	router.Use(RequestIdentityCacheMiddleware)
}

// keepAccessLogLine drops the access line of a webhook's 503, which would count
// as a server error. Only the missing secret answers 503 here, and that writes
// its own line; a failed store answers 500 and keeps its line.
func keepAccessLogLine(ww middleware.WrapResponseWriter, r *http.Request) bool {
	return ww.Status() != http.StatusServiceUnavailable || !strings.HasPrefix(r.URL.Path, emailWebhookPath+"/")
}

func syncClientIPToRemoteAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := middleware.GetClientIP(r.Context()); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}

func setupCORSIfEnabled(router chi.Router, enabled bool) {
	if enabled {
		setupCORS(router)
	}
}

// setupCORS configures CORS middleware with allowed origins from environment.
// Supports wildcard subdomain patterns like "*.example.com" via AllowOriginFunc.
func setupCORS(router chi.Router) {
	router.Use(corsHandler(os.Getenv("CORS_ALLOWED_ORIGINS")))
}

// corsHandler builds the CORS middleware for a CORS_ALLOWED_ORIGINS value.
func corsHandler(allowedOrigins string) func(http.Handler) http.Handler {
	exactOrigins, wildcardSuffixes := parseAllowedOrigins(allowedOrigins)

	opts := cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Staff-PIN", "X-Staff-ID", "X-Device-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}

	if len(wildcardSuffixes) > 0 {
		opts.AllowOriginFunc = buildCORSOriginFunc(exactOrigins, wildcardSuffixes)
	} else {
		opts.AllowedOrigins = exactOrigins
	}

	return cors.Handler(opts)
}

// buildCORSOriginFunc returns a CORS origin matcher that accepts any exact
// origin or any origin whose host ends in one of the wildcard suffixes
// (e.g. ".example.com" matches "https://school-a.example.com").
func buildCORSOriginFunc(exactOrigins, wildcardSuffixes []string) func(*http.Request, string) bool {
	// Build a set for O(1) exact-match lookups
	exactSet := make(map[string]bool, len(exactOrigins))
	for _, o := range exactOrigins {
		exactSet[o] = true
	}
	return func(_ *http.Request, origin string) bool {
		if exactSet[origin] {
			return true
		}
		return matchesWildcardSuffix(origin, wildcardSuffixes)
	}
}

// matchesWildcardSuffix reports whether the host portion of origin ends in one
// of the given suffixes (with at least one leading subdomain label).
func matchesWildcardSuffix(origin string, wildcardSuffixes []string) bool {
	// Extract the host portion of the origin (e.g. "https://school-a.example.com" → "school-a.example.com")
	host := origin
	if idx := strings.Index(origin, "://"); idx >= 0 {
		host = origin[idx+3:]
	}
	for _, suffix := range wildcardSuffixes {
		if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
			return true
		}
	}
	return false
}

// parseAllowedOrigins parses CORS_ALLOWED_ORIGINS and splits entries into
// exact-match origins and wildcard subdomain suffixes (e.g. "*.example.com"
// becomes suffix ".example.com").
func parseAllowedOrigins(originsEnv string) (exact []string, wildcardSuffixes []string) {
	if originsEnv == "" {
		return []string{"*"}, nil
	}

	for _, raw := range strings.Split(originsEnv, ",") {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			continue
		}
		if strings.HasPrefix(origin, "*.") {
			// "*.example.com" → match any subdomain of ".example.com"
			wildcardSuffixes = append(wildcardSuffixes, origin[1:]) // ".example.com"
		} else {
			exact = append(exact, origin)
		}
	}

	if len(exact) == 0 && len(wildcardSuffixes) == 0 {
		return []string{"*"}, nil
	}
	return exact, wildcardSuffixes
}

// securityLoggerFromEnv returns the security logger the rate limiters log
// their rejections to, nil unless SECURITY_LOGGING_ENABLED is set. The
// limiters are the only source of RATE_LIMIT_EXCEEDED events: one rejection,
// one event.
func securityLoggerFromEnv() *customMiddleware.SecurityLogger {
	if os.Getenv("SECURITY_LOGGING_ENABLED") != "true" {
		return nil
	}
	return customMiddleware.NewSecurityLogger()
}

// demoLoopbackExempt reports whether the rate limiters let loopback peers
// through. Only the public demo does: its demo process shares the server
// container's network namespace and drives more than a thousand API calls per
// seeded school under one account. Public requests never arrive from
// loopback (see customMiddleware.RateLimiter.ExemptLoopback).
func demoLoopbackExempt() bool {
	return IsDemoEnvironment(viper.GetString("app_env"))
}

// setupRateLimiting configures rate limiting middleware if enabled
// It runs after the session Verifier, whose parse the per-account key reads,
// and before the session family check, so a flood of signed tokens is limited
// before it reaches the database.
func setupRateLimiting(router chi.Router, securityLogger *customMiddleware.SecurityLogger, enabled, exemptLoopback bool) {
	if !enabled {
		return
	}

	generalLimit := parsePositiveInt(os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE"), 60)
	generalBurst := parsePositiveInt(os.Getenv("RATE_LIMIT_BURST"), 10)

	generalRateLimiter := customMiddleware.NewRateLimiter(generalLimit, generalBurst)
	generalRateLimiter.ExemptPathPrefix(emailWebhookPath + "/")
	generalRateLimiter.SetBucketFunc(func(r *http.Request) string {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return "read"
		default:
			return "write"
		}
	})
	generalRateLimiter.SetRejectObserver(observability.RecordRateLimitRejection)
	generalRateLimiter.SetKeyFunc(sessiontoken.RateLimitKey)
	if exemptLoopback {
		generalRateLimiter.ExemptLoopback()
	}
	if securityLogger != nil {
		generalRateLimiter.SetLogger(securityLogger)
	}
	router.Use(generalRateLimiter.Middleware())
}

// parsePositiveInt parses a positive integer from environment variable with a default value
func parsePositiveInt(valueStr string, defaultValue int) int {
	if valueStr == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(valueStr)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

// authRateLimiters bundles the auth-endpoint rate limiters. Each field is nil
// when rate limiting is disabled; separate instances give email-confirm and
// invitation endpoints independent per-IP counters so they can't exhaust each
// other's budget.
type authRateLimiters struct {
	auth         *customMiddleware.RateLimiter
	emailConfirm *customMiddleware.RateLimiter
	invitation   *customMiddleware.RateLimiter
}

// buildAuthRateLimiters constructs the stricter auth-endpoint rate limiters
// from RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE (default 5), wiring the security
// logger when present. exemptLoopback lets loopback peers through (demo only).
func buildAuthRateLimiters(securityLogger *customMiddleware.SecurityLogger, configuredLimit string, exemptLoopback bool) authRateLimiters {
	authLimit := 5 // default: 5 requests per minute for auth
	if limit := configuredLimit; limit != "" {
		if parsed, err := strconv.Atoi(limit); err == nil && parsed > 0 {
			authLimit = parsed
		}
	}
	// The burst matches one minute's budget, at least 10: a school's staff
	// share one public IP, so a morning sign-in wave (login plus MFA) arrives
	// at once.
	burst := max(10, authLimit)
	limiters := authRateLimiters{
		auth:         customMiddleware.NewRateLimiter(authLimit, burst),
		emailConfirm: customMiddleware.NewRateLimiter(authLimit, burst),
		invitation:   customMiddleware.NewRateLimiter(authLimit, burst),
	}
	for bucket, limiter := range map[string]*customMiddleware.RateLimiter{
		"auth": limiters.auth, "email-confirm": limiters.emailConfirm, "invitation": limiters.invitation,
	} {
		limiter.SetBucketFunc(func(*http.Request) string { return bucket })
		limiter.SetRejectObserver(observability.RecordRateLimitRejection)
		if exemptLoopback {
			limiter.ExemptLoopback()
		}
		if securityLogger != nil {
			limiter.SetLogger(securityLogger)
		}
	}
	return limiters
}

// authRateLimitersFromEnv builds the auth-endpoint limiters from the root
// configuration snapshot. Disabled limiters carry nil fields.
func authRateLimitersFromEnv(enabled bool) authRateLimiters {
	if !enabled {
		return authRateLimiters{}
	}
	return buildAuthRateLimiters(securityLoggerFromEnv(), os.Getenv("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE"), demoLoopbackExempt())
}

// authMiddleware is the auth limiter's middleware, nil while rate limiting is
// disabled. Tenant, operator, parent and school login share its budget.
func (limiters authRateLimiters) authMiddleware() func(http.Handler) http.Handler {
	if limiters.auth == nil {
		return nil
	}
	return limiters.auth.Middleware()
}

// A letter to a whole school brings a burst of provider events from a few IPs,
// far above the limit of one person. The webhook has its own limit for that.
const (
	emailWebhookRequestsPerMinute = 1200
	emailWebhookBurst             = 300
)

// emailWebhookRateLimit is nil when rate limiting is disabled.
func emailWebhookRateLimit(enabled bool) func(http.Handler) http.Handler {
	if !enabled {
		return nil
	}
	return newEmailWebhookRateLimit(securityLoggerFromEnv())
}

func newEmailWebhookRateLimit(securityLogger *customMiddleware.SecurityLogger) func(http.Handler) http.Handler {
	limiter := customMiddleware.NewRateLimiter(emailWebhookRequestsPerMinute, emailWebhookBurst)
	limiter.SetBucketFunc(func(*http.Request) string { return "webhook" })
	limiter.SetRejectObserver(observability.RecordRateLimitRejection)
	if securityLogger != nil {
		limiter.SetLogger(securityLogger)
	}
	return limiter.Middleware()
}
