package presenceprojectionhttp

import (
	errors "errors"
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (rs *Resource) getStudentStatusDays(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	// Whoever may WRITE this child's absences may also read them: the planning
	// dialog refuses to save until it has checked the existing status days, so
	// a caller authorized only by the open-care absence gate (#2232) would see
	// the actions and then be unable to use them. The payload is exactly the
	// absence data that gate covers — no Stammdaten ride along.
	if !rs.checkStudentReadAccess(r, student) && !rs.checkStudentAbsenceWriteAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("full access required")))
		return
	}
	if rs.StudentStatusDayService == nil {
		common.Respond(w, r, http.StatusOK, []studentpresence.StudentStatusDayResponse{}, "Student status days retrieved successfully")
		return
	}

	from, to, err := parseStatusDayRange(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	rows, err := rs.StudentStatusDayService.FindActiveByStudentAndDateRange(r.Context(), student.ID, from, to)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to fetch student status days", err))
		return
	}

	common.Respond(w, r, http.StatusOK, studentpresence.NewStudentStatusDayResponses(rows), "Student status days retrieved successfully")
}

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
