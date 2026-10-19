//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type CalendarPeriodRecords interface {
	Create(arg0 context.Context, arg1 *CalendarPeriod) error
	CreateIfAbsent(arg0 context.Context, arg1 *CalendarPeriod) (bool, error)
	Delete(arg0 context.Context, arg1 int64) error
	FindActiveByTenantID(arg0 context.Context) ([]*CalendarPeriod, error)
	FindActiveOverlapping(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 int64) ([]*CalendarPeriod, error)
	FindActiveOverlappingByType(arg0 context.Context, arg1 string, arg2 calendar.Date, arg3 calendar.Date, arg4 int64) ([]*CalendarPeriod, error)
	FindByID(arg0 context.Context, arg1 int64) (*CalendarPeriod, error)
	FindByName(arg0 context.Context, arg1 string) (*CalendarPeriod, error)
	FindByTenantID(arg0 context.Context) ([]*CalendarPeriod, error)
	Update(arg0 context.Context, arg1 *CalendarPeriod) error
	UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error)
}

type fromCalendarPeriodRecords struct {
	value ports.CalendarPeriodRecords
}

func FromCalendarPeriodRecords(value ports.CalendarPeriodRecords) CalendarPeriodRecords {
	if original, ok := value.(interface{ fixtureCalendarPeriodRecords() CalendarPeriodRecords }); ok {
		return original.fixtureCalendarPeriodRecords()
	}
	if value == nil {
		return nil
	}
	return fromCalendarPeriodRecords{value: value}
}

func (adapter fromCalendarPeriodRecords) Create(arg0 context.Context, arg1 *CalendarPeriod) error {
	result0 := adapter.value.Create(arg0, toNative7(arg1))
	return result0
}

func (adapter fromCalendarPeriodRecords) CreateIfAbsent(arg0 context.Context, arg1 *CalendarPeriod) (bool, error) {
	result0, result1 := adapter.value.CreateIfAbsent(arg0, toNative7(arg1))
	return result0, result1
}

func (adapter fromCalendarPeriodRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromCalendarPeriodRecords) FindActiveByTenantID(arg0 context.Context) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveByTenantID(arg0)
	return fromNative8(result0), result1
}

func (adapter fromCalendarPeriodRecords) FindActiveOverlapping(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 int64) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlapping(arg0, arg1, arg2, arg3)
	return fromNative8(result0), result1
}

func (adapter fromCalendarPeriodRecords) FindActiveOverlappingByType(arg0 context.Context, arg1 string, arg2 calendar.Date, arg3 calendar.Date, arg4 int64) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlappingByType(arg0, arg1, arg2, arg3, arg4)
	return fromNative8(result0), result1
}

func (adapter fromCalendarPeriodRecords) FindByID(arg0 context.Context, arg1 int64) (*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative7(result0), result1
}

func (adapter fromCalendarPeriodRecords) FindByName(arg0 context.Context, arg1 string) (*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByName(arg0, arg1)
	return fromNative7(result0), result1
}

func (adapter fromCalendarPeriodRecords) FindByTenantID(arg0 context.Context) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByTenantID(arg0)
	return fromNative8(result0), result1
}

func (adapter fromCalendarPeriodRecords) Update(arg0 context.Context, arg1 *CalendarPeriod) error {
	result0 := adapter.value.Update(arg0, toNative7(arg1))
	return result0
}

func (adapter fromCalendarPeriodRecords) UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.UsageCounts(arg0)
	return fromNative9(result0), result1
}

type intoCalendarPeriodRecords struct {
	value CalendarPeriodRecords
}

func IntoCalendarPeriodRecords(value CalendarPeriodRecords) ports.CalendarPeriodRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.CalendarPeriodRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeCalendarPeriodRecords() ports.CalendarPeriodRecords
	}); ok {
		return original.nativeCalendarPeriodRecords()
	}
	if value == nil {
		return nil
	}
	return intoCalendarPeriodRecords{value: value}
}

func (adapter intoCalendarPeriodRecords) Create(arg0 context.Context, arg1 *records.CalendarPeriod) error {
	fixtureArg1 := fromNative7(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.CalendarPeriod
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}

func (adapter intoCalendarPeriodRecords) CreateIfAbsent(arg0 context.Context, arg1 *records.CalendarPeriod) (bool, error) {
	fixtureArg1 := fromNative7(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.CalendarPeriod
		}
	}()
	result0, result1 := adapter.value.CreateIfAbsent(arg0, fixtureArg1)
	return result0, result1
}

func (adapter intoCalendarPeriodRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter intoCalendarPeriodRecords) FindActiveByTenantID(arg0 context.Context) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveByTenantID(arg0)
	return toNative8(result0), result1
}

func (adapter intoCalendarPeriodRecords) FindActiveOverlapping(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 int64) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlapping(arg0, arg1, arg2, arg3)
	return toNative8(result0), result1
}

func (adapter intoCalendarPeriodRecords) FindActiveOverlappingByType(arg0 context.Context, arg1 string, arg2 calendar.Date, arg3 calendar.Date, arg4 int64) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlappingByType(arg0, arg1, arg2, arg3, arg4)
	return toNative8(result0), result1
}

func (adapter intoCalendarPeriodRecords) FindByID(arg0 context.Context, arg1 int64) (*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative7(result0), result1
}

func (adapter intoCalendarPeriodRecords) FindByName(arg0 context.Context, arg1 string) (*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByName(arg0, arg1)
	return toNative7(result0), result1
}

func (adapter intoCalendarPeriodRecords) FindByTenantID(arg0 context.Context) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByTenantID(arg0)
	return toNative8(result0), result1
}

func (adapter intoCalendarPeriodRecords) Update(arg0 context.Context, arg1 *records.CalendarPeriod) error {
	fixtureArg1 := fromNative7(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.CalendarPeriod
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}

func (adapter intoCalendarPeriodRecords) UsageCounts(arg0 context.Context) (map[int64]records.CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.UsageCounts(arg0)
	return toNative9(result0), result1
}

func (adapter fromCalendarPeriodRecords) nativeCalendarPeriodRecords() ports.CalendarPeriodRecords {
	return adapter.value
}

func (adapter intoCalendarPeriodRecords) fixtureCalendarPeriodRecords() CalendarPeriodRecords {
	return adapter.value
}

func (adapter fromCalendarPeriodRecords) nativeRecordSource() any { return adapter.value }
