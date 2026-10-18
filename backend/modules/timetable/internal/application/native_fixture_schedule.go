//go:build testhooks

package application

import (
	"context"

	models "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type nativeFixtureSchedule struct {
	fixtureRecord[timetable.Schedule, models.Schedule]
	rows models.ScheduleRepository
}

func NativeScheduleRecords(rows models.ScheduleRepository) ports.NativeScheduleRecords {
	return nativeFixtureSchedule{fixtureRecord: fixtureRecord[timetable.Schedule, models.Schedule]{rows: rows, encode: encodeFixtureSchedule, decode: decodeFixtureSchedule}, rows: rows}
}
func encodeFixtureSchedule(value *timetable.Schedule) (*models.Schedule, error) {
	if value == nil {
		return nil, nil
	}
	return legacySchedule(*value), nil
}
func decodeFixtureSchedule(value *models.Schedule) *timetable.Schedule {
	if value == nil {
		return nil
	}
	input := publicScheduleInput(value)
	return &timetable.Schedule{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Weekday: input.Weekday, TimeframeID: input.TimeframeID, ActivityGroupID: input.ActivityGroupID, WeekPattern: input.WeekPattern, CalendarPeriodID: input.CalendarPeriodID, ValidUntil: input.ValidUntil, ValidFrom: input.ValidFrom}
}
func (r nativeFixtureSchedule) FindByGroupID(ctx context.Context, id int64) ([]*timetable.Schedule, error) {
	rows, err := r.rows.FindByGroupID(ctx, id)
	return fixtureNativeRows(rows, decodeFixtureSchedule), err
}
