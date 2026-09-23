package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (e engine) RecordPickupDayExtension(ctx context.Context, input timetable.PickupDayExtension) error {
	exceptionID := input.PickupExceptionID
	return mapError(e.service.RecordPickupDayExtension(ctx, domain.PickupExtensionTask{
		StudentID: input.StudentID, PickupExceptionID: &exceptionID, Date: domain.Date(input.Date),
		PreviousPickup: input.PreviousPickup, Pickup: input.Pickup,
	}))
}

func (e engine) ClearPickupDayExtension(ctx context.Context, studentID int64, date string) error {
	return mapError(e.service.ClearPickupDayExtension(ctx, studentID, domain.Date(date)))
}

func (e engine) RecordPickupWeekdayExtension(ctx context.Context, input timetable.PickupWeekdayExtension) error {
	return mapError(e.service.RecordPickupWeekdayExtension(ctx, domain.PickupExtensionTask{
		StudentID: input.StudentID, Weekday: input.Weekday, EffectiveFrom: domain.Date(input.EffectiveFrom),
		PreviousPickup: input.PreviousPickup, Pickup: input.Pickup,
	}))
}

func (e engine) ClearPickupWeekdayExtension(ctx context.Context, studentID int64, weekday int) error {
	return mapError(e.service.ClearPickupWeekdayExtension(ctx, studentID, weekday))
}

func (e engine) ListOpenPickupExtensions(ctx context.Context, studentID int64) ([]timetable.PickupExtensionTask, error) {
	values, err := e.service.ListOpenPickupExtensions(ctx, studentID)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.PickupExtensionTask, 0, len(values))
	for _, value := range values {
		result = append(result, pickupExtensionTaskToPublic(value))
	}
	return result, nil
}

func (e engine) FindPickupExtensionStudent(ctx context.Context, taskID int64) (int64, error) {
	studentID, err := e.service.FindPickupExtensionStudent(ctx, taskID)
	return studentID, mapError(err)
}

func (e engine) ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (timetable.PickupExtensionResolution, error) {
	value, err := e.service.ResolvePickupExtension(ctx, taskID, blockIDs)
	if err != nil {
		return timetable.PickupExtensionResolution{}, mapError(err)
	}
	return timetable.PickupExtensionResolution{
		StudentID:      value.Task.StudentID,
		Kind:           pickupExtensionKind(value.Task),
		AssignedBlocks: pickupExtensionBlocksToPublic(value.Assigned),
		InstanceIDs:    append([]int64{}, value.InstanceIDs...),
		Instances:      pickupExtensionInstancesToPublic(value.Instances),
	}, nil
}

func pickupExtensionTaskToPublic(value application.OpenPickupExtension) timetable.PickupExtensionTask {
	return timetable.PickupExtensionTask{
		ID:             value.Task.ID,
		StudentID:      value.Task.StudentID,
		Kind:           pickupExtensionKind(value.Task),
		Date:           value.Task.Date.String(),
		Weekday:        value.Task.Weekday,
		EffectiveFrom:  value.Task.EffectiveFrom.String(),
		PreviousPickup: value.Task.PreviousPickup,
		Pickup:         value.Task.Pickup,
		Blocks:         pickupExtensionBlocksToPublic(value.Blocks),
	}
}

func pickupExtensionKind(task domain.PickupExtensionTask) string {
	if task.IsDay() {
		return timetable.PickupExtensionKindDay
	}
	return timetable.PickupExtensionKindWeekday
}

func pickupExtensionBlocksToPublic(values []domain.PickupExtensionBlock) []timetable.PickupExtensionBlock {
	result := make([]timetable.PickupExtensionBlock, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PickupExtensionBlock{
			ID: value.ID, Title: value.Title, StartTime: value.StartTime, EndTime: value.EndTime,
		})
	}
	return result
}

func pickupExtensionInstancesToPublic(values []domain.PickupExtensionInstance) []timetable.PickupExtensionInstance {
	result := make([]timetable.PickupExtensionInstance, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PickupExtensionInstance{ID: value.ID, Date: value.Date.String()})
	}
	return result
}
