package settings_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolSettingsRejectTraegerTransactionBeforeDefaultsOrCaches(t *testing.T) {
	t.Parallel()
	f := newTraegerLevelFixture(t)
	key := records.KeyGDPRAttendanceRetentionDays
	for school, days := range map[int64]int{f.first: 40, f.second: 90} {
		require.NoError(t, testpkg.WithinTenantContext(t, testpkg.ContextForTenant(f.ctx, school), f.db, school, func(ctx context.Context) error {
			return f.module.Settings.SetValue(ctx, key, days, &f.account, nil)
		}))
	}
	svc := f.module.Settings
	ctx := records.WithSettingsRequestCache(testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, f.db)))
	// Warm the explicit school's request cache outside the Träger transaction.
	snapshot, err := svc.ResolveManyForTenant(ctx, f.first, []string{key})
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTraeger(ctx, f.traeger, func(traegerCtx context.Context) error {
		readers := map[string]func(context.Context) error{
			"Resolve":    func(ctx context.Context) error { _, err := svc.Resolve(ctx, key); return err },
			"ResolveInt": func(ctx context.Context) error { _, err := svc.ResolveInt(ctx, key); return err },
			"ResolveString": func(ctx context.Context) error {
				_, err := svc.ResolveString(ctx, records.KeySessionEndTime)
				return err
			},
			"ResolveBool": func(ctx context.Context) error {
				_, err := svc.ResolveBool(ctx, records.KeyParentNewsEnabled)
				return err
			},
			"ResolveBools": func(ctx context.Context) error {
				_, err := svc.ResolveBools(ctx, []string{records.KeyParentNewsEnabled})
				return err
			},
			"ResolveMany":       func(ctx context.Context) error { _, err := svc.ResolveMany(ctx, []string{key}); return err },
			"HasTenantOverride": func(ctx context.Context) error { _, err := svc.HasTenantOverride(ctx, key); return err },
			"ResolveManyForTenant": func(ctx context.Context) error {
				_, err := svc.ResolveManyForTenant(ctx, f.first, []string{key})
				return err
			},
			"ResolveManyForTenants": func(ctx context.Context) error {
				_, err := svc.ResolveManyForTenants(ctx, []int64{f.first}, []string{key})
				return err
			},
			"ResolveStringForTenantInTx": func(ctx context.Context) error {
				_, err := svc.ResolveStringForTenantInTx(ctx, f.first, records.KeySessionEndTime)
				return err
			},
			"ResolveBoolForTenantInTx": func(ctx context.Context) error {
				_, err := svc.ResolveBoolForTenantInTx(ctx, f.first, records.KeyParentNewsEnabled)
				return err
			},
		}
		zeroSnapshot, snapshotErr := records.NewSettingsSnapshot(records.DefaultRegistry(), 0, []string{key}, nil)
		require.NoError(t, snapshotErr)
		for name, read := range readers {
			for scope, readCtx := range map[string]context.Context{
				"unbound":                        traegerCtx,
				"warm cache":                     testpkg.ContextForTenant(traegerCtx, f.first),
				"default snapshot":               records.WithSettingsSnapshot(traegerCtx, zeroSnapshot),
				"school snapshot and warm cache": records.WithSettingsSnapshot(testpkg.ContextForTenant(traegerCtx, f.first), snapshot),
			} {
				assert.ErrorIs(t, read(readCtx), records.ErrSchoolReadInTraegerTransaction, "%s: %s", name, scope)
			}
		}
		// Use the original caller context for a separate admin/school read.
		// The existing #6169 Mensa test covers detaching the Träger context.
		isolated := ctx
		batch, readErr := svc.ResolveManyForTenants(isolated, []int64{f.first, f.second}, []string{key})
		require.NoError(t, readErr)
		assert.Len(t, batch, 2)
		assert.NotContains(t, batch, f.otherSchool)
		for school, want := range map[int64]int{f.first: 40, f.second: 90} {
			got, valueErr := batch[school].Int(key)
			require.NoError(t, valueErr)
			assert.Equal(t, want, got)
		}
		// A separate school unit can still read its override and a genuinely unset key.
		own, readErr := svc.ResolveIntForTenant(isolated, f.first, key)
		require.NoError(t, readErr)
		assert.Equal(t, 40, own)
		unset, readErr := svc.ResolveIntForTenant(isolated, f.otherSchool, key)
		require.NoError(t, readErr)
		assert.EqualValues(t, records.GetDefinition(key).Default, unset)
		require.NoError(t, testpkg.WithinTenantContext(t, testpkg.ContextForTenant(isolated, f.first), f.db, f.first, func(schoolCtx context.Context) error {
			_, foreignErr := svc.ResolveIntForTenant(schoolCtx, f.otherSchool, key)
			assert.Error(t, foreignErr, "a school transaction cannot read another school")
			return nil
		}))
		return nil
	}))
}
