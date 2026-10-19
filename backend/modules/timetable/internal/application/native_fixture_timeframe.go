//go:build testhooks

package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	models "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type nativeFixtureTimeframe struct {
	fixtureRecord[timetable.Timeframe, models.Timeframe]
}

func NativeTimeframeRecords(rows ports.TimeframeRecords) ports.NativeTimeframeRecords {
	return nativeFixtureTimeframe{fixtureRecord: fixtureRecord[timetable.Timeframe, models.Timeframe]{rows: rows, find: fixtureFind(rows.FindByID), encode: encodeFixtureTimeframe, decode: decodeFixtureTimeframe}}
}
func encodeFixtureTimeframe(value *timetable.Timeframe) (*models.Timeframe, error) {
	if value == nil {
		return nil, nil
	}
	return legacyTimeframe(*value)
}
func decodeFixtureTimeframe(value *models.Timeframe) *timetable.Timeframe {
	if value == nil {
		return nil
	}
	input := publicTimeframeInput(value)
	return &timetable.Timeframe{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StartTime: input.StartTime, EndTime: input.EndTime, IsActive: input.IsActive, Description: input.Description}
}
