package iot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// deviceConfigCheckout holds checkout button visibility settings.
type deviceConfigCheckout struct {
	RaumwechselEnabled bool    `json:"raumwechsel_enabled"`
	SchulhofEnabled    bool    `json:"schulhof_enabled"`
	WCEnabled          bool    `json:"wc_enabled"`
	DailyCheckoutTime  *string `json:"daily_checkout_time"` // "HH:MM" or null (always available)
}

// deviceConfigFeedback holds feedback settings.
type deviceConfigFeedback struct {
	Enabled bool `json:"enabled"`
}

// deviceConfigResponse is the payload for GET /api/iot/config.
// PresenceMode tells the kiosk whether the tenant runs the detailed flow
// (room selection, visit tracking) or the binary flow (attendance only —
// simpler single-tap door kiosk). Old kiosk builds that don't read the field
// default to detailed behavior, so the contract is backwards-compatible.
type deviceConfigResponse struct {
	Checkout     deviceConfigCheckout `json:"checkout"`
	Feedback     deviceConfigFeedback `json:"feedback"`
	PresenceMode string               `json:"presence_mode"`
}

// getDeviceConfig returns device-relevant settings for the authenticated device's school.
// Auth: device API key only (no PIN required).
func (rs *Resource) getDeviceConfig(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		rs.getLogger().WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			rs.getLogger().ErrorContext(r.Context(), "failed to render device auth error", slog.String("error", err.Error()))
		}
		return
	}

	response, err := rs.resolveDeviceConfig(r.Context(), deviceCtx.TenantID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to resolve device configuration", err))
		return
	}
	common.Respond(w, r, http.StatusOK, response, "Device configuration retrieved")
}

func (rs *Resource) resolveDeviceConfig(ctx context.Context, tenantID int64) (deviceConfigResponse, error) {
	var response deviceConfigResponse
	settingsCtx, err := rs.deviceConfigSettingsContext(ctx, tenantID)
	if err != nil {
		return response, err
	}
	rawTime, err := rs.SettingsService.ResolveStringForTenant(settingsCtx, tenantID, configModel.KeyStudentDailyCheckoutTime)
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
		value, resolveErr := rs.SettingsService.ResolveBoolForTenant(settingsCtx, tenantID, setting.key)
		if resolveErr != nil {
			return response, resolveErr
		}
		*setting.target = value
	}
	response.PresenceMode, err = rs.SettingsService.ResolveStringForTenant(settingsCtx, tenantID, configModel.KeyPresenceMode)
	return response, err
}

func (rs *Resource) deviceConfigSettingsContext(ctx context.Context, tenantID int64) (context.Context, error) {
	if rs.SettingsService == nil {
		return ctx, fmt.Errorf("device configuration requires settings service")
	}
	batch, ok := rs.SettingsService.(interface {
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
