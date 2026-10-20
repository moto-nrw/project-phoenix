//go:build testhooks

package application

import (
	"context"
	"fmt"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// nativeFixtureSchedule writes fixture schedules through the owner's
// schedule capability, with the legacy schedule checks the fixtures relied on.
type nativeFixtureSchedule struct {
	schedules timetable.ScheduleCapability
}

func NativeScheduleRecords(schedules timetable.ScheduleCapability) ports.NativeScheduleRecords {
	return nativeFixtureSchedule{schedules: schedules}
}

func (r nativeFixtureSchedule) Create(ctx context.Context, value *timetable.Schedule) error {
	return r.write(ctx, value, createLegacySchedule)
}

func (r nativeFixtureSchedule) Update(ctx context.Context, value *timetable.Schedule) error {
	return r.write(ctx, value, updateLegacySchedule)
}

func (r nativeFixtureSchedule) write(ctx context.Context, value *timetable.Schedule, write func(context.Context, timetable.ScheduleCommand, *activityContract.Schedule) error) error {
	var row *activityContract.Schedule
	if value != nil {
		row = legacySchedule(*value)
	}
	err := write(ctx, r.schedules, row)
	if value != nil {
		*value = *decodeFixtureSchedule(row)
	}
	return err
}

func (r nativeFixtureSchedule) FindByID(ctx context.Context, id any) (*timetable.Schedule, error) {
	scheduleID, ok := LegacyGroupID(id)
	if !ok {
		return nil, legacyDatabaseError("find by id", fmt.Errorf("invalid activity record id %T", id))
	}
	row, err := findLegacySchedule(ctx, r.schedules, scheduleID)
	return decodeFixtureSchedule(row), err
}

func (r nativeFixtureSchedule) FindByGroupID(ctx context.Context, id int64) ([]*timetable.Schedule, error) {
	rows, err := groupSchedules(ctx, r.schedules, id)
	return fixtureNativeRows(rows, decodeFixtureSchedule), err
}

func decodeFixtureSchedule(value *activityContract.Schedule) *timetable.Schedule {
	if value == nil {
		return nil
	}
	input := publicScheduleInput(value)
	return &timetable.Schedule{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Weekday: input.Weekday, TimeframeID: input.TimeframeID, ActivityGroupID: input.ActivityGroupID, WeekPattern: input.WeekPattern, CalendarPeriodID: input.CalendarPeriodID, ValidUntil: input.ValidUntil, ValidFrom: input.ValidFrom}
}
