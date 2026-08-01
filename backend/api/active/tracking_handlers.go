package active

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// getTrackingIndicators handles POST /tracking-indicators
// Returns per-student match results for configured tracking indicator labels.
func (rs *Resource) getTrackingIndicators(w http.ResponseWriter, r *http.Request) {
	var req TrackingIndicatorsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	if len(req.StudentIDs) == 0 {
		common.Respond(w, r, http.StatusOK, TrackingIndicatorsResponse{
			Labels:  []string{},
			Results: map[int64][]bool{},
		}, "")
		return
	}

	for _, id := range req.StudentIDs {
		if id <= 0 {
			common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid student ID")))
			return
		}
	}

	// One batch query for the toggle plus the three label keys; the per-key
	// reads below then resolve from the snapshot (issue #2065).
	ctx := common.PrefetchSettings(r.Context(), rs.SettingsService,
		configModel.KeyTrackingIndicatorsEnabled,
		configModel.KeyTrackingIndicator1,
		configModel.KeyTrackingIndicator2,
		configModel.KeyTrackingIndicator3,
	)

	// Check if tracking indicators are enabled for this tenant.
	enabled, err := rs.SettingsService.ResolveBool(ctx, configModel.KeyTrackingIndicatorsEnabled)
	if err != nil {
		rs.getLogger().Warn("tracking_indicators_settings_error",
			"error", err.Error(),
		)
		common.Respond(w, r, http.StatusOK, TrackingIndicatorsResponse{
			Labels:  []string{},
			Results: map[int64][]bool{},
		}, "")
		return
	}

	if !enabled {
		common.Respond(w, r, http.StatusOK, TrackingIndicatorsResponse{
			Labels:  []string{},
			Results: map[int64][]bool{},
		}, "")
		return
	}

	// Read configured indicator labels.
	labelKeys := []string{
		configModel.KeyTrackingIndicator1,
		configModel.KeyTrackingIndicator2,
		configModel.KeyTrackingIndicator3,
	}
	var labels []string
	for _, key := range labelKeys {
		val, err := rs.SettingsService.ResolveString(ctx, key)
		if err != nil {
			continue
		}
		trimmed := strings.TrimSpace(val)
		if trimmed != "" {
			labels = append(labels, trimmed)
		}
	}

	if len(labels) == 0 {
		common.Respond(w, r, http.StatusOK, TrackingIndicatorsResponse{
			Labels:  []string{},
			Results: map[int64][]bool{},
		}, "")
		return
	}

	// Get tracking indicator results from the service.
	results, err := rs.ActiveService.GetTrackingIndicators(ctx, req.StudentIDs, labels)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, TrackingIndicatorsResponse{
		Labels:  labels,
		Results: results,
	}, "")
}
