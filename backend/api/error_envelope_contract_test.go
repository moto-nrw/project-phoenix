package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
)

// checkErrorEnvelopeOnEveryRoute fires an unauthenticated request with a
// malformed JSON body at every route of the production router and requires
// every failure to arrive in the one shared error envelope (ADR 0006, #2507):
// tenant, parent, school and operator portals, IoT, CalDAV and the public
// routes alike. Handler-level bodies are pinned by each package's wire tests;
// this walk proves no route leaves the router past ProblemResponseMiddleware.
func checkErrorEnvelopeOnEveryRoute(t *testing.T, apiInstance *serveGraph) {
	t.Parallel()
	const requestID = "2507e0e0-0000-4000-8000-000000002507"

	var routes []string
	walkErr := chi.Walk(apiInstance.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	})
	require.NoError(t, walkErr)
	sort.Strings(routes)

	failures := 0
	var violations []string
	for _, methodRoute := range routes {
		method, pattern, _ := strings.Cut(methodRoute, " ")
		probePath := chiParamPattern.ReplaceAllString(pattern, "1")
		probePath = strings.ReplaceAll(probePath, "*", "x")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req := httptest.NewRequestWithContext(ctx, method, probePath, strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.RequestIDHeader, requestID)
		rec := httptest.NewRecorder()
		apiInstance.router.ServeHTTP(rec, req)
		cancel()

		if rec.Code < http.StatusBadRequest {
			continue
		}
		failures++
		problems := routetest.ProblemEnvelopeViolations(rec.Body.Bytes(), true)
		if contentType := rec.Header().Get("Content-Type"); contentType != "application/problem+json" {
			problems = append(problems, "content type "+contentType)
		}
		if len(problems) > 0 {
			violations = append(violations, fmt.Sprintf("%s -> %d: %s", methodRoute, rec.Code, strings.Join(problems, "; ")))
		}
	}

	require.Empty(t, violations, "every error response must use the shared error envelope")
	// The walk is only evidence while it reaches the routes: nearly every
	// route rejects an anonymous, malformed request.
	require.Greater(t, failures, len(routes)*9/10, "too few routes answered with an error; the probe no longer reaches them")
}
