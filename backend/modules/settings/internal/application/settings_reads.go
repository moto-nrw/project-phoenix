package config

import (
	"context"
	"fmt"
	"time"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Resolve returns the value for a setting: tenant override if it exists,
// otherwise the registry default.
func (s *settingsService) Resolve(ctx context.Context, key string) (any, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return nil, err
	}
	return snapshot.Value(key)
}

// ResolveMany resolves several settings in one repository query. A compatible
// context snapshot wins, which lets scheduler-invoked downstream services
// share the minute snapshot without knowing about scheduler internals. Below
// the explicit snapshot sits the request-scoped memo cache (issue #2065):
// within one request every (tenant_id, key) pair is loaded from PostgreSQL at
// most once; only cache-missing keys reach the repository.
func (s *settingsService) ResolveMany(ctx context.Context, keys []string) (result *config.SettingsSnapshot, err error) {
	started := time.Now()
	cacheState := make(map[string]string, len(keys))
	for _, key := range keys {
		cacheState[key] = "bypass"
	}
	defer func() {
		s.observeLookups(keys, cacheState, time.Since(started), err)
	}()

	tenantID := s.tenantID(ctx)
	if snapshot := config.SnapshotFromContext(ctx, tenantID, keys); snapshot != nil {
		// Explicit snapshots are returned as-is and deliberately NEVER copied
		// into the request cache: the scheduler's minute snapshot may be up to
		// a minute old, and promoting its values into request scope would
		// leak that staleness into paths that expect request freshness.
		for _, key := range keys {
			cacheState[key] = "snapshot"
		}
		return snapshot, nil
	}

	if tenantID <= 0 || len(keys) == 0 {
		return config.NewSettingsSnapshot(s.registry, tenantID, keys, nil)
	}

	// Validate every key up front so an unknown key fails deterministically
	// regardless of which keys happen to be cached already.
	if err := s.validateSnapshotKeys(keys); err != nil {
		return nil, err
	}

	return s.resolveCachedSettings(ctx, tenantID, keys, cacheState)
}

func (s *settingsService) observeLookups(keys []string, cacheState map[string]string, duration time.Duration, err error) {
	if s.observeLookup == nil {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	for _, key := range keys {
		s.observeLookup(key, cacheState[key], outcome, duration)
	}
}

// ResolveManyForTenant resolves several settings inside one tenant
// transaction. A matching context snapshot avoids both the transaction and
// query when a scheduler already prefetched the values, and a full
// request-cache hit skips the tenant transaction entirely (issue #2065).
func (s *settingsService) ResolveManyForTenant(ctx context.Context, tenantID int64, keys []string) (*config.SettingsSnapshot, error) {
	if snapshot := config.SnapshotFromContext(ctx, tenantID, keys); snapshot != nil {
		s.observeLookups(keys, lookupState(keys, "snapshot"), 0, nil)
		return snapshot, nil
	}

	// Consult the request cache only when the ambient tenant context is absent
	// or matches tenantID: a mismatched ambient tenant must still reach
	// WithTenantTx below so its nested-transaction guard surfaces the wiring
	// bug instead of the cache silently succeeding.
	cached, err := s.cachedTenantSnapshot(ctx, tenantID, keys)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		started := time.Now()
		s.observeLookups(keys, lookupState(keys, "hit"), time.Since(started), nil)
		return cached, nil
	}

	if s.runtime == nil {
		return nil, config.ErrRuntimeUnavailable
	}
	var result *config.SettingsSnapshot
	err = s.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		var resolveErr error
		result, resolveErr = s.ResolveMany(txCtx, keys)
		return resolveErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func lookupState(keys []string, state string) map[string]string {
	states := make(map[string]string, len(keys))
	for _, key := range keys {
		states[key] = state
	}
	return states
}

// ResolveManyForTenants resolves all tenant/key pairs in one privileged query.
// Registry defaults are filled independently for every requested tenant.
// Deliberately NOT request-cached: its only callers are the scheduler (which
// owns the minute snapshot) and other admin-tx batch paths that already
// coalesce their reads.
func (s *settingsService) ResolveManyForTenants(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]*config.SettingsSnapshot, error) {
	uniqueTenantIDs, seenTenantIDs := uniqueSettingsTenants(tenantIDs)

	result := make(map[int64]*config.SettingsSnapshot, len(uniqueTenantIDs))
	if len(uniqueTenantIDs) == 0 {
		return result, nil
	}

	var stored []*config.SettingValue
	if len(keys) > 0 {
		if s.runtime == nil {
			return nil, config.ErrRuntimeUnavailable
		}
		err := s.runtime.WithinAdmin(ctx, func(txCtx context.Context) error {
			var findErr error
			stored, findErr = s.valueRepo.FindByTenantsAndKeys(txCtx, uniqueTenantIDs, keys)
			return findErr
		})
		if err != nil {
			return nil, &config.SettingsError{Op: "resolve_many_for_tenants", Err: err}
		}
	}

	storedByTenant := settingsRowsByTenant(stored, seenTenantIDs)
	for _, tenantID := range uniqueTenantIDs {
		snapshot, err := config.NewSettingsSnapshot(s.registry, tenantID, keys, storedByTenant[tenantID])
		if err != nil {
			return nil, err
		}
		result[tenantID] = snapshot
	}
	return result, nil
}

