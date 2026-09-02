package timetracking

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// getStaffScheduleTargets handles
// GET /api/staff/{id}/time-tracking/schedule-targets?from=&to=&target_only=true — the admin-side
// twin of the own endpoint, feeding the daily table the same per-day Soll,
// Gutschrift, Ist and Saldo the Monatskarte is computed from (#1842, #2443).
// The path keeps its name: it is the same resolution, now returning the whole
// day instead of only its target.
func (rs *StaffAdminResource) getStaffScheduleTargets(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}
	from, to, err := ParseDateRangeQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if r.URL.Query().Get("target_only") == "true" {
		targets, err := rs.WorkTimeMonthService.GetDailyTargets(r.Context(), staffID, from, to)
		if err != nil {
			rs.renderScheduleTargetsError(w, r, staffID, err)
			return
		}
		common.Respond(w, r, http.StatusOK, targets, "Schedule targets retrieved successfully")
		return
	}

	projection, err := rs.WorkTimeMonthService.GetDailyProjection(r.Context(), staffID, from, to)
	if err != nil {
		rs.renderScheduleTargetsError(w, r, staffID, err)
		return
	}
	common.Respond(w, r, http.StatusOK, projection, "Daily projection retrieved successfully")
}

func (rs *StaffAdminResource) renderScheduleTargetsError(w http.ResponseWriter, r *http.Request, staffID int64, err error) {
	if errors.Is(err, activeSvc.ErrInvalidTargetRange) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.logger.Error("failed to get staff schedule targets",
		"staff_id", staffID,
		"error", err.Error(),
	)
	common.RenderError(w, r, common.ErrorInternalServer(err))
}
