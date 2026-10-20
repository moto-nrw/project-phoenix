package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// slotException reads the exception of one template slot, or nil when the
// slot has none.
func (s *InstanceLifecycleService) slotException(ctx context.Context, activityGroupID int64, date calendar.Date) (*scheduleModel.ActivityException, error) {
	text := date.String()
	values, err := s.deps.ExceptionRepo.ListActivityExceptions(ctx, timetable.ActivityExceptionFilter{
		ActivityGroupID: &activityGroupID, ExceptionDate: &text, Limit: 1,
	})
	if err != nil {
		return nil, WrapDatabaseError("find by activity group and date", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	result, err := legacyActivityException(values[0])
	if err != nil {
		return nil, WrapDatabaseError("find by activity group and date", err)
	}
	return result, nil
}

// createSlotException stores a slot exception and copies the stored row
// back into value.
func (s *InstanceLifecycleService) createSlotException(ctx context.Context, value *scheduleModel.ActivityException) error {
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := s.deps.ExceptionRepo.CreateActivityException(ctx, publicActivityExceptionInput(value))
	if err != nil {
		return WrapDatabaseError("create", err)
	}
	return replaceLegacyActivityException(value, created)
}

// updateSlotException rewrites a slot exception and copies the stored row
// back into value.
func (s *InstanceLifecycleService) updateSlotException(ctx context.Context, value *scheduleModel.ActivityException) error {
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := s.deps.ExceptionRepo.UpdateActivityException(ctx, value.ID, publicActivityExceptionInput(value))
	if err != nil {
		return legacyActivityExceptionError("update", err)
	}
	return replaceLegacyActivityException(value, updated)
}
