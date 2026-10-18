package careplanhttp

import (
	errors "errors"
	http "net/http"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func parseStatusDayRange(r *http.Request) (calendar.Date, calendar.Date, error) {
	return parseStatusDayRangeAt(r, calendar.TodayDate())
}

func parseStatusDayRangeAt(r *http.Request, today calendar.Date) (calendar.Date, calendar.Date, error) {
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")

	from := today
	// Two calendar months ahead, mirroring time.Time.AddDate(0, 2, 0).
	to := calendar.NewDate(today.Year(), today.Month()+2, today.Day())
	var err error
	if fromRaw != "" {
		if from, err = calendar.ParseDate(fromRaw); err != nil {
			return calendar.Date(""), calendar.Date(""), errors.New("invalid from date format, expected YYYY-MM-DD")
		}
	}
	if toRaw != "" {
		if to, err = calendar.ParseDate(toRaw); err != nil {
			return calendar.Date(""), calendar.Date(""), errors.New("invalid to date format, expected YYYY-MM-DD")
		}
	}
	if to.Before(from) {
		return calendar.Date(""), calendar.Date(""), errors.New("to must be after from")
	}
	return from, to, nil
}
