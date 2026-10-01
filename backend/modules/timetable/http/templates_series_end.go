package timetablehttp

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Optional series end (#3594): a series may stop before its planning period
// ends, e.g. a holiday-care week. The request names the inclusive last day,
// stored as activities.groups.series_last_day; materialization plans nothing
// after it. It is not a schedule valid_until, which marks a capped segment.

// parseSeriesEndDate reads an end_date value (YYYY-MM-DD).
func parseSeriesEndDate(w http.ResponseWriter, r *http.Request, raw *string) (*calendar.Date, bool) {
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
	lastDay *calendar.Date,
	firstDay calendar.Date,
	calendarPeriodID *int64,
) bool {
	if lastDay == nil {
		return true
	}
	var periodEnd *calendar.Date
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
		end := calendar.Date(period.EndDate)
		periodEnd = &end
	}
	if err := timetableModule.ValidateSeriesLastDay(*lastDay, firstDay, periodEnd); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return false
	}
	return true
}

// seriesLastDayString is the stored form of a validated last day.
func seriesLastDayString(lastDay *calendar.Date) *string {
	if lastDay == nil {
		return nil
	}
	day := lastDay.String()
	return &day
}

// updateSeriesFirstDay is the lower bound for an edited last day: the pulled
// forward start when the edit moves it, otherwise the stored series start
// (zero when the series starts with its planning period). The stored rows
// arrive in weekday order, not by valid_from, so the earliest of them is the
// series start; a later split day must not reject a valid last day. A row
// without valid_from starts with the planning period and so precedes every
// split — it leaves no lower bound beyond the period itself.
func updateSeriesFirstDay(start *calendar.Date, stored templateResponse) calendar.Date {
	if start != nil {
		return *start
	}
	earliest := calendar.Date("")
	for _, schedule := range stored.Schedules {
		if schedule.ValidFrom == "" {
			return calendar.Date("")
		}
		validFrom := calendar.Date(schedule.ValidFrom)
		if earliest.IsZero() || validFrom.Before(earliest) {
			earliest = validFrom
		}
	}
	return earliest
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
