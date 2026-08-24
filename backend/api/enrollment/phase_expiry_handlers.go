package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type PhaseExpiryWarningResponse struct {
	SourcePhaseID      string  `json:"source_phase_id"`
	SourcePhaseName    string  `json:"source_phase_name"`
	SuccessorPhaseID   *string `json:"successor_phase_id,omitempty"`
	SuccessorPhaseName *string `json:"successor_phase_name,omitempty"`
	FirstAffectedDate  string  `json:"first_affected_date"`
	AffectedChildren   int     `json:"affected_children"`
	UnresolvedChildren int     `json:"unresolved_children"`
	State              string  `json:"state"`
	Overdue            bool    `json:"overdue"`
}

func (rs *Resource) listPhaseExpiryWarnings(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseExpiryService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase expiry service not configured")))
		return
	}

	var warnings []*enrollmentService.PhaseExpiryWarning
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		var listErr error
		warnings, listErr = rs.PhaseExpiryService.ListWarnings(ctx, timezone.TodayDate())
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := make([]PhaseExpiryWarningResponse, 0, len(warnings))
	for _, warning := range warnings {
		response = append(response, toPhaseExpiryWarningResponse(warning))
	}
	common.Respond(w, r, http.StatusOK, response, "Phase expiry warnings retrieved")
}

func toPhaseExpiryWarningResponse(warning *enrollmentService.PhaseExpiryWarning) PhaseExpiryWarningResponse {
	response := PhaseExpiryWarningResponse{
		SourcePhaseID:      strconv.FormatInt(warning.SourcePhaseID, 10),
		SourcePhaseName:    warning.SourcePhaseName,
		SuccessorPhaseName: warning.SuccessorPhaseName,
		FirstAffectedDate:  warning.FirstAffectedDate.String(),
		AffectedChildren:   warning.AffectedChildren,
		UnresolvedChildren: warning.UnresolvedChildren,
		State:              warning.State,
		Overdue:            warning.Overdue,
	}
	if warning.SuccessorPhaseID != nil {
		id := strconv.FormatInt(*warning.SuccessorPhaseID, 10)
		response.SuccessorPhaseID = &id
	}
	return response
}
