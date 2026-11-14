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
	customMiddleware "github.com/moto-ogs/project-phoenix/middleware"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/observability"
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

	generalLimit, generalBurst := generalRateLimitFromEnv()

	generalRateLimiter := customMiddleware.NewRateLimiter(generalLimit, generalBurst)
	generalRateLimiter.ExemptPathPrefix(emailWebhookPath + "/")
	generalRateLimiter.ExemptPathPrefix(holidayRegistrationPath + "/")
	generalRateLimiter.ExemptPathPrefix(holidayQuoteMount + "/")
	generalRateLimiter.ExemptRequest(isPublicEnrollmentRequest)
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
	router.Use(limitOnly(isPublicEnrollmentRequest, publicLinkRateLimit("enrollment-link", exemptLoopback)))
	router.Use(generalRateLimiter.Middleware())
}

// limitOnly applies limit to the requests match selects and passes every
// other request through untouched.
func limitOnly(match func(*http.Request) bool, limit func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		limited := limit(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if match(r) {
				limited.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// enrollmentPublicRoutes are the parent-facing routes of the school's online
// enrollment below /api/enrollment.
var enrollmentPublicRoutes = enrollmentAPI.PublicRoutes()

// isPublicEnrollmentRequest reports whether r reaches a parent-facing route
// of the school's online enrollment. The rate limiters run before routing,
// so they match the path against the enrollment's own route table.
func isPublicEnrollmentRequest(r *http.Request) bool {
	path, ok := strings.CutPrefix(r.URL.Path, "/api"+enrollmentMount)
	if !ok || !strings.HasPrefix(path, "/") {
		return false
	}
	return enrollmentPublicRoutes.Match(chi.NewRouteContext(), r.Method, path)
}

// generalRateLimitFromEnv is the configured budget of one caller of the
// general limiter per minute and its burst.
func generalRateLimitFromEnv() (requestsPerMinute, burst int) {
	return parsePositiveInt(os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE"), 60),
		parsePositiveInt(os.Getenv("RATE_LIMIT_BURST"), 10)
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
//
// The login limiter spends its tokens on refused attempts and on mails only
// (#6738, docs/agents/contracts.md "Client IP and rate limits"), so the many
// accepted logins behind one school address do not use them up.
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
	limiters.auth.SpendOnRefusals(sendsAuthMail)
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

// authMailPathSuffixes end the auth-limited routes that mail a password-reset
// link or a code under every portal: the reset request, the resend of a
// sign-in code, and the caterer's start of a second factor or a passkey
// enrollment, and the Träger's code step in a running session (#7068). The
// confirmation of a reset sends none.
var authMailPathSuffixes = []string{"/password-reset", "/mfa/resend", "/account/mfa/start", "/passkeys/enrollment/challenge", "/mfa/step-up"}

// sendsAuthMail reports whether an auth-limited request asks for a mail.
func sendsAuthMail(r *http.Request) bool {
	for _, suffix := range authMailPathSuffixes {
		if strings.HasSuffix(r.URL.Path, suffix) {
			return true
		}
	}
	return false
}

// authRateLimitersFromEnv builds the auth-endpoint limiters from the root
// configuration snapshot. Disabled limiters carry nil fields.
func authRateLimitersFromEnv(enabled bool) authRateLimiters {
	if !enabled {
		return authRateLimiters{}
	}
	return buildAuthRateLimiters(securityLoggerFromEnv(), os.Getenv("RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE"), demoLoopbackExempt())
}

// enabled reports whether rate limiting is switched on.
func (limiters authRateLimiters) enabled() bool { return limiters.auth != nil }

// authMiddleware is the auth limiter's middleware, nil while rate limiting is
// disabled. Tenant, operator, parent and school login share its budget of
// refused attempts per address.
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

// The retrieval of a return package (ADR 0203) is a few requests per
// recipient; a tight limit per address stops token guessing and repeated
// large downloads alike.
const (
	dataReturnRequestsPerMinute = 30
	dataReturnBurst             = 10
)

// A public link of a school or a Träger, the holiday registration (#6726)
// and the school's online enrollment (#6739), is opened by many families
// from one address at once: the school network, a parents' evening, a mobile
// carrier. One family needs a form, its texts, its proofs and the
// registration; the limit of one person would refuse the families behind it.
// Each link has its own limit for that, per address and split into reads and
// writes like the general one. Turnstile and the stored-registration limits
// of Enrollment stay in front of every write.
//
// These values are a floor. The links are exempt from the general limiter,
// so their own limit must never be tighter than the general one an
// environment configures: publicLinkLimits takes the larger of both.
const (
	publicLinkRequestsPerMinute = 1200
	publicLinkBurst             = 300
)

// publicLinkLimits is the budget of a public link beside a general limiter
// of generalLimit and generalBurst.
func publicLinkLimits(generalLimit, generalBurst int) (requestsPerMinute, burst int) {
	return max(publicLinkRequestsPerMinute, generalLimit), max(publicLinkBurst, generalBurst)
}

// publicLinkRateLimit is the limiter of one public link; link names its
// buckets in the rejection metric.
func publicLinkRateLimit(link string, exemptLoopback bool) func(http.Handler) http.Handler {
	limiter := customMiddleware.NewRateLimiter(publicLinkLimits(generalRateLimitFromEnv()))
	limiter.SetBucketFunc(func(r *http.Request) string {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return link + "-read"
		default:
			return link + "-write"
		}
	})
	limiter.SetRejectObserver(observability.RecordRateLimitRejection)
	if exemptLoopback {
		limiter.ExemptLoopback()
	}
	if securityLogger := securityLoggerFromEnv(); securityLogger != nil {
		limiter.SetLogger(securityLogger)
	}
	return limiter.Middleware()
}

// holidayRegistrationRateLimit is nil when rate limiting is disabled.
func holidayRegistrationRateLimit(enabled, exemptLoopback bool) func(http.Handler) http.Handler {
	if !enabled {
		return nil
	}
	return publicLinkRateLimit("holiday-link", exemptLoopback)
}

// dataReturnRateLimit is nil when rate limiting is disabled.
func dataReturnRateLimit(enabled bool) func(http.Handler) http.Handler {
	if !enabled {
		return nil
	}
	limiter := customMiddleware.NewRateLimiter(dataReturnRequestsPerMinute, dataReturnBurst)
	limiter.SetBucketFunc(func(*http.Request) string { return "data-return" })
	limiter.SetRejectObserver(observability.RecordRateLimitRejection)
	if securityLogger := securityLoggerFromEnv(); securityLogger != nil {
		limiter.SetLogger(securityLogger)
	}
	return limiter.Middleware()
}

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
