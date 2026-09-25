package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// PhaseResponseRowResponse is one expected child of the response overview
// (#3379). request_id is set once the family answered; pending_request_id
// points at a rollover row that still waits for the family or the school.
type PhaseResponseRowResponse struct {
	StudentID        string  `json:"student_id"`
	FirstName        string  `json:"first_name"`
	LastName         string  `json:"last_name"`
	SchoolClass      string  `json:"school_class"`
	HasParentApp     bool    `json:"has_parent_app"`
	Responded        bool    `json:"responded"`
	RequestID        *string `json:"request_id,omitempty"`
	PendingRequestID *string `json:"pending_request_id,omitempty"`
	ChildStatus      string  `json:"child_status,omitempty"`
}

// PhaseResponseExclusionResponse counts the children left out for one reason.
type PhaseResponseExclusionResponse struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// PhaseResponseOverviewResponse is the wire shape of
// GET /enrollment/phases/{id}/responses.
type PhaseResponseOverviewResponse struct {
	Applicable bool                             `json:"applicable"`
	Expected   int                              `json:"expected"`
	Responded  int                              `json:"responded"`
	Children   []PhaseResponseRowResponse       `json:"children"`
	Excluded   []PhaseResponseExclusionResponse `json:"excluded"`
}

func toPhaseResponseOverviewResponse(overview *capability.PhaseResponseOverview) PhaseResponseOverviewResponse {
	response := PhaseResponseOverviewResponse{
		Applicable: overview.Applicable,
		Expected:   overview.Expected,
		Responded:  overview.Responded,
		Children:   make([]PhaseResponseRowResponse, 0, len(overview.Rows)),
		Excluded:   make([]PhaseResponseExclusionResponse, 0, len(overview.Excluded)),
	}
	for _, row := range overview.Rows {
		response.Children = append(response.Children, PhaseResponseRowResponse{
			StudentID: strconv.FormatInt(row.StudentID, 10), FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, HasParentApp: row.HasParentApp, Responded: row.Responded,
			RequestID: stringID(row.RequestID), PendingRequestID: stringID(row.PendingRequestID), ChildStatus: row.ChildStatus,
		})
	}
	for _, exclusion := range overview.Excluded {
		response.Excluded = append(response.Excluded, PhaseResponseExclusionResponse{
			Reason: exclusion.Reason, Count: exclusion.Count,
		})
	}
	return response
}

func stringID(id *int64) *string {
	if id == nil {
		return nil
	}
	value := strconv.FormatInt(*id, 10)
	return &value
}

func (rs *Resource) getPhaseResponseOverview(w http.ResponseWriter, r *http.Request) {
	if rs.PhaseService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("phase service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	var overview *capability.PhaseResponseOverview
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		value, err := rs.PhaseService.ResponseOverview(ctx, id)
		overview = value
		return err
	})
	if err != nil {
		if errors.Is(err, capability.ErrPhaseNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toPhaseResponseOverviewResponse(overview), "Phase responses retrieved")
}
