package presenceprojectionhttp

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
// status rows onto the responses only with users:read. The rows are those
// that stand for the day, active or archived at its end, as the day log and
// the class day read them (#6291). Errors are returned:
// on a non-today view resetScheduledStatusFlags has already cleared the
// row-seeded flags, so a silent lookup failure would report a sick, excused, or
// class-trip child as expected. Callers must surface the error (500) instead of
// serving an incorrect attendance plan (#1939).
func (rs *Resource) applyStatusDaysForDate(ctx context.Context, responses []StudentResponse, now time.Time) error {
	if !canReadSensitiveStudentData(ctx) || rs.StudentStatusDayService == nil || len(responses) == 0 {
		return nil
	}

	studentIDs := make([]int64, 0, len(responses))
	for _, response := range responses {
		studentIDs = append(studentIDs, response.ID)
	}
	rows, err := rs.StudentStatusDayService.FindSignedOffByStudentIDsAndDate(ctx, studentIDs, calendar.DateFromTime(now))
	if err != nil {
		return err
	}
	applyEffectiveStatusDaysToResponses(responses, rows)
	return nil
}

func (rs *Resource) applyStatusDaysForDateToResponse(ctx context.Context, response *StudentResponse, now time.Time) error {
	if !canReadSensitiveStudentData(ctx) || response == nil || rs.StudentStatusDayService == nil {
		return nil
	}
	date := calendar.DateFromTime(now)
	rows, err := rs.StudentStatusDayService.FindSignedOffByStudentIDsAndDate(ctx, []int64{response.ID}, date)
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
