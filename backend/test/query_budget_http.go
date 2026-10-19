package test

import (
	"net/http"
	"testing"

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
