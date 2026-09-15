package config

import (
	"context"
	"errors"
	"fmt"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func parentReportFields(t *testing.T, ctx context.Context, settings SettingsService) map[string]*ResolvedSetting {
	t.Helper()
	schema, err := settings.GetSchema(ctx, []string{"admin:*"})
	require.NoError(t, err)
	fields := map[string]*ResolvedSetting{}
	for _, tab := range schema.Tabs {
		for _, category := range tab.Categories {
			for _, field := range category.Items {
				fields[field.Key] = field
			}
		}
	}
	return fields
}

func TestParentReportModeLegacyProfilesAndIndependentWrites(t *testing.T) {
	t.Parallel()
	for _, main := range []bool{false, true} {
		for _, sickApproval := range []bool{false, true} {
			for _, excusedApproval := range []bool{false, true} {
				t.Run(fmt.Sprintf("main=%t/sick=%t/excused=%t", main, sickApproval, excusedApproval), func(t *testing.T) {
					t.Parallel()
					ctx := testpkg.OwnCtx(t)
					settings := attendanceScopeSettings(t)
					for key, value := range map[string]bool{
						configModel.KeyParentSickNoteEnabled:         main,
						configModel.KeyParentSickRequiresApproval:    sickApproval,
						configModel.KeyParentExcusedRequiresApproval: excusedApproval,
					} {
						require.NoError(t, settings.SetValue(ctx, key, value, nil, nil))
					}
					fields := parentReportFields(t, ctx, settings)
					require.NotContains(t, fields, configModel.KeyParentSickNoteEnabled)
					require.NotContains(t, fields, configModel.KeyParentSickRequiresApproval)
					require.NotContains(t, fields, configModel.KeyParentExcusedRequiresApproval)
					for _, report := range []struct {
						key      string
						approval bool
					}{
						{configModel.KeyParentSickReportsEnabled, sickApproval},
						{configModel.KeyParentExcusedReportsEnabled, excusedApproval},
					} {
						want := "off"
						if main {
							want = "immediate"
							if report.approval {
								want = "approval"
							}
						}
						require.Equal(t, want, fields[report.key].Value)
						require.Equal(t, configModel.FieldSelect, fields[report.key].Type)
						require.Len(t, fields[report.key].Options.Static, 3)
					}
					const key = configModel.KeyParentSickReportsEnabled
					otherMode := fields[configModel.KeyParentExcusedReportsEnabled].Value
					for _, mode := range []string{"off", "immediate", "approval", "off"} {
						require.NoError(t, settings.SetValue(ctx, key, mode, nil, []string{"config:manage"}))
						fields = parentReportFields(t, ctx, settings)
						require.Equal(t, mode, fields[key].Value)
						require.Equal(t, otherMode, fields[configModel.KeyParentExcusedReportsEnabled].Value)
						approval, err := settings.ResolveBool(ctx, configModel.KeyParentExcusedRequiresApproval)
						require.NoError(t, err)
						require.Equal(t, excusedApproval, approval)
					}
					require.NoError(t, settings.ResetValue(ctx, key, nil, []string{"config:manage"}))
					fields = parentReportFields(t, ctx, settings)
					require.True(t, fields[key].IsDefault)
					require.Equal(t, "approval", fields[key].Value)
					require.Equal(t, otherMode, fields[configModel.KeyParentExcusedReportsEnabled].Value)
				})
			}
		}
	}
}

func TestParentReportModesConcurrentEnablesAndRollback(t *testing.T) {
	t.Parallel()
	ctx := testpkg.Ctx(t)
	settings := attendanceScopeSettings(t)
	require.NoError(t, settings.SetValue(ctx, configModel.KeyParentSickNoteEnabled, false, nil, nil))
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, key := range []string{configModel.KeyParentSickReportsEnabled, configModel.KeyParentExcusedReportsEnabled} {
		go func() { <-start; results <- settings.SetValue(ctx, key, "immediate", nil, nil) }()
	}
	close(start)
	for range 2 {
		require.NoError(t, <-results)
	}
	fields := parentReportFields(t, ctx, settings)
	require.Equal(t, "immediate", fields[configModel.KeyParentSickReportsEnabled].Value)
	require.Equal(t, "immediate", fields[configModel.KeyParentExcusedReportsEnabled].Value)
	runtime := testpkg.SettingsRuntime(t, testpkg.SetupTestDB(t))
	abort := errors.New("rollback")
	err := runtime.WithinTenant(ctx, testpkg.Tenant(t), func(txCtx context.Context) error {
		if err := settings.SetValue(txCtx, configModel.KeyParentSickReportsEnabled, "approval", nil, nil); err != nil {
			return err
		}
		return abort
	})
	require.ErrorIs(t, err, abort)
	fields = parentReportFields(t, ctx, settings)
	require.Equal(t, "immediate", fields[configModel.KeyParentSickReportsEnabled].Value)
	require.Error(t, settings.SetValue(ctx, configModel.KeyParentSickReportsEnabled, "unknown", nil, nil))
	require.Error(t, settings.SetValue(ctx, configModel.KeyParentSickReportsEnabled, "approval", nil, []string{"config:read"}))
}
