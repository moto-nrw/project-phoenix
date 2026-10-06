package config

import (
	"context"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestParentAbsenceReviewScopeProjectsAndResetsWithoutChangingLegacyRights(t *testing.T) {
	t.Parallel()
	for _, profile := range []string{"missing", "explicit false", "explicit true"} {
		t.Run(profile, func(t *testing.T) {
			t.Parallel()
			ctx := testpkg.OwnCtx(t)
			settings := attendanceScopeSettings(t)
			const oldKey = configModel.KeyParentRequestGroupLeaderReviewEnabled
			const newKey = configModel.KeyParentAbsenceReviewScope
			if profile != "missing" {
				require.NoError(t, settings.SetValue(ctx, oldKey, profile == "explicit true", nil, nil))
			}
			readField := func() *ResolvedSetting {
				t.Helper()
				schema, err := settings.GetSchema(ctx, []string{"admin:*"})
				require.NoError(t, err)
				for _, tab := range schema.Tabs {
					for _, category := range tab.Categories {
						for _, setting := range category.Items {
							if setting.Key == newKey {
								return setting
							}
						}
					}
				}
				t.Fatal("absence review setting missing")
				return nil
			}
			want := configModel.ParentAbsenceReviewScopeAdmins
			if profile == "explicit true" {
				want = configModel.ParentAbsenceReviewScopeGroupLeaders
			}
			field := readField()
			require.Equal(t, want, field.Value)
			require.Equal(t, want, field.Default)
			require.True(t, field.IsDefault)
			require.Len(t, field.Options.Static, 3)
			for _, option := range field.Options.Static {
				require.NotEqual(t, configModel.ParentAbsenceReviewScopeInherit, option.Value)
			}
			hasOverride, err := settings.HasTenantOverride(ctx, newKey)
			require.NoError(t, err)
			require.False(t, hasOverride, "schema reads must not materialize inherited rights")
			for _, value := range []string{configModel.ParentAbsenceReviewScopeAllStaff, want, configModel.ParentAbsenceReviewScopeInherit} {
				require.NoError(t, settings.SetValue(ctx, newKey, value, nil, nil))
				require.False(t, readField().IsDefault, "explicit defaults retain their reset affordance")
			}
			require.NoError(t, settings.ResetValue(ctx, newKey, nil, nil))
			field = readField()
			require.True(t, field.IsDefault)
			require.Equal(t, want, field.Value)
			oldValue, err := settings.ResolveBool(ctx, oldKey)
			require.NoError(t, err)
			require.Equal(t, profile == "explicit true", oldValue)
			hasOverride, err = settings.HasTenantOverride(ctx, oldKey)
			require.NoError(t, err)
			require.Equal(t, profile != "missing", hasOverride)
		})
	}
}

// schemaSetting returns the tenant schema entry of key, or nil when the
// schema leaves it out.
func schemaSetting(t *testing.T, ctx context.Context, settings SettingsService, key string) *ResolvedSetting {
	t.Helper()
	schema, err := settings.GetSchema(ctx, []string{"admin:*"})
	require.NoError(t, err)
	for _, tab := range schema.Tabs {
		for _, category := range tab.Categories {
			for _, setting := range category.Items {
				if setting.Key == key {
					return setting
				}
			}
		}
	}
	return nil
}

// #3804: one request scope replaces the group-leader switch on the settings
// page. Without an override it shows the switch's effective choice, and the
// absence scope's inherit value follows the request scope.
func TestParentRequestReviewScopeProjectsSwitchAndDrivesAbsenceInheritance(t *testing.T) {
	t.Parallel()
	for _, profile := range []string{"missing", "explicit false", "explicit true"} {
		t.Run(profile, func(t *testing.T) {
			t.Parallel()
			ctx := testpkg.OwnCtx(t)
			settings := attendanceScopeSettings(t)
			const switchKey = configModel.KeyParentRequestGroupLeaderReviewEnabled
			const requestKey = configModel.KeyParentRequestReviewScope
			const absenceKey = configModel.KeyParentAbsenceReviewScope
			if profile != "missing" {
				require.NoError(t, settings.SetValue(ctx, switchKey, profile == "explicit true", nil, nil))
			}
			inherited := configModel.ParentRequestReviewScopeAdmins
			if profile == "explicit true" {
				inherited = configModel.ParentRequestReviewScopeGroupLeaders
			}

			require.Nil(t, schemaSetting(t, ctx, settings, switchKey), "the switch has one visible successor")
			field := schemaSetting(t, ctx, settings, requestKey)
			require.NotNil(t, field)
			require.Equal(t, inherited, field.Value)
			require.Equal(t, inherited, field.Default)
			require.True(t, field.IsDefault)
			require.Len(t, field.Options.Static, 3)
			for _, option := range field.Options.Static {
				require.NotEqual(t, configModel.ParentRequestReviewScopeInherit, option.Value)
			}
			require.Equal(t, inherited, schemaSetting(t, ctx, settings, absenceKey).Value)
			hasOverride, err := settings.HasTenantOverride(ctx, requestKey)
			require.NoError(t, err)
			require.False(t, hasOverride, "schema reads must not materialize inherited rights")

			for _, value := range []string{
				configModel.ParentRequestReviewScopeAllStaff,
				configModel.ParentRequestReviewScopeAdmins,
				configModel.ParentRequestReviewScopeGroupLeaders,
			} {
				require.NoError(t, settings.SetValue(ctx, requestKey, value, nil, nil))
				field = schemaSetting(t, ctx, settings, requestKey)
				require.Equal(t, value, field.Value)
				require.False(t, field.IsDefault, "explicit choices retain their reset affordance")
				absence := schemaSetting(t, ctx, settings, absenceKey)
				require.Equal(t, value, absence.Value, "absence inherit follows the request scope")
				require.Equal(t, value, absence.Default)
			}

			require.NoError(t, settings.ResetValue(ctx, requestKey, nil, nil))
			field = schemaSetting(t, ctx, settings, requestKey)
			require.True(t, field.IsDefault)
			require.Equal(t, inherited, field.Value)
			switchValue, err := settings.ResolveBool(ctx, switchKey)
			require.NoError(t, err)
			require.Equal(t, profile == "explicit true", switchValue, "the stored switch stays intact")
		})
	}
}
