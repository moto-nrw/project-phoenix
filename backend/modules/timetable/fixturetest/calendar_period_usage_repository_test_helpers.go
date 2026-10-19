//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type CalendarPeriodUsageRepository interface {
	Usage(arg0 context.Context) (map[int64]ports.CalendarPeriodUsage, error)
	UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error)
}

type fromCalendarPeriodUsageRepository struct {
	value ports.CalendarPeriodUsageRepository
}

func FromCalendarPeriodUsageRepository(value ports.CalendarPeriodUsageRepository) CalendarPeriodUsageRepository {
	if original, ok := value.(interface {
		fixtureCalendarPeriodUsageRepository() CalendarPeriodUsageRepository
	}); ok {
		return original.fixtureCalendarPeriodUsageRepository()
	}
	if value == nil {
		return nil
	}
	return fromCalendarPeriodUsageRepository{value: value}
}

func (adapter fromCalendarPeriodUsageRepository) Usage(arg0 context.Context) (map[int64]ports.CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.Usage(arg0)
	return result0, result1
}

func (adapter fromCalendarPeriodUsageRepository) UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.UsageCounts(arg0)
	return fromNative9(result0), result1
}

func (adapter fromCalendarPeriodUsageRepository) nativeRecordSource() any { return adapter.value }