func (s *settingsService) EnrollmentEnabledForTenants(ctx context.Context, tenantIDs []int64) (map[int64]bool, error) {
	snapshots, err := s.ResolveManyForTenants(ctx, tenantIDs, []string{config.KeyEnrollmentEnabled})
	if err != nil {
		return nil, err
	}

	result := make(map[int64]bool, len(snapshots))
	for tenantID, snapshot := range snapshots {
		enabled, resolveErr := snapshot.Bool(config.KeyEnrollmentEnabled)
		if resolveErr != nil {
			return nil, resolveErr
		}
		result[tenantID] = enabled
	}
	return result, nil
}

// ResolveStringForTenant resolves a setting as a string for a specific tenant,
// wrapping the query in a tenant transaction to satisfy RLS.
func (s *settingsService) ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error) {
	snapshot, err := s.ResolveManyForTenant(ctx, tenantID, []string{key})
	if err != nil {
		return "", err
	}
	return snapshot.String(key)
}

// ResolveStringForTenantInTx resolves a setting as a string on the caller's
// already-open transaction, deliberately consulting neither the context
// snapshot nor the request-scoped memo cache — see the interface doc for why
// that matters and who needs it.
//
// It opens no transaction of its own: the repository read runs on the ambient
// tx via base.GetDB, so the value is the committed state seen by a statement
// of THAT transaction, and the caller's locks still hold when it is used. The
// alternative — resolving through ResolveManyForTenant, which opens its own
// transaction — would take a second pooled connection while the caller holds
// one, which is a deadlock waiting for a saturated pool.
func (s *settingsService) ResolveStringForTenantInTx(ctx context.Context, tenantID int64, key string) (string, error) {
	snapshot, err := s.resolveForTenantInTx(ctx, tenantID, key)
	if err != nil {
		return "", err
	}
	return snapshot.String(key)
}

// ResolveBoolForTenantInTx resolves a bool setting on the caller's already-open
// transaction. Like its string counterpart, it bypasses snapshots and the
// request cache so the value comes from the transaction the caller holds.
func (s *settingsService) ResolveBoolForTenantInTx(ctx context.Context, tenantID int64, key string) (bool, error) {
	snapshot, err := s.resolveForTenantInTx(ctx, tenantID, key)
	if err != nil {
		return false, err
	}
	return snapshot.Bool(key)
}

func (s *settingsService) resolveForTenantInTx(ctx context.Context, tenantID int64, key string) (*config.SettingsSnapshot, error) {
	if tenantID <= 0 {
		return nil, &config.SettingsError{
			Op:  "resolve_in_tx",
			Err: fmt.Errorf("tenant id is required to resolve %q inside a transaction", key),
		}
	}
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return nil, &config.SettingsError{
			Op:  "resolve_in_tx",
			Err: fmt.Errorf("resolving %q inside a transaction requires an ambient transaction", key),
		}
	}
	if s.registry.GetDefinition(key) == nil {
		return nil, &config.SettingsError{Op: "resolve_in_tx", Err: &config.DefinitionNotFoundError{Key: key}}
	}

	stored, err := s.valueRepo.FindByTenantAndKeys(ctx, tenantID, []string{key})
	if err != nil {
		return nil, &config.SettingsError{Op: "resolve_in_tx", Err: err}
	}
	snapshot, err := config.NewSettingsSnapshot(s.registry, tenantID, []string{key}, stored)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// ResolveBoolForTenant resolves a setting as a bool for a specific tenant,
// wrapping the query in a tenant transaction to satisfy RLS. Required from
// call sites that run outside TenantTxMiddleware (e.g. /auth/mfa/verify)
// but already know which tenant they're acting on.
func (s *settingsService) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	snapshot, err := s.ResolveManyForTenant(ctx, tenantID, []string{key})
	if err != nil {
		return false, err
	}
	return snapshot.Bool(key)
}

// ResolveIntForTenant mirrors ResolveBoolForTenant but for integer settings.
func (s *settingsService) ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error) {
	snapshot, err := s.ResolveManyForTenant(ctx, tenantID, []string{key})
	if err != nil {
		return 0, err
	}
	return snapshot.Int(key)
}

