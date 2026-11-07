//go:build testhooks

package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// StoreRawTenantSettingForTests stores a tenant override straight through the
// settings repository, without the registry's type and option checks, so a
// reader's test can pin how it meets a row that no longer matches the
// registry (#2736: the account-route tests may not name the repository).
func StoreRawTenantSettingForTests(ctx context.Context, db transactionruntime.Database, tenantID int64, key string, raw json.RawMessage) error {
	unit := db.UnitOfWork()
	runtime := settingsCompose.NewTenantSettingsRuntime(db.Pool(), &unit)
	repos := owners.NewSettingsTestRepositories(runtime)
	value := &configModels.SettingValue{SettingKey: key, Value: raw}
	value.SetTenantID(tenantID)
	return runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error { return repos.Values.Upsert(txCtx, value) })
}

// WithSettingsRequestCacheForTests attaches the request-scoped settings memo
// cache production requests run with, for tests that mount a route without
// the root router.
func WithSettingsRequestCacheForTests(ctx context.Context) context.Context {
	return settingsCompose.WithSettingsRequestCache(ctx)
}

// SettingsRequestCacheMiddleware attaches the request-scoped settings memo
// cache the root router attaches to every request.
func SettingsRequestCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithSettingsRequestCacheForTests(r.Context())))
	})
}

// TenantRequestEventForTests exposes only the request outcome fields owner
// tests observe, without importing the Tenant Runtime HTTP implementation.
type TenantRequestEventForTests struct {
	Scope    string
	TenantID int64
	Status   int
	Outcome  string
}

// NewTenantRequestObservedRouterForTests keeps the actual request observer
// around a route-sized test router, with the same middleware/mount order.
func NewTenantRequestObservedRouterForTests(next http.Handler, observe func(TenantRequestEventForTests)) chi.Router {
	router := chi.NewRouter()
	router.Use(common.TenantRequestObserverMiddleware(func(event common.TenantRequestEvent) {
		observe(TenantRequestEventForTests{Scope: event.Scope, TenantID: event.TenantID, Status: event.Status, Outcome: event.Outcome})
	}))
	router.Mount("/", next)
	return router
}
