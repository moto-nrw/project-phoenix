package config

import (
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
