//go:build testhooks

package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// nativeFixtureException writes fixture exceptions through the owner's
// exception capability.
type nativeFixtureException struct {
	exceptions timetable.ActivityExceptionCapability
}

func NativeExceptionRecords(exceptions timetable.ActivityExceptionCapability) ports.NativeExceptionRecords {
	return nativeFixtureException{exceptions: exceptions}
}

func (r nativeFixtureException) Create(ctx context.Context, value *timetable.ActivityException) error {
	if value == nil {
		return errors.New("activity exception cannot be nil or zero value")
	}
	created, err := r.exceptions.CreateActivityException(ctx, fixtureExceptionInput(*value))
	if err != nil {
		return WrapDatabaseError("create", err)
	}
	*value = created
	return nil
}

func (r nativeFixtureException) Update(ctx context.Context, value *timetable.ActivityException) error {
	if value == nil {
		return errors.New("activity exception cannot be nil or zero value")
	}
	updated, err := r.exceptions.UpdateActivityException(ctx, value.ID, fixtureExceptionInput(*value))
	if err != nil {
		return legacyActivityExceptionError("update", err)
	}
	*value = updated
	return nil
}

func (r nativeFixtureException) FindByID(ctx context.Context, id any) (*timetable.ActivityException, error) {
	exceptionID, ok := LegacyGroupID(id)
	if !ok {
		return nil, WrapDatabaseError("find by id", fmt.Errorf("invalid activity exception id %T", id))
	}
	value, err := r.exceptions.FindActivityException(ctx, exceptionID)
	if err != nil {
		return nil, legacyActivityExceptionError("find by id", err)
	}
	return &value, nil
}

func fixtureExceptionInput(value timetable.ActivityException) timetable.ActivityExceptionInput {
	return timetable.ActivityExceptionInput{
		ActivityGroupID: value.ActivityGroupID, ExceptionDate: value.ExceptionDate, ExceptionType: value.ExceptionType,
		StartTime: value.StartTime, EndTime: value.EndTime, RoomID: value.RoomID, Reason: value.Reason, CreatedBy: value.CreatedBy,
	}
}
