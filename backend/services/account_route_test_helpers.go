package services

import (
	"context"
	"encoding/json"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	"github.com/uptrace/bun"
)

// StoreRawTenantSettingForTests stores a tenant override straight through the
// settings repository, without the registry's type and option checks, so a
// reader's test can pin how it meets a row that no longer matches the
// registry (#2736: the account-route tests may not name the repository).
func StoreRawTenantSettingForTests(ctx context.Context, db *bun.DB, tenantID int64, key string, raw json.RawMessage) error {
	repos := NewSettingsTestRepositories(settingsCompose.NewTenantSettingsRuntime(db, nil))
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
