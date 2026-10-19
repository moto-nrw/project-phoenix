//go:build testhooks

package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	models "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type nativeFixtureException struct {
	fixtureRecord[timetable.ActivityException, models.ActivityException]
}

func NativeExceptionRecords(rows ports.ActivityExceptionRecords) ports.NativeExceptionRecords {
	return nativeFixtureException{fixtureRecord: fixtureRecord[timetable.ActivityException, models.ActivityException]{rows: rows, find: fixtureFind(rows.FindByID), encode: encodeFixtureException, decode: decodeFixtureException}}
}
func encodeFixtureException(value *timetable.ActivityException) (*models.ActivityException, error) {
	if value == nil {
		return nil, nil
	}
	return legacyActivityException(*value)
}
func decodeFixtureException(value *models.ActivityException) *timetable.ActivityException {
	if value == nil {
		return nil
	}
	input := publicActivityExceptionInput(value)
	return &timetable.ActivityException{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, ActivityGroupID: input.ActivityGroupID, ExceptionDate: input.ExceptionDate, ExceptionType: input.ExceptionType, StartTime: input.StartTime, EndTime: input.EndTime, RoomID: input.RoomID, Reason: input.Reason, CreatedBy: input.CreatedBy}
}
