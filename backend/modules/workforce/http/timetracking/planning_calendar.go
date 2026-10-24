package timetracking

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// maxCalendarRangeDays bounds the holiday and closing-day query windows; the
// UI never needs more than a year plus surrounding weeks at once.
const maxCalendarRangeDays = 400

// closingDayRangeResponse is the wire shape of GET /closing-days: the stored
// ranges overlapping the window. The frontend expands them into per-day
// entries for badge display.
type closingDayRangeResponse struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Reason    string `json:"reason"`
}

// getHolidays handles GET /api/time-tracking/holidays?from=&to= — the
// tenant's public holidays (per the operations.federal_state setting) for
// calendar marking and the holiday-session warning (#1418 3a). The data is
// tenant-global, not staff-specific, so own- and manage-scoped callers share
// the endpoint.
func (rs *Resource) getHolidays(w http.ResponseWriter, r *http.Request) {
	from, to, err := ParseDateRangeQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if from.DaysUntil(to) > maxCalendarRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("range must not exceed 400 days")))
		return
	}

	holidayList, err := rs.Calendar.HolidaysInRange(r.Context(), from.String(), to.String())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, holidayList, "Holidays retrieved successfully")
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
	if from.DaysUntil(to) > maxCalendarRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("range must not exceed 400 days")))
		return
	}

	days, err := rs.Calendar.ClosingDaysInRange(r.Context(), from.String(), to.String())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]closingDayRangeResponse, len(days))
	for i, d := range days {
		responses[i] = closingDayRangeResponse{StartDate: d.StartDate, EndDate: d.EndDate, Reason: d.Reason}
	}

	common.Respond(w, r, http.StatusOK, responses, "Closing days retrieved successfully")
}
