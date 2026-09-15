package presence

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
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

	ctx, labels, err := common.TrackingIndicatorLabels(r.Context(), rs.SettingsService)
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

	if len(labels) == 0 {
		common.Respond(w, r, http.StatusOK, TrackingIndicatorsResponse{
			Labels:  []string{},
			Results: map[int64][]bool{},
		}, "")
		return
	}

	// Get tracking indicator results from the service.
	results, err := rs.Operations.TrackingIndicators(ctx, req.StudentIDs, labels)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, TrackingIndicatorsResponse{
		Labels:  labels,
		Results: results,
	}, "")
}
