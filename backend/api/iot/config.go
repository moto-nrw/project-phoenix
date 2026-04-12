package iot

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/auth/device"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
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
type deviceConfigResponse struct {
	Checkout deviceConfigCheckout `json:"checkout"`
	Feedback deviceConfigFeedback `json:"feedback"`
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

	// Resolve checkout button settings (defaults to true if service unavailable)
	raumwechsel := true
	schulhof := true
	wc := true
	feedbackEnabled := false

	if rs.SettingsService != nil {
		if val, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyCheckoutRaumwechselEnabled); err == nil {
			raumwechsel = val
		}
		if val, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyCheckoutSchulhofEnabled); err == nil {
			schulhof = val
		}
		if val, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyCheckoutWCEnabled); err == nil {
			wc = val
		}
		if val, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyFeedbackEnabled); err == nil {
			feedbackEnabled = val
		}
	}

	// Resolve daily checkout time via the shared helper so the fallback chain
	// (tenant DB override → env var → nil) lives in a single place.
	var dailyCheckoutTime *string
	if rawTime := checkinAPI.ResolveRawDailyCheckoutTime(r.Context(), rs.SettingsService); rawTime != "" {
		dailyCheckoutTime = &rawTime
	}

	response := deviceConfigResponse{
		Checkout: deviceConfigCheckout{
			RaumwechselEnabled: raumwechsel,
			SchulhofEnabled:    schulhof,
			WCEnabled:          wc,
			DailyCheckoutTime:  dailyCheckoutTime,
		},
		Feedback: deviceConfigFeedback{
			Enabled: feedbackEnabled,
		},
	}

	common.Respond(w, r, http.StatusOK, response, "Device configuration retrieved")
}
