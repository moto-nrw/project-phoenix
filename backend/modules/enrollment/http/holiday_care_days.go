package enrollmenthttp

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// renderHolidayWeekWithoutCare answers a week that contains a day without
// care at its host school (#4702) with 409 and the day it names, and reports
// whether err was such a refusal. The week is its position in the written
// period or booking, week_id its ID once stored.
func renderHolidayWeekWithoutCare(w http.ResponseWriter, r *http.Request, err error) bool {
	var refusal *capability.HolidayWeekNoCareDayRefusal
	if !errors.As(err, &refusal) {
		return false
	}
	common.RenderError(w, r, common.ErrorConflictWithDetails(err, common.CodeEnrollmentHolidayWeekNoCareDay, map[string]any{
		"week": refusal.Week, "week_id": refusal.WeekID, "date": refusal.Day.Date.Format("02.01.2006"),
		"iso_date": refusal.Day.Date.String(), "kind": refusal.Day.Kind, "name": refusal.Day.Name,
	}))
	return true
}

// noCareDaysOrEmpty answers an empty list for an unmarked week.
func noCareDaysOrEmpty(days []capability.HolidayNoCareDay) []capability.HolidayNoCareDay {
	if days == nil {
		return []capability.HolidayNoCareDay{}
	}
	return days
}
