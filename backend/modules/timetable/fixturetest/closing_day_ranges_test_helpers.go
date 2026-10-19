//go:build testhooks

package fixturetest

import (
	"context"
	"fmt"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type closingDayRangeSource interface {
	FindOverlappingRange(context.Context, calendar.Date, calendar.Date) ([]*records.ClosingDay, error)
}

// ClosingDayRangeFixture exposes the retained inclusive-range behavior only
// to suites. Production consumers use School Calendar's native typed query.
type ClosingDayRangeFixture struct{ source closingDayRangeSource }

func ClosingDayRanges(rows ports.ClosingDayRecords) ClosingDayRangeFixture {
	source, ok := rows.(closingDayRangeSource)
	if !ok {
		panic(fmt.Sprintf("closing day fixture has no range capability: %T", rows))
	}
	return ClosingDayRangeFixture{source: source}
}

func (fixture ClosingDayRangeFixture) FindOverlappingRange(ctx context.Context, from, to calendar.Date) ([]*ClosingDay, error) {
	values, err := fixture.source.FindOverlappingRange(ctx, from, to)
	if values == nil {
		return nil, err
	}
	result := make([]*ClosingDay, len(values))
	for i, value := range values {
		if value != nil {
			result[i] = &ClosingDay{*value}
		}
	}
	return result, err
}
