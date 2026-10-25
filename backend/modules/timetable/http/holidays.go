package timetablehttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// maxHolidayRangeDays bounds the holiday window: a school year plus the weeks
// a calendar view shows around it.
const maxHolidayRangeDays = 400

// Holidays is the slice of the School Calendar the holiday list reads
// (#4775): the statutory holidays of the school's federal state and that
// state, so the school can check which holidays apply.
type Holidays interface {
	TenantHolidays(ctx context.Context, from, to string) ([]schoolcalendar.Holiday, error)
	TenantHolidayRegion(ctx context.Context) (schoolcalendar.HolidayRegion, error)
}

// HolidaysResponse is the wire shape of GET /holidays.
type HolidaysResponse struct {
	FederalState     string                   `json:"federal_state"`
	FederalStateName string                   `json:"federal_state_name"`
	Holidays         []schoolcalendar.Holiday `json:"holidays"`
}

// listHolidays handles GET /api/timetable/holidays?from=&to= — the statutory
// holidays of the tenant's federal state (operations.federal_state) for the
// school calendar views. The holidays are computed, never stored, and hold
// for the whole school, so every calendar reader may see them.
func (rs *Resource) listHolidays(w http.ResponseWriter, r *http.Request) {
	from, to, ok := parseDateWindow(w, r, maxHolidayRangeDays)
	if !ok {
		return
	}
	if rs.Holidays == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("holiday service is not configured")))
		return
	}
	region, err := rs.Holidays.TenantHolidayRegion(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Bundesland konnte nicht gelesen werden", err))
		return
	}
	holidays, err := rs.Holidays.TenantHolidays(r.Context(), from.String(), to.String())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Feiertage konnten nicht berechnet werden", err))
		return
	}
	if holidays == nil {
		holidays = []schoolcalendar.Holiday{}
	}
	common.Respond(w, r, http.StatusOK, HolidaysResponse{
		FederalState: region.Code, FederalStateName: region.Name, Holidays: holidays,
	}, "Holidays retrieved successfully")
}
