package config

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type slotListTestRuntime struct {
	testSettingsRuntime
	active bool
	lock   func(context.Context, string, bool) error
}

func (r slotListTestRuntime) HasTransaction(context.Context) bool { return r.active }
func (r slotListTestRuntime) AcquireLock(ctx context.Context, key string, shared bool) error {
	if r.lock != nil {
		return r.lock(ctx, key, shared)
	}
	return nil
}

func newSlotListTestService(t *testing.T, runtime config.Runtime) (*settingsService, *mockValueRepo, *config.Registry, context.Context, int64) {
	t.Helper()
	tenantID := testpkg.UniqueTestTenantID(t)
	registry := config.NewRegistry()
	registerTestSetting(registry, config.KeySlotListShortDayCutoff, config.FieldTime, "14:30")
	registerTestSetting(registry, config.KeySlotListLongDayCutoff, config.FieldTime, "16:00")
	registerTestSetting(registry, config.KeyTimetableEnabled, config.FieldBoolean, true)
	registerTestSetting(registry, "test.other", config.FieldText, "other")
	repo := newMockValueRepo()
	for key, value := range map[string]string{config.KeySlotListShortDayCutoff: `"15:00"`, config.KeySlotListLongDayCutoff: `"17:30"`} {
		repo.values[repo.key(tenantID, key)] = slotListStoredValue(tenantID, key, value)
	}
	service := newSettingsService(repo, &mockAuditRepo{}, nil, runtime, slog.Default(), nil, config.ServiceOptions{}, registry)
	return service, repo, registry, withTestTenantID(context.Background(), tenantID), tenantID
}

func slotListStoredValue(tenantID int64, key, value string) *config.SettingValue {
	stored := &config.SettingValue{SettingKey: key, Value: json.RawMessage(value)}
	stored.SetTenantID(tenantID)
	return stored
}

func TestSlotListSettingsPreservesExplicitSnapshotContexts(t *testing.T) {
	t.Parallel()
	short, long := config.KeySlotListShortDayCutoff, config.KeySlotListLongDayCutoff
	for _, tc := range []struct {
		name                string
		keys                []string
		foreign             bool
		wantShort, wantLong string
		queries             int
	}{
		{"complete", []string{short, long}, false, "14:30", "16:00", 0},
		{"short-only", []string{short}, false, "14:30", "17:30", 1},
		{"long-only", []string{long}, false, "15:00", "16:00", 1},
		{"empty", []string{}, false, "15:00", "17:30", 2},
		{"other-key", []string{"test.other"}, false, "15:00", "17:30", 2},
		{"other-tenant", []string{short, long}, true, "15:00", "17:30", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, repo, registry, ctx, tenantID := newSlotListTestService(t, slotListTestRuntime{active: true})
			if tc.foreign {
				tenantID = testpkg.UniqueTestTenantID(t)
			}
			snapshot, err := config.NewSettingsSnapshot(registry, tenantID, tc.keys, nil)
			require.NoError(t, err)
			ctx = config.WithSettingsRequestCache(config.WithSettingsSnapshot(ctx, snapshot))
			gotShort, gotLong, err := service.PickupCutoffs(ctx)
			require.NoError(t, err)
			require.Equal(t, tc.wantShort, gotShort)
			require.Equal(t, tc.wantLong, gotLong)
			require.Equal(t, tc.queries, repo.findManyCalls)
			for _, keys := range repo.findManyKeys {
				require.Len(t, keys, 1, "explicit snapshot keeps scalar lookup boundaries")
			}
		})
	}
}

func TestSlotListSettingsRefreshesRequestSnapshotAfterLock(t *testing.T) {
	t.Parallel()
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-transaction", true: "locked-transaction"}[active], func(t *testing.T) {
			locks := 0
			runtime := slotListTestRuntime{active: active, lock: func(_ context.Context, _ string, shared bool) error {
				locks++
				require.True(t, shared)
				return nil
			}}
			service, repo, registry, ctx, tenantID := newSlotListTestService(t, runtime)
			ctx = config.WithSettingsRequestCache(ctx)
			warm, err := config.NewSettingsSnapshot(registry, tenantID, []string{config.KeySlotListShortDayCutoff, config.KeySlotListLongDayCutoff}, nil)
			require.NoError(t, err)
			config.RequestCacheFromContext(ctx).StoreSnapshot(tenantID, warm)
			gotShort, gotLong, err := service.PickupCutoffs(ctx)
			require.NoError(t, err)
			require.Equal(t, "15:00", gotShort, "the existing lock helper flushes old memo entries")
			require.Equal(t, "17:30", gotLong)
			if active {
				require.Equal(t, 1, locks)
			} else {
				require.Zero(t, locks)
			}
			require.Equal(t, [][]string{{"test.other", config.KeyTimetableEnabled, config.KeySlotListLongDayCutoff, config.KeySlotListShortDayCutoff}}, repo.findManyKeys)
		})
	}
}

