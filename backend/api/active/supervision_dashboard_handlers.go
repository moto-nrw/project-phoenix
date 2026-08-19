package active

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	supervisionDashboardService "github.com/moto-nrw/project-phoenix/services/supervisiondashboard"
)

// Compatibility alias: the projection and its error contract live in the
// service layer (#2096), mirroring the OGS group live endpoint (#2056).
type SupervisionDashboardResponse = supervisionDashboardService.Projection

// getSupervisionDashboard handles GET /active/supervision-dashboard?group_id={id}.
// The service owns scope authorization, projection shaping, and the
// all-or-nothing sub-load contract; this boundary only parses input and maps
// domain errors.
func (rs *Resource) getSupervisionDashboard(w http.ResponseWriter, r *http.Request) {
	requestedGroupID, ok := parseOptionalDashboardGroupID(w, r)
	if !ok {
		return
	}
	if rs.SupervisionDashboardService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("supervision dashboard service is not configured")))
		return
	}

	response, err := rs.SupervisionDashboardService.Get(r.Context(), requestedGroupID)
	if errors.Is(err, supervisionDashboardService.ErrForbiddenGroup) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("you do not supervise this active group")))
		return
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to build supervision dashboard projection", err))
		return
	}
	common.Respond(w, r, http.StatusOK, response, "Supervision dashboard retrieved successfully")
}

func parseOptionalDashboardGroupID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("group_id"))
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid group_id")))
		return 0, false
	}
	return id, true
}
