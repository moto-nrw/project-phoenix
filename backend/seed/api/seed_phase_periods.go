package api

import (
	"fmt"
	"strconv"
	"time"
)

// seedSchoolYearStart is the year the school year holding day starts in. The
// school year runs from 1 August to 31 July, as in the planning bootstrap.
func seedSchoolYearStart(day seedDate) int {
	if day.Month() < time.August {
		return day.Year() - 1
	}
	return day.Year()
}

// seedPhaseSchoolYear returns the school year starting in startYear as a
// planning period, creating it when the school has none yet. The demo's
// enrollment phases link to it (#3924): unlinked, Planung > Zeiträume lists
// the school year without its Anmeldephase, and the period dialog shows the
// phase unticked, which reads as if enrollment were switched off.
func seedPhaseSchoolYear(rt *Runtime, auth AuthRef, startYear int) (seedCalendarPeriod, error) {
	startDate := fmt.Sprintf("%d-08-01", startYear)
	endDate := fmt.Sprintf("%d-07-31", startYear+1)
	raw, err := rt.Client.PostWithAuth(auth, "/api/timetable/periods/bootstrap", nil)
	if err != nil {
		return seedCalendarPeriod{}, fmt.Errorf("bootstrap planning periods: %w", err)
	}
	var response struct {
		Data struct {
			Periods []seedCalendarPeriod `json:"periods"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return seedCalendarPeriod{}, fmt.Errorf("parse planning periods: %w", err)
	}
	for _, period := range response.Data.Periods {
		if period.PeriodType == "school_year" && period.StartDate == startDate && period.EndDate == endDate {
			return period, nil
		}
	}
	raw, err = rt.Client.PostWithAuth(auth, "/api/timetable/periods", map[string]any{
		"name":        fmt.Sprintf("Schuljahr %d/%d", startYear, startYear+1),
		"period_type": "school_year", "start_date": startDate, "end_date": endDate,
		"week_cycle_length": 1, "is_active": true,
	})
	if err != nil {
		return seedCalendarPeriod{}, fmt.Errorf("create school year %d/%d: %w", startYear, startYear+1, err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return seedCalendarPeriod{}, fmt.Errorf("parse school year %d/%d: %w", startYear, startYear+1, err)
	}
	return seedCalendarPeriod{ID: id, PeriodType: "school_year", StartDate: startDate, EndDate: endDate, IsActive: true}, nil
}

// linkPhaseToPeriod points a phase body at its planning period. The phase
// covers exactly the period, so its Betreuungszeitraum and the Zeitraum
// under Planung show the same dates.
func linkPhaseToPeriod(body map[string]any, period seedCalendarPeriod) {
	body["calendar_period_id"] = strconv.FormatInt(period.ID, 10)
	body["service_start_date"] = period.StartDate
	body["service_end_date"] = period.EndDate
}
