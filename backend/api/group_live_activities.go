package api

// The root binds the live group's planned offerings (#6487) to two
// Timetable reads: the planned participants of the day's blocks, then the
// blocks themselves. Both are set reads, so the statement count does not grow
// with the group.

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// groupLiveTimetable is the Timetable read of the planned participants and
// their blocks.
type groupLiveTimetable interface {
	ListPlannedInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.PlannedInstanceStudent, error)
	ListActivityInstances(ctx context.Context, filter timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error)
}

// PlannedActivities lists, per child, the day's blocks it is planned for,
// without cancelled ones, ordered by start time.
func (p groupLivePlanning) PlannedActivities(ctx context.Context, studentIDs []int64, date grouplive.Date) (map[int64][]grouplive.PlannedActivity, error) {
	result := map[int64][]grouplive.PlannedActivity{}
	if len(studentIDs) == 0 {
		return result, nil
	}
	day := string(date)
	participants, err := p.timetable.ListPlannedInstanceStudents(ctx, timetable.InstanceStudentFilter{
		StudentIDs: studentIDs, Date: &day, ExcludeCancelled: true,
	})
	if err != nil || len(participants) == 0 {
		return result, err
	}
	instanceIDs := make([]int64, 0, len(participants))
	for _, participant := range participants {
		instanceIDs = append(instanceIDs, participant.InstanceID)
	}
	slices.Sort(instanceIDs)
	instances, err := p.timetable.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{
		IDs: slices.Compact(instanceIDs), Status: timetable.InstanceStatusPlanned, OrderByDateAndTime: true,
	})
	if err != nil {
		return nil, err
	}
	studentsByInstance := make(map[int64][]int64, len(instances))
	for _, participant := range participants {
		studentsByInstance[participant.InstanceID] = append(studentsByInstance[participant.InstanceID], participant.StudentID)
	}
	for _, instance := range instances {
		activity := grouplive.PlannedActivity{
			Title: instance.Title, StartTime: wallClockMinutes(instance.StartTime), EndTime: wallClockMinutes(instance.EndTime),
		}
		for _, studentID := range studentsByInstance[instance.ID] {
			result[studentID] = append(result[studentID], activity)
		}
	}
	return result, nil
}

// wallClockMinutes cuts a TIME text ("14:00:00") to the card's "14:00".
func wallClockMinutes(clock string) string {
	if len(clock) > len("15:04") {
		return clock[:len("15:04")]
	}
	return clock
}
