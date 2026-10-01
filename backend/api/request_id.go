package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/observability"
)

// requestIDMiddleware installs the shared CorrelationID while preserving
// Chi's context contract for the existing logging stack.
func requestIDMiddleware(tracer *observability.Tracer, next http.Handler) http.Handler {
	return requestIDMiddlewareWithStartRequest(tracer, func(ctx context.Context, requestValue string) (context.Context, string, error) {
		ctx, id, err := tracer.StartRequest(ctx, requestValue)
		return ctx, id.String(), err
	}, next)
}

func requestIDMiddlewareWithStartRequest(tracer *observability.Tracer, startRequest func(context.Context, string) (context.Context, string, error), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, id, err := startRequest(r.Context(), r.Header.Get(middleware.RequestIDHeader))
		if err != nil {
			tracer.Failure(r.Context(), "http", "request-id", "generation_failure", err)
			// This middleware precedes the router-wide problem middleware, so its
			// own failure must pass through the same response adapter explicitly.
			apiCommon.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			})).ServeHTTP(w, r)
			return
		}
		w.Header().Set(middleware.RequestIDHeader, id)
		ctx = context.WithValue(ctx, middleware.RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
