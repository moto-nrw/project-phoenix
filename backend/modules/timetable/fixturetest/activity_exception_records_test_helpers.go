//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ActivityExceptionRecords interface {
	Create(arg0 context.Context, arg1 *ActivityException) error
	Delete(arg0 context.Context, arg1 int64) error
	DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error)
	FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*ActivityException, error)
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityException, error)
	FindByActivityGroupID(arg0 context.Context, arg1 int64) ([]*ActivityException, error)
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityException, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityException, error)
	OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error)
	Update(arg0 context.Context, arg1 *ActivityException) error
}

type fromActivityExceptionRecords struct {
	value ports.ActivityExceptionRecords
}

func FromActivityExceptionRecords(value ports.ActivityExceptionRecords) ActivityExceptionRecords {
	if original, ok := value.(interface {
		fixtureActivityExceptionRecords() ActivityExceptionRecords
	}); ok {
		return original.fixtureActivityExceptionRecords()
	}
	if value == nil {
		return nil
	}
	return fromActivityExceptionRecords{value: value}
}

func (adapter fromActivityExceptionRecords) Create(arg0 context.Context, arg1 *ActivityException) error {
	result0 := adapter.value.Create(arg0, toNative0(arg1))
	return result0
}

func (adapter fromActivityExceptionRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromActivityExceptionRecords) DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeleteOlderThan(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromActivityExceptionRecords) FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDate(arg0, arg1, arg2)
	return fromNative0(result0), result1
}

func (adapter fromActivityExceptionRecords) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative1(result0), result1
}

func (adapter fromActivityExceptionRecords) FindByActivityGroupID(arg0 context.Context, arg1 int64) ([]*ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupID(arg0, arg1)
	return fromNative1(result0), result1
}

func (adapter fromActivityExceptionRecords) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityException, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return fromNative1(result0), result1
}

func (adapter fromActivityExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*ActivityException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative0(result0), result1
}

func (adapter fromActivityExceptionRecords) OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error) {
	result0, result1 := adapter.value.OldestBefore(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromActivityExceptionRecords) Update(arg0 context.Context, arg1 *ActivityException) error {
	result0 := adapter.value.Update(arg0, toNative0(arg1))
	return result0
}

type intoActivityExceptionRecords struct {
	value ActivityExceptionRecords
}

func IntoActivityExceptionRecords(value ActivityExceptionRecords) ports.ActivityExceptionRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ActivityExceptionRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeActivityExceptionRecords() ports.ActivityExceptionRecords
	}); ok {
		return original.nativeActivityExceptionRecords()
	}
	if value == nil {
		return nil
	}
	return intoActivityExceptionRecords{value: value}
}

func (adapter intoActivityExceptionRecords) Create(arg0 context.Context, arg1 *records.ActivityException) error {
	fixtureArg1 := fromNative0(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityException
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}

func (adapter intoActivityExceptionRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter intoActivityExceptionRecords) DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeleteOlderThan(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoActivityExceptionRecords) FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDate(arg0, arg1, arg2)
	return toNative0(result0), result1
}

func (adapter intoActivityExceptionRecords) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return toNative1(result0), result1
}

func (adapter intoActivityExceptionRecords) FindByActivityGroupID(arg0 context.Context, arg1 int64) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupID(arg0, arg1)
	return toNative1(result0), result1
}

func (adapter intoActivityExceptionRecords) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return toNative1(result0), result1
}

func (adapter intoActivityExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative0(result0), result1
}

func (adapter intoActivityExceptionRecords) OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error) {
	result0, result1 := adapter.value.OldestBefore(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoActivityExceptionRecords) Update(arg0 context.Context, arg1 *records.ActivityException) error {
	fixtureArg1 := fromNative0(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityException
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}

func (adapter fromActivityExceptionRecords) nativeActivityExceptionRecords() ports.ActivityExceptionRecords {
	return adapter.value
}

func (adapter intoActivityExceptionRecords) fixtureActivityExceptionRecords() ActivityExceptionRecords {
	return adapter.value
}

func (adapter fromActivityExceptionRecords) nativeRecordSource() any { return adapter.value }
