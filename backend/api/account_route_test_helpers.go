//go:build testhooks

package api

import (
	"context"
	"encoding/json"
	"net/http"

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
	repos := owners.NewSettingsTestRepositories(settingsCompose.NewTenantSettingsRuntime(db.Pool(), nil))
	value := &configModels.SettingValue{SettingKey: key, Value: raw}
	value.SetTenantID(tenantID)
	return repos.Values.Upsert(ctx, value)
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
