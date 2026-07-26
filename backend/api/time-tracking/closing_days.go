package timetracking

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// maxClosingDayRangeDays bounds the closing-days query window, mirroring the
// holidays endpoint; the UI never needs more than a year plus surrounding
// weeks at once.
const maxClosingDayRangeDays = 400

// closingDayRangeResponse is the wire shape of GET /closing-days: the stored
// ranges overlapping the window. The frontend expands them into per-day
// entries for badge display.
type closingDayRangeResponse struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Reason    string `json:"reason"`
}

// getClosingDays handles GET /api/time-tracking/closing-days?from=&to= — the
// tenant's closure periods (#1418 3b) for calendar marking. Like /holidays,
// the data is tenant-global, not staff-specific, so own- and manage-scoped
// callers share the endpoint.
func (rs *Resource) getClosingDays(w http.ResponseWriter, r *http.Request) {
	from, to, err := ParseDateRangeQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if from.DaysUntil(to) > maxClosingDayRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("range must not exceed 400 days")))
		return
	}

	days, err := rs.ClosingDayService.ClosingDaysInRange(r.Context(), from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]closingDayRangeResponse, len(days))
	for i, d := range days {
		responses[i] = mapClosingDayRange(d)
	}

	common.Respond(w, r, http.StatusOK, responses, "Closing days retrieved successfully")
}

func mapClosingDayRange(d *schedule.ClosingDay) closingDayRangeResponse {
	return closingDayRangeResponse{
		StartDate: d.StartDate.String(),
		EndDate:   d.EndDate.String(),
		Reason:    d.Reason,
	}
}
