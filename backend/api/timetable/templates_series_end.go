package timetable

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Optional series end (#3594): a series may stop before its planning period
// ends, e.g. a holiday-care week. The request names the inclusive last day,
// stored as activities.groups.series_last_day; materialization plans nothing
// after it. It is not a schedule valid_until, which marks a capped segment.

// parseSeriesEndDate reads an end_date value (YYYY-MM-DD).
func parseSeriesEndDate(w http.ResponseWriter, r *http.Request, raw *string) (*timezone.Date, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	parsed, err := berlinDate(*raw)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return nil, false
	}
	return &parsed, true
}

// validateSeriesEnd checks the last day against the series start and the
// planning period; nil (no end) always passes.
func (rs *Resource) validateSeriesEnd(
	w http.ResponseWriter,
	r *http.Request,
	lastDay *timezone.Date,
	firstDay timezone.Date,
	calendarPeriodID *int64,
) bool {
	if lastDay == nil {
		return true
	}
	var periodEnd *timezone.Date
	if calendarPeriodID != nil {
		if rs.CalendarPeriods == nil {
			common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar period service not wired")))
			return false
		}
		period, err := rs.CalendarPeriods.FindCalendarPeriod(r.Context(), *calendarPeriodID)
		if err != nil {
			renderTemplatePeriodLookupError(w, r, err)
			return false
		}
		end := timezone.Date(period.EndDate)
		periodEnd = &end
	}
	if err := timetableModule.ValidateSeriesLastDay(*lastDay, firstDay, periodEnd); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return false
	}
	return true
}

// seriesLastDayString is the stored form of a validated last day.
func seriesLastDayString(lastDay *timezone.Date) *string {
	if lastDay == nil {
		return nil
	}
	day := lastDay.String()
	return &day
}

// seriesLastDayActivityDate is the create-input form of a validated last day.
func seriesLastDayActivityDate(lastDay *timezone.Date) *activitiesModel.Date {
	if lastDay == nil {
		return nil
	}
	day := activitiesModel.Date(lastDay.String())
	return &day
}

// updateSeriesFirstDay is the lower bound for an edited last day: the pulled
// forward start when the edit moves it, otherwise the stored series start
// (zero when the series starts with its planning period).
func updateSeriesFirstDay(start *timezone.Date, stored templateResponse) timezone.Date {
	if start != nil {
		return *start
	}
	for _, schedule := range stored.Schedules {
		if schedule.ValidFrom != "" {
			return timezone.Date(schedule.ValidFrom)
		}
	}
	return timezone.Date("")
}

// prepareTemplateUpdate checks the edit against the stored series and fills in
// what the request left out: workdays, the target period, offering sources
// and, when sent, the series' last day (#3594).
func (rs *Resource) prepareTemplateUpdate(
	w http.ResponseWriter,
	r *http.Request,
	parsed *parsedUpdateTemplate,
	stored templateResponse,
) bool {
	if err := validateLegacyTemplateWorkdays(stored.Schedules, parsed.req.Weekdays); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return false
	}
	parsed.req.CalendarPeriodID = updateCalendarPeriodID(
		parsed.startDate, parsed.req.CalendarPeriodID, stored)
	applyOfferingSourcePresence(parsed.req, stored)
	if !parsed.req.EndDate.Set {
		return true
	}
	return rs.validateSeriesEnd(w, r, parsed.endDate, updateSeriesFirstDay(parsed.startDate, stored), parsed.req.CalendarPeriodID)
}
