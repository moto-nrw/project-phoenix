package api

// The root binds the live group's planned offerings (#6487) to three
// Timetable reads: the planned participants of the day's blocks, the blocks
// themselves and their block types. All are set reads, so the statement count
// does not grow with the group.

import (
	"context"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/grouplive"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
)

// groupLiveTimetable is the Timetable read of the planned participants and
// their blocks.
type groupLiveTimetable interface {
	ListPlannedInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.PlannedInstanceStudent, error)
	ListActivityInstances(ctx context.Context, filter timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error)
}

// groupLiveBlockTypes is the Timetable read of each block's display data;
// the card reads only the block type from it.
type groupLiveBlockTypes interface {
	ListBlockDisplayMetadata(ctx context.Context, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error)
}

// PlannedActivities lists, per child, the day's offerings it is planned for,
// without cancelled ones and without care or duty blocks, by start time.
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
	if err != nil || len(instances) == 0 {
		return result, err
	}
	instances, err = p.offeringsOnly(ctx, instances)
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

// offeringsOnly keeps the blocks whose type is an offering, in their order.
func (p groupLivePlanning) offeringsOnly(ctx context.Context, instances []timetable.ActivityInstance) ([]timetable.ActivityInstance, error) {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	metadata, err := p.blockTypes.ListBlockDisplayMetadata(ctx, ids)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(instances, func(instance timetable.ActivityInstance) bool {
		return !timetable.IsOfferingType(metadata[instance.ID].ActivityType)
	}), nil
}

// wallClockMinutes cuts a TIME text ("14:00:00") to the card's "14:00".
func wallClockMinutes(clock string) string {
	if len(clock) > len("15:04") {
		return clock[:len("15:04")]
	}
	return clock
}
