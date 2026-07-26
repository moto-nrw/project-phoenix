package staff

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	timetracking "github.com/moto-nrw/project-phoenix/api/time-tracking"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// getStaffScheduleTargets handles
// GET /api/staff/{id}/time-tracking/schedule-targets?from=&to= — the admin-side
// twin of the own endpoint, feeding the same date-valid Soll into the daily
// table that the Monatskarte is computed from (#1842).
func (rs *Resource) getStaffScheduleTargets(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}
	from, to, err := timetracking.ParseDateRangeQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	targets, err := rs.WorkTimeMonthService.GetDailyTargets(r.Context(), staffID, from, to)
	if err != nil {
		if errors.Is(err, activeSvc.ErrInvalidTargetRange) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		rs.getLogger().Error("failed to get staff schedule targets",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, targets, "Schedule targets retrieved successfully")
}
