package timetracking

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// maxHolidayRangeDays bounds the holidays query window; the UI never needs
// more than a year plus surrounding weeks at once.
const maxHolidayRangeDays = 400

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
	if from.DaysUntil(to) > maxHolidayRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("range must not exceed 400 days")))
		return
	}

	holidayList, err := rs.HolidayService.HolidaysInRange(r.Context(), from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, holidayList, "Holidays retrieved successfully")
}
