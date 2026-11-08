package test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/uptrace/bun"
)

// MeasuredTenantRequest arranges the production school request transaction
// before measuring handler SQL. Only unit setup/teardown is outside the
// measurement; handler statements and the existing budget stay unchanged.
func MeasuredTenantRequest(tb testing.TB, db *bun.DB, tenantID int64, counter *QueryCounter, next http.Handler) http.Handler {
	tb.Helper()
	measured := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Start()
		defer counter.Stop()
		next.ServeHTTP(w, r)
	})
	transactional := TenantTxMiddleware(db)(measured)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Stop()
		ctx := WithTenantRuntime(tb, ContextForTenant(r.Context(), tenantID), db)
		transactional.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MeasuredHTTPRequest owns chi's context for this request so its completed
// pattern remains available after chi returns pooled contexts to its pool.
func MeasuredHTTPRequest(counter *QueryCounter, next http.Handler, w http.ResponseWriter, r *http.Request) {
	next.ServeHTTP(w, r.WithContext(counter.Context(r.Context())))
}

// MountedQueryBudgetRouter runs a resource-sized fixture at its production
// mount. Callers declare the mount explicitly; neither URLs nor descriptions
// are used to guess it. Relative fixture requests are cloned, not mutated.
func MountedQueryBudgetRouter(prefix string, resource http.Handler) chi.Router {
	router := chi.NewRouter()
	router.Mount(prefix, resource)
	return &mountedQueryBudgetRouter{Router: router, prefix: prefix}
}

type mountedQueryBudgetRouter struct {
	chi.Router
	prefix string
}

func (r *mountedQueryBudgetRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if chi.RouteContext(ctx) == nil {
		ctx = context.WithValue(ctx, chi.RouteCtxKey, chi.NewRouteContext())
	}
	cloned := req.Clone(ctx)
	cloned.URL.Path = r.prefix + req.URL.Path
	if req.URL.RawPath != "" {
		cloned.URL.RawPath = r.prefix + req.URL.RawPath
	}
	cloned.RequestURI = cloned.URL.RequestURI()
	r.Router.ServeHTTP(w, cloned)
}
