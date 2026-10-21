package presenceprojectionhttp

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func applyEffectiveStatusDaysToResponses(responses []StudentResponse, statusRows []*absencerecords.StudentStatusDay) {
	if len(responses) == 0 || len(statusRows) == 0 {
		return
	}

	rowsByStudent := make(map[int64][]*absencerecords.StudentStatusDay, len(statusRows))
	for _, row := range statusRows {
		rowsByStudent[row.StudentID] = append(rowsByStudent[row.StudentID], row)
	}

	for i := range responses {
		applyEffectiveStatusDays(&responses[i], rowsByStudent[responses[i].ID])
	}
}

// applyStatusDaysForDate overlays the requested day's sick/excused/class-trip
// status rows onto the responses. The error is returned rather than swallowed:
// on a non-today view resetScheduledStatusFlags has already cleared the
// row-seeded flags, so a silent lookup failure would report a sick, excused, or
// class-trip child as expected. Callers must surface the error (500) instead of
// serving an incorrect attendance plan (#1939).
func (rs *Resource) applyStatusDaysForDate(ctx context.Context, responses []StudentResponse, now time.Time) error {
	if rs.StudentStatusDayService == nil || len(responses) == 0 {
		return nil
	}

	studentIDs := make([]int64, 0, len(responses))
	for _, response := range responses {
		studentIDs = append(studentIDs, response.ID)
	}
	rows, err := rs.StudentStatusDayService.FindActiveByStudentIDsAndDate(ctx, studentIDs, calendar.DateFromTime(now))
	if err != nil {
		return err
	}
	applyEffectiveStatusDaysToResponses(responses, rows)
	return nil
}

func (rs *Resource) applyStatusDaysForDateToResponse(ctx context.Context, response *StudentResponse, now time.Time) error {
	if response == nil || rs.StudentStatusDayService == nil {
		return nil
	}
	date := calendar.DateFromTime(now)
	rows, err := rs.StudentStatusDayService.FindActiveByStudentAndDateRange(ctx, response.ID, date, date)
	if err != nil {
		return err
	}
	applyEffectiveStatusDays(response, rows)
	return nil
}

func applyEffectiveStatusDays(response *StudentResponse, statusRows []*absencerecords.StudentStatusDay) {
	if response == nil || len(statusRows) == 0 {
		return
	}

	eff := studentpresence.ResolveEffectiveStatus(statusRows)
	if eff.Sick {
		response.Sick = true
		response.SickSince = eff.SickSince
		response.ClassTrip = false
		response.ClassTripSince = nil
		response.Excused = false
		response.ExcusedSince = nil
		return
	}

	if eff.ClassTrip {
		response.ClassTrip = true
		response.ClassTripSince = eff.ClassTripSince
		response.Sick = false
		response.SickSince = nil
		response.Excused = false
		response.ExcusedSince = nil
		return
	}

	if eff.Excused && !response.Sick {
		response.Excused = true
		response.ExcusedSince = eff.ExcusedSince
	}
}
