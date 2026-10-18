package presenceprojectionhttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// CountAtSchoolToday counts the given students who read "Schule" right now.
// The dashboard hands in its "Zuhause" candidates (active, not present, not
// sick or excused) and moves this many into its own tile (#3260). It runs the
// same loaders and rule as the student list, so both agree on every child.
func (rs *Resource) CountAtSchoolToday(ctx context.Context, studentIDs []int64) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	today := rs.todayDate()
	attendances, err := rs.ActiveService.GetStudentsAttendanceStatuses(ctx, studentIDs)
	if err != nil {
		return 0, err
	}
	arrivals, err := rs.loadDayPlanningArrivals(ctx, studentIDs, today)
	if err != nil {
		return 0, err
	}
	pickups, err := rs.loadDayPlanningPickups(ctx, studentIDs, today)
	if err != nil {
		return 0, err
	}
	timetableIDs, err := rs.loadDayPlanningTimetableIDs(ctx, studentIDs, today)
	if err != nil {
		return 0, err
	}
	careDayEnd, err := rs.SettingsService.ResolveString(ctx, settingSessionEndTime)
	if err != nil {
		return 0, err
	}

	responses := make([]StudentResponse, len(studentIDs))
	for i, id := range studentIDs {
		responses[i] = StudentResponse{ID: id, HasFullAccess: true, Location: studentpresence.AbsentLocationLabel}
	}
	applyDayPlanning(responses, arrivals, pickups, attendances, timetableIDs, true)
	applyAtSchoolLocation(responses, pickups, attendances, careDayEnd, rs.Now())

	count := 0
	for i := range responses {
		if responses[i].Location == studentpresence.AtSchoolLocationLabel {
			count++
		}
	}
	return count, nil
}
