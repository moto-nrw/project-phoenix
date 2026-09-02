package timetracking

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// getStaffMonthSummary handles GET /api/staff/{id}/time-tracking/month-summary?year=&month=
// — the admin Monatskarte (#1842). Everything is computed on read; the
// Übertrag is live.
func (rs *StaffAdminResource) getStaffMonthSummary(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}
	year, month, err := ParseYearMonthQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	summary, err := rs.WorkTimeMonthService.GetMonthSummary(r.Context(), staffID, year, month)
	if err != nil {
		if errors.Is(err, activeSvc.ErrMonthOutOfRange) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		rs.logger.Error("failed to get staff month summary",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, summary, "Month summary retrieved successfully")
}
