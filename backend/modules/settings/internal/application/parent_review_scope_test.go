package config

import (
	"context"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
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
			readField := func() *configModel.ResolvedSetting {
				t.Helper()
				schema, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.GetSchema, []string{"admin:*"})
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
			hasOverride, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.HasTenantOverride, newKey)
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
			oldValue, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveBool, oldKey)
			require.NoError(t, err)
			require.Equal(t, profile == "explicit true", oldValue)
			hasOverride, err = testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.HasTenantOverride, oldKey)
			require.NoError(t, err)
			require.Equal(t, profile != "missing", hasOverride)
		})
	}
}

// schemaSetting returns the tenant schema entry of key, or nil when the
// schema leaves it out.
func schemaSetting(t *testing.T, ctx context.Context, settings configModel.SettingsService, key string) *configModel.ResolvedSetting {
	t.Helper()
	schema, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.GetSchema, []string{"admin:*"})
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
			hasOverride, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.HasTenantOverride, requestKey)
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
			switchValue, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), settings.ResolveBool, switchKey)
			require.NoError(t, err)
			require.Equal(t, profile == "explicit true", switchValue, "the stored switch stays intact")
		})
	}
}

func reviewScopeOptionValues(t *testing.T, field *configModel.ResolvedSetting) []any {
	t.Helper()
	require.NotNil(t, field)
	values := make([]any, 0, len(field.Options.Static))
	for _, option := range field.Options.Static {
		values = append(values, option.Value)
	}
	return values
}

// #1480: a school without fixed groups has no group leaders, so neither
// review scope offers them. An effective group-leader choice stays listed so
// the page still shows what applies until an admin changes it.
func TestParentReviewScopesHideGroupLeadersInOpenCare(t *testing.T) {
	t.Parallel()
	const requestKey = configModel.KeyParentRequestReviewScope
	const absenceKey = configModel.KeyParentAbsenceReviewScope
	groupLeaders := configModel.ParentRequestReviewScopeGroupLeaders

	t.Run("fixed groups keep the option", func(t *testing.T) {
		t.Parallel()
		ctx := testpkg.OwnCtx(t)
		settings := attendanceScopeSettings(t)
		require.NoError(t, settings.SetValue(ctx, configModel.KeyGroupMode, configModel.GroupModeFixedGroups, nil, nil))
		for _, key := range []string{requestKey, absenceKey} {
			require.Contains(t, reviewScopeOptionValues(t, schemaSetting(t, ctx, settings, key)), groupLeaders)
		}
	})

	t.Run("open care drops the option", func(t *testing.T) {
		t.Parallel()
		ctx := testpkg.OwnCtx(t)
		settings := attendanceScopeSettings(t)
		require.NoError(t, settings.SetValue(ctx, configModel.KeyGroupMode, configModel.GroupModeOpenCare, nil, nil))
		for _, key := range []string{requestKey, absenceKey} {
			field := schemaSetting(t, ctx, settings, key)
			require.Equal(t, []any{
				configModel.ParentRequestReviewScopeAdmins,
				configModel.ParentRequestReviewScopeAllStaff,
			}, reviewScopeOptionValues(t, field), key)
		}
	})

	t.Run("open care keeps an effective group-leader choice", func(t *testing.T) {
		t.Parallel()
		ctx := testpkg.OwnCtx(t)
		settings := attendanceScopeSettings(t)
		require.NoError(t, settings.SetValue(ctx, configModel.KeyGroupMode, configModel.GroupModeOpenCare, nil, nil))
		require.NoError(t, settings.SetValue(ctx, requestKey, groupLeaders, nil, nil))
		require.NoError(t, settings.SetValue(ctx, absenceKey, configModel.ParentAbsenceReviewScopeAllStaff, nil, nil))

		request := schemaSetting(t, ctx, settings, requestKey)
		require.Equal(t, groupLeaders, request.Value)
		require.Contains(t, reviewScopeOptionValues(t, request), groupLeaders)
		absence := schemaSetting(t, ctx, settings, absenceKey)
		require.NotContains(t, reviewScopeOptionValues(t, absence), groupLeaders)
	})
	// The old switch names group leaders a school without groups has none of;
	// the inherited scope falls to admins, as the review policy enforces.
	t.Run("open care ignores the switch on the inherited path", func(t *testing.T) {
		t.Parallel()
		ctx := testpkg.OwnCtx(t)
		settings := attendanceScopeSettings(t)
		require.NoError(t, settings.SetValue(ctx, configModel.KeyGroupMode, configModel.GroupModeOpenCare, nil, nil))
		require.NoError(t, settings.SetValue(ctx, configModel.KeyParentRequestGroupLeaderReviewEnabled, true, nil, nil))

		for _, key := range []string{requestKey, absenceKey} {
			field := schemaSetting(t, ctx, settings, key)
			require.Equal(t, configModel.ParentRequestReviewScopeAdmins, field.Value, key)
			require.Equal(t, configModel.ParentRequestReviewScopeAdmins, field.Default, key)
			require.NotContains(t, reviewScopeOptionValues(t, field), groupLeaders, key)
		}
	})
}
