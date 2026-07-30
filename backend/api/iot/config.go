package iot

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	checkinSvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
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

	response := rs.resolveDeviceConfig(r.Context(), deviceCtx.TenantID)
	common.Respond(w, r, http.StatusOK, response, "Device configuration retrieved")
}

func (rs *Resource) resolveDeviceConfig(ctx context.Context, tenantID int64) deviceConfigResponse {
	response := deviceConfigResponse{
		Checkout: deviceConfigCheckout{
			RaumwechselEnabled: true,
			SchulhofEnabled:    true,
			WCEnabled:          true,
		},
		PresenceMode: configModel.PresenceModeDetailed,
	}

	settingsCtx, available := rs.deviceConfigSettingsContext(ctx, tenantID)
	if !available {
		return response
	}

	response.Checkout.RaumwechselEnabled = resolveDeviceConfigBool(
		settingsCtx, rs.SettingsService, configModel.KeyCheckoutRaumwechselEnabled, true)
	response.Checkout.SchulhofEnabled = resolveDeviceConfigBool(
		settingsCtx, rs.SettingsService, configModel.KeyCheckoutSchulhofEnabled, true)
	response.Checkout.WCEnabled = resolveDeviceConfigBool(
		settingsCtx, rs.SettingsService, configModel.KeyCheckoutWCEnabled, true)
	response.Feedback.Enabled = resolveDeviceConfigBool(
		settingsCtx, rs.SettingsService, configModel.KeyFeedbackEnabled, false)

	if rawTime := checkinSvc.ResolveRawDailyCheckoutTime(settingsCtx, rs.SettingsService); rawTime != "" {
		response.Checkout.DailyCheckoutTime = &rawTime
	}
	response.PresenceMode = configSvc.ResolvePresenceModeForTenant(
		settingsCtx, rs.SettingsService, tenantID, rs.getLogger())
	return response
}

func (rs *Resource) deviceConfigSettingsContext(ctx context.Context, tenantID int64) (context.Context, bool) {
	if rs.SettingsService == nil {
		return ctx, true
	}
	batch, ok := rs.SettingsService.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*configSvc.SettingsSnapshot, error)
	})
	if !ok {
		return ctx, true
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
		rs.getLogger().WarnContext(ctx, "device config settings batch failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return ctx, false
	}
	if snapshot == nil {
		return ctx, true
	}
	settingsCtx := tenant.WithTenantID(ctx, tenantID)
	return configSvc.WithSettingsSnapshot(settingsCtx, snapshot), true
}

func resolveDeviceConfigBool(
	ctx context.Context,
	settings configSvc.SettingsService,
	key string,
	fallback bool,
) bool {
	if settings == nil {
		return fallback
	}
	value, err := settings.ResolveBool(ctx, key)
	if err != nil {
		return fallback
	}
	return value
}
