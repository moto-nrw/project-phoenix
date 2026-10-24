//go:build testhooks

package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// nativeFixtureTimeframe writes fixture timeframes through the owner's
// timeframe capability.
type nativeFixtureTimeframe struct {
	timeframes timetable.TimeframeCapability
}

func NativeTimeframeRecords(timeframes timetable.TimeframeCapability) ports.NativeTimeframeRecords {
	return nativeFixtureTimeframe{timeframes: timeframes}
}

func (r nativeFixtureTimeframe) Create(ctx context.Context, value *timetable.Timeframe) error {
	if value == nil {
		return errors.New("timeframe cannot be nil or zero value")
	}
	created, err := r.timeframes.CreateTimeframe(ctx, fixtureTimeframeInput(*value))
	if err != nil {
		return WrapDatabaseError("create", err)
	}
	*value = created
	return nil
}

func (r nativeFixtureTimeframe) Update(ctx context.Context, value *timetable.Timeframe) error {
	if value == nil {
		return errors.New("timeframe cannot be nil or zero value")
	}
	updated, err := r.timeframes.UpdateTimeframe(ctx, value.ID, fixtureTimeframeInput(*value))
	if err != nil {
		return fixtureTimeframeError("update", err)
	}
	*value = updated
	return nil
}

func (r nativeFixtureTimeframe) FindByID(ctx context.Context, id any) (*timetable.Timeframe, error) {
	timeframeID, ok := LegacyGroupID(id)
	if !ok {
		return nil, WrapDatabaseError("find by id", fmt.Errorf("invalid timeframe id %T", id))
	}
	value, err := r.timeframes.FindTimeframe(ctx, timeframeID)
	if err != nil {
		return nil, fixtureTimeframeError("find by id", err)
	}
	return &value, nil
}

func fixtureTimeframeInput(value timetable.Timeframe) timetable.TimeframeInput {
	return timetable.TimeframeInput{StartTime: value.StartTime, EndTime: value.EndTime, IsActive: value.IsActive, Description: value.Description}
}

func fixtureTimeframeError(operation string, err error) error {
	if errors.Is(err, timetable.ErrTimeframeNotFound) {
		return WrapNotFoundDatabaseError(operation)
	}
	return WrapDatabaseError(operation, err)
}
