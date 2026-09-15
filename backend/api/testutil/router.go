package testutil

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Router is the router shape NewJSONRouter already returns. Aliased so a route
// test names it through this boundary instead of the router library.
type Router = chi.Router

// NewRouter is a bare router, for a test that wants no JSON middleware.
func NewRouter() Router { return chi.NewRouter() }

// WithURLParams returns the request with the given path parameters attached
// the way the router would have, for a handler exercised directly instead of
// through a mounted route. Pairs are key then value.
//
// It exists because doing this by hand means knowing that the parameters ride
// on a route context under a library-private context key, which is the router's
// business and not a route test's.
func WithURLParams(req *http.Request, pairs ...string) *http.Request {
	if len(pairs)%2 != 0 {
		panic("testutil: WithURLParams needs key/value pairs")
	}
	rctx := chi.NewRouteContext()
	for i := 0; i < len(pairs); i += 2 {
		rctx.URLParams.Add(pairs[i], pairs[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
