package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/moto-nrw/project-phoenix/observability"
)

// requestIDMiddleware installs the shared CorrelationID while preserving
// Chi's context contract for the existing logging stack.
func requestIDMiddleware(tracer *observability.Tracer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, id, err := tracer.StartRequest(r.Context(), r.Header.Get(middleware.RequestIDHeader))
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set(middleware.RequestIDHeader, id.String())
		ctx = context.WithValue(ctx, middleware.RequestIDKey, id.String())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
