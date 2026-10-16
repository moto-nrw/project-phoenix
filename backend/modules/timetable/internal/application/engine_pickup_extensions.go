package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (e Engine) RecordPickupDayExtension(ctx context.Context, input timetable.PickupDayExtension) error {
	exceptionID := input.PickupExceptionID
	return MapError(e.service.RecordPickupDayExtension(ctx, domain.PickupExtensionTask{
		StudentID: input.StudentID, PickupExceptionID: &exceptionID, Date: domain.Date(input.Date),
		PreviousPickup: input.PreviousPickup, Pickup: input.Pickup,
	}))
}

func (e Engine) ClearPickupDayExtension(ctx context.Context, studentID int64, date string) error {
	return MapError(e.service.ClearPickupDayExtension(ctx, studentID, domain.Date(date)))
}

func (e Engine) RecordPickupWeekdayExtension(ctx context.Context, input timetable.PickupWeekdayExtension) error {
	return MapError(e.service.RecordPickupWeekdayExtension(ctx, domain.PickupExtensionTask{
		StudentID: input.StudentID, Weekday: input.Weekday, EffectiveFrom: domain.Date(input.EffectiveFrom),
		PreviousPickup: input.PreviousPickup, Pickup: input.Pickup,
	}))
}

func (e Engine) ClearPickupWeekdayExtension(ctx context.Context, studentID int64, weekday int) error {
	return MapError(e.service.ClearPickupWeekdayExtension(ctx, studentID, weekday))
}

func (e Engine) ListOpenPickupExtensions(ctx context.Context, studentID int64) ([]timetable.PickupExtensionTask, error) {
	values, err := e.service.ListOpenPickupExtensions(ctx, studentID)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.PickupExtensionTask, 0, len(values))
	for _, value := range values {
		result = append(result, pickupExtensionTaskToPublic(value))
	}
	return result, nil
}

func (e Engine) FindPickupExtensionStudent(ctx context.Context, taskID int64) (int64, error) {
	studentID, err := e.service.FindPickupExtensionStudent(ctx, taskID)
	return studentID, MapError(err)
}

func (e Engine) ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (timetable.PickupExtensionResolution, error) {
	value, err := e.service.ResolvePickupExtension(ctx, taskID, blockIDs)
	if err != nil {
		return timetable.PickupExtensionResolution{}, MapError(err)
	}
	return timetable.PickupExtensionResolution{
		StudentID:      value.Task.StudentID,
		Kind:           pickupExtensionKind(value.Task),
		AssignedBlocks: pickupExtensionBlocksToPublic(value.Assigned),
		InstanceIDs:    append([]int64{}, value.InstanceIDs...),
		Instances:      pickupExtensionInstancesToPublic(value.Instances),
	}, nil
}

func pickupExtensionTaskToPublic(value OpenPickupExtension) timetable.PickupExtensionTask {
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
