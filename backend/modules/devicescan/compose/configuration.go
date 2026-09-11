package compose

import (
	"context"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type ConfigurationQuery = devicescan.ConfigurationQuery
type configurationResolver interface {
	ResolveStringForTenant(context.Context, int64, string) (string, error)
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}

type configurationSnapshot = configSvc.SettingsSnapshot
type configurationSettings struct{ settings configurationResolver }

func NewConfiguration(settings configurationResolver) ConfigurationQuery {
	return application.NewConfiguration(configurationSettings{settings}, principals{})
}

func (rs configurationSettings) LoadConfiguration(ctx context.Context, tenantID int64) (devicescan.Configuration, error) {
	return rs.resolveDeviceConfig(ctx, tenantID)
}

func (rs configurationSettings) resolveDeviceConfig(ctx context.Context, tenantID int64) (devicescan.Configuration, error) {
	var response devicescan.Configuration
	settingsCtx, err := rs.deviceConfigSettingsContext(ctx, tenantID)
	if err != nil {
		return response, err
	}
	rawTime, err := rs.settings.ResolveStringForTenant(settingsCtx, tenantID, configModel.KeyStudentDailyCheckoutTime)
	if err != nil {
		return response, err
	}
	if rawTime != "" {
		response.Checkout.DailyCheckoutTime = &rawTime
	}
	for _, setting := range []struct {
		key    string
		target *bool
	}{
		{configModel.KeyCheckoutRaumwechselEnabled, &response.Checkout.RaumwechselEnabled},
		{configModel.KeyCheckoutSchulhofEnabled, &response.Checkout.SchulhofEnabled},
		{configModel.KeyCheckoutWCEnabled, &response.Checkout.WCEnabled},
		{configModel.KeyFeedbackEnabled, &response.Feedback.Enabled},
	} {
		value, resolveErr := rs.settings.ResolveBoolForTenant(settingsCtx, tenantID, setting.key)
		if resolveErr != nil {
			return response, resolveErr
		}
		*setting.target = value
	}
	response.PresenceMode, err = rs.settings.ResolveStringForTenant(settingsCtx, tenantID, configModel.KeyPresenceMode)
	return response, err
}

func (rs configurationSettings) deviceConfigSettingsContext(ctx context.Context, tenantID int64) (context.Context, error) {
	if rs.settings == nil {
		return ctx, fmt.Errorf("device configuration requires settings service")
	}
	batch, ok := rs.settings.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*configSvc.SettingsSnapshot, error)
	})
	if !ok {
		return ctx, nil
	}

	snapshot, err := batch.ResolveManyForTenant(ctx, tenantID, []string{
		configModel.KeyCheckoutRaumwechselEnabled,
		configModel.KeyCheckoutSchulhofEnabled,
		configModel.KeyCheckoutWCEnabled,
		configModel.KeyFeedbackEnabled,
		configModel.KeyStudentDailyCheckoutTime,
		configModel.KeyPresenceMode,
	})
	if err != nil {
		return ctx, err
	}
	if snapshot == nil {
		return ctx, nil
	}
	settingsCtx := tenant.WithTenantID(ctx, tenantID)
	return configSvc.WithSettingsSnapshot(settingsCtx, snapshot), nil
}
