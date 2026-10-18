//go:build testhooks

package application

import (
	models "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type nativeFixtureTimeframe struct {
	fixtureRecord[timetable.Timeframe, models.Timeframe]
}

func NativeTimeframeRecords(rows models.TimeframeRepository) ports.NativeTimeframeRecords {
	return nativeFixtureTimeframe{fixtureRecord: fixtureRecord[timetable.Timeframe, models.Timeframe]{rows: rows, encode: encodeFixtureTimeframe, decode: decodeFixtureTimeframe}}
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
