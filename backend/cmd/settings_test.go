package cmd

import (
	"context"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

type settingsOverrideSchoolQuery struct {
	services.SchoolQuery
	list func(context.Context) ([]services.School, error)
}

func (q settingsOverrideSchoolQuery) ListSchools(ctx context.Context) ([]services.School, error) {
	return q.list(ctx)
}

type settingsOverrideValueRepository struct {
	configModel.SettingValueRepository
	find func(context.Context, int64, string) (*configModel.SettingValue, error)
}

func (r settingsOverrideValueRepository) FindByTenantAndKey(ctx context.Context, tenantID int64, key string) (*configModel.SettingValue, error) {
	return r.find(ctx, tenantID, key)
}

func TestRunSettingsOverridesWithContextAttachesTenantRuntime(t *testing.T) {
	var adminCalls int
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, callback func(context.Context, any) error) error {
			return callback(ctx, nil)
		},
		func(ctx context.Context, callback func(context.Context, any) error) error {
			adminCalls++
			return callback(ctx, nil)
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)

	assertRuntime := func(ctx context.Context) error {
		return tenant.WithinAdmin(ctx, func(context.Context) error { return nil })
	}
	commandContext := &settingsCommandContext{
		cleanupContext: &cleanupContext{TenantRuntime: runtime},
		schools: settingsOverrideSchoolQuery{list: func(ctx context.Context) ([]services.School, error) {
			return []services.School{{ID: 1}}, assertRuntime(ctx)
		}},
		values: settingsOverrideValueRepository{find: func(ctx context.Context, _ int64, _ string) (*configModel.SettingValue, error) {
			return nil, assertRuntime(ctx)
		}},
	}

	require.NoError(t, runSettingsOverridesWithContext(commandContext))
	require.Greater(t, adminCalls, 1)
}