// ResolveString resolves a setting as a string.
func (s *settingsService) ResolveString(ctx context.Context, key string) (string, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return "", err
	}
	return snapshot.String(key)
}

// ResolveBool resolves a setting as a bool.
func (s *settingsService) ResolveBool(ctx context.Context, key string) (bool, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return false, err
	}
	return snapshot.Bool(key)
}

// ResolveBools resolves multiple boolean settings while loading all tenant
// overrides in one query. Registry defaults are filled first, then explicit
// tenant values replace them.
func (s *settingsService) ResolveBools(ctx context.Context, keys []string) (map[string]bool, error) {
	snapshot, err := s.ResolveMany(ctx, keys)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]bool, len(keys))
	for _, key := range keys {
		if _, seen := resolved[key]; seen {
			continue
		}
		value, resolveErr := snapshot.Bool(key)
		if resolveErr != nil {
			return nil, resolveErr
		}
		resolved[key] = value
	}
	return resolved, nil
}

// ResolveInt resolves a setting as an int.
func (s *settingsService) ResolveInt(ctx context.Context, key string) (int, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return 0, err
	}
	return snapshot.Int(key)
}

// HasTenantOverride checks if a tenant has an explicit DB override for a setting.
func (s *settingsService) HasTenantOverride(ctx context.Context, key string) (bool, error) {
	snapshot, err := s.ResolveMany(ctx, []string{key})
	if err != nil {
		return false, err
	}
	return snapshot.HasOverride(key)
}

func (s *settingsService) validateSnapshotKeys(keys []string) error {
	for _, key := range keys {
		if s.registry.GetDefinition(key) == nil {
			return &config.SettingsError{
				Op:  "resolve_many",
				Err: &config.DefinitionNotFoundError{Key: key},
			}
		}
	}
	return nil
}

func (s *settingsService) resolveCachedSettings(ctx context.Context, tenantID int64, keys []string, cacheState map[string]string) (*config.SettingsSnapshot, error) {
	cache := config.RequestCacheFromContext(ctx)
	if cache == nil {
		stored, err := s.valueRepo.FindByTenantAndKeys(ctx, tenantID, keys)
		if err != nil {
			return nil, &config.SettingsError{Op: "resolve_many", Err: err}
		}
		return config.NewSettingsSnapshot(s.registry, tenantID, keys, stored)
	}

	resolved, missing := cache.Lookup(tenantID, keys)
	for _, key := range keys {
		cacheState[key] = "hit"
	}
	for _, key := range missing {
		cacheState[key] = "miss"
	}
	if len(missing) > 0 {
		stored, err := s.valueRepo.FindByTenantAndKeys(ctx, tenantID, missing)
		if err != nil {
			return nil, &config.SettingsError{Op: "resolve_many", Err: err}
		}
		loaded, err := config.NewSettingsSnapshot(s.registry, tenantID, missing, stored)
		if err != nil {
			return nil, err
		}
		cache.StoreSnapshot(tenantID, loaded)
		resolved = config.MergeSettingsSnapshots(resolved, loaded)
	}
	return resolved, nil
}

func (s *settingsService) cachedTenantSnapshot(ctx context.Context, tenantID int64, keys []string) (*config.SettingsSnapshot, error) {
	if ctxTenant := s.tenantID(ctx); tenantID > 0 && len(keys) > 0 &&
		(ctxTenant == 0 || ctxTenant == tenantID) {
		if cache := config.RequestCacheFromContext(ctx); cache != nil {
			if err := s.validateSnapshotKeys(keys); err != nil {
				return nil, err
			}
			if resolved, missing := cache.Lookup(tenantID, keys); len(missing) == 0 {
				return resolved, nil
			}
		}
	}
	return nil, nil
}

func uniqueSettingsTenants(tenantIDs []int64) ([]int64, map[int64]struct{}) {
	uniqueTenantIDs := make([]int64, 0, len(tenantIDs))
	seenTenantIDs := make(map[int64]struct{}, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		if tenantID <= 0 {
			continue
		}
		if _, seen := seenTenantIDs[tenantID]; seen {
			continue
		}
		seenTenantIDs[tenantID] = struct{}{}
		uniqueTenantIDs = append(uniqueTenantIDs, tenantID)
	}
	return uniqueTenantIDs, seenTenantIDs
}

func settingsRowsByTenant(stored []*config.SettingValue, seenTenantIDs map[int64]struct{}) map[int64][]*config.SettingValue {
	storedByTenant := make(map[int64][]*config.SettingValue, len(seenTenantIDs))
	for _, value := range stored {
		if value == nil {
			continue
		}
		tenantID := value.GetTenantID()
		if _, requested := seenTenantIDs[tenantID]; requested {
			storedByTenant[tenantID] = append(storedByTenant[tenantID], value)
		}
	}
	return storedByTenant
}
