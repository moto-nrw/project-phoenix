package common

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// NewReadinessHandler checks the server's database pool without reading tenant
// data. The caller binds the pool ping; dependency details never reach the wire
// or the shared error logger.
func NewReadinessHandler(ping func(context.Context) error) http.Handler {
	if ping == nil {
		panic("readiness requires a database ping")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			RenderError(w, r, ErrorServiceUnavailable(errors.New("service unavailable")))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})
}