func TestSlotListSettingsPreservesLockAndLookupFailures(t *testing.T) {
	t.Parallel()
	failed := errors.New("lookup unavailable")
	service, repo, _, ctx, _ := newSlotListTestService(t, slotListTestRuntime{active: true, lock: func(context.Context, string, bool) error { return failed }})
	_, _, err := service.PickupCutoffs(ctx)
	require.ErrorIs(t, err, failed)
	require.Zero(t, repo.findManyCalls)
	service, repo, _, ctx, _ = newSlotListTestService(t, slotListTestRuntime{active: true})
	repo.err = failed
	_, _, err = service.PickupCutoffs(ctx)
	require.ErrorIs(t, err, failed)
	require.ErrorContains(t, err, "resolve short-day pickup cutoff:")
	require.Equal(t, 1, repo.findManyCalls, "do not retry a failed batch")
}

func TestSlotListSettingsPreservesEntryFailureOrder(t *testing.T) {
	t.Parallel()
	for _, badKey := range []string{config.KeySlotListShortDayCutoff, config.KeySlotListLongDayCutoff} {
		t.Run(badKey, func(t *testing.T) {
			service, repo, _, ctx, tenantID := newSlotListTestService(t, slotListTestRuntime{active: true})
			repo.values[repo.key(tenantID, badKey)] = slotListStoredValue(tenantID, badKey, `{broken`)
			_, _, err := service.PickupCutoffs(ctx)
			require.Error(t, err)
			if badKey == config.KeySlotListShortDayCutoff {
				require.ErrorContains(t, err, "resolve short-day pickup cutoff:")
			} else {
				require.ErrorContains(t, err, "resolve long-day pickup cutoff:")
			}
			require.Equal(t, 1, repo.findManyCalls)
		})
	}
}

func TestSlotListSettingsRefilledMemoRetainsEarlyError(t *testing.T) {
	t.Parallel()
	service, repo, registry, ctx, tenantID := newSlotListTestService(t, nil)
	ctx = config.WithSettingsRequestCache(ctx)
	key := config.KeySlotListShortDayCutoff
	broken, err := config.NewSettingsSnapshot(registry, tenantID, []string{key}, []*config.SettingValue{slotListStoredValue(tenantID, key, `{broken`)})
	require.NoError(t, err)
	service.runtime = slotListTestRuntime{active: true, lock: func(ctx context.Context, _ string, _ bool) error {
		config.RequestCacheFromContext(ctx).StoreSnapshot(tenantID, broken)
		return nil
	}}
	repo.err = errors.New("later long read unavailable")
	_, _, err = service.PickupCutoffs(ctx)
	require.ErrorContains(t, err, "resolve short-day pickup cutoff:")
	require.ErrorContains(t, err, "unmarshal value")
	require.Zero(t, repo.findManyCalls, "the early cached entry error wins over a later database failure")
}

func TestSlotListSettingsDoesNotParseClockValuesEarly(t *testing.T) {
	t.Parallel()
	service, repo, _, ctx, tenantID := newSlotListTestService(t, slotListTestRuntime{active: true})
	repo.values[repo.key(tenantID, config.KeySlotListShortDayCutoff)] = slotListStoredValue(tenantID, config.KeySlotListShortDayCutoff, `"not-a-time"`)
	repo.values[repo.key(tenantID, config.KeySlotListLongDayCutoff)] = slotListStoredValue(tenantID, config.KeySlotListLongDayCutoff, `true`)
	short, long, err := service.PickupCutoffs(ctx)
	require.NoError(t, err)
	require.Equal(t, "not-a-time", short)
	require.Equal(t, "true", long)
	enabled, err := service.TimetableEnabled(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestSlotListSettingsMissingDefinitionKeepsScalarErrorOrder(t *testing.T) {
	t.Parallel()
	service, repo, _, ctx, tenantID := newSlotListTestService(t, slotListTestRuntime{active: true})
	registry := config.NewRegistry()
	key := config.KeySlotListShortDayCutoff
	registerTestSetting(registry, key, config.FieldTime, "14:30")
	service.registry = registry // no long-key definition in this custom registry
	repo.values[repo.key(tenantID, key)] = slotListStoredValue(tenantID, key, `{broken`)
	_, _, err := service.PickupCutoffs(ctx)
	require.ErrorContains(t, err, "resolve short-day pickup cutoff:")
	require.ErrorContains(t, err, "unmarshal value")
	require.Equal(t, [][]string{{key}}, repo.findManyKeys, "do not report the missing long definition ahead of the first entry error")
}
