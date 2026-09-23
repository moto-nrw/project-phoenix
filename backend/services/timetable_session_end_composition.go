package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewTimetableEndedSessionCompletion composes the Timetable owner's
// completion of blocks behind ended live sessions (#1747, #3551) over the
// retained instance and participant rows, which carry the Student Presence
// session state. careDays is optional: without it no child is spared the
// absent stamp.
func NewTimetableEndedSessionCompletion(
	instances scheduleModels.ActivityInstanceRepository,
	participants scheduleModels.InstanceStudentRepository,
	careDays careplan.CareDayQuery,
) (timetable.EndedSessionCompletion, error) {
	listing, ok := instances.(timetableCompose.EndedSessionInstances)
	if !ok {
		return nil, fmt.Errorf("ended session completion: %T cannot list instances by session", instances)
	}
	return timetableCompose.NewEndedSessionCompletion(timetableCompose.EndedSessionCompletionDependencies{
		Instances:    listing,
		Participants: participants,
		CareDays:     newTimetableCareDays(careDays),
	})
}

// newTimetableCareDays serves the Timetable owner's care-day port from Care
// Plan's verdict and rules; nil when Care Plan is not wired.
func newTimetableCareDays(query careplan.CareDayQuery) timetableCompose.CareDays {
	if query == nil {
		return nil
	}
	return timetableCareDays{query: query}
}

// timetableCareDays is the Care Plan binding of the Timetable care-day port:
// the verdict and every rule that reads it stay Care Plan's.
type timetableCareDays struct {
	query careplan.CareDayQuery
}

func (c timetableCareDays) ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]timetable.CareDayStatus, error) {
	resolved, err := c.query.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]timetable.CareDayStatus, len(resolved))
	for studentID, status := range resolved {
		result[studentID] = timetable.CareDayStatus(status)
	}
	return result, nil
}

func (c timetableCareDays) ResolveForRange(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[int64]map[timezone.Date]timetable.CareDayStatus, error) {
	resolved, err := c.query.ResolveForRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[timezone.Date]timetable.CareDayStatus, len(resolved))
	for studentID, days := range resolved {
		byDate := make(map[timezone.Date]timetable.CareDayStatus, len(days))
		for date, status := range days {
			byDate[date] = timetable.CareDayStatus(status)
		}
		result[studentID] = byDate
	}
	return result, nil
}

func (timetableCareDays) AttendanceRowCareDay(instanceCompleted bool, row timetable.CareDayAttendance, planVerdict timetable.CareDayStatus) timetable.CareDayStatus {
	return timetable.CareDayStatus(careplan.AttendanceRowCareDay(instanceCompleted, &careplan.CareDayAttendance{
		Expected:         row.Expected,
		NotScheduled:     row.NotScheduled,
		ManuallyDecided:  row.ManuallyDecided,
		PlanOwnedAbsence: row.PlanOwnedAbsence,
	}, careplan.CareDayStatus(planVerdict)))
}

func (timetableCareDays) Expected(status timetable.CareDayStatus) bool {
	return careplan.CareDayStatus(status).Expected()
}

func (timetableCareDays) ExemptFromAbsence(status timetable.CareDayStatus) bool {
	return careplan.CareDayStatus(status).ExemptFromAbsence()
}
