//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type InstanceStaffRecords interface {
	CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error)
	Create(arg0 context.Context, arg1 *InstanceStaff) error
	Delete(arg0 context.Context, arg1 int64) error
	DeleteByInstanceID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*InstanceStaff, error)
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error)
	FindByStaffAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*InstanceStaff, error)
	FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*InstanceStaff, error)
	FindByStaffIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*InstanceStaff, error)
	Update(arg0 context.Context, arg1 *InstanceStaff) error
	UpdateColumns(arg0 context.Context, arg1 *InstanceStaff, arg2 ...string) (int64, error)
}

type fromInstanceStaffRecords struct {
	value ports.InstanceStaffRecords
}

func FromInstanceStaffRecords(value ports.InstanceStaffRecords) InstanceStaffRecords {
	if original, ok := value.(interface{ fixtureInstanceStaffRecords() InstanceStaffRecords }); ok {
		return original.fixtureInstanceStaffRecords()
	}
	if value == nil {
		return nil
	}
	return fromInstanceStaffRecords{value: value}
}

func (adapter fromInstanceStaffRecords) CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error) {
	result0, result1 := adapter.value.CountNonAbsentByInstanceIDs(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceStaffRecords) Create(arg0 context.Context, arg1 *InstanceStaff) error {
	result0 := adapter.value.Create(arg0, toNative31(arg1))
	return result0
}

func (adapter fromInstanceStaffRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromInstanceStaffRecords) DeleteByInstanceID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByInstanceID(arg0, arg1)
	return result0
}

func (adapter fromInstanceStaffRecords) FindByID(arg0 context.Context, arg1 int64) (*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative31(result0), result1
}

func (adapter fromInstanceStaffRecords) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return fromNative6(result0), result1
}

func (adapter fromInstanceStaffRecords) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return fromNative6(result0), result1
}

func (adapter fromInstanceStaffRecords) FindByStaffAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDate(arg0, arg1, arg2)
	return fromNative6(result0), result1
}

func (adapter fromInstanceStaffRecords) FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative6(result0), result1
}

func (adapter fromInstanceStaffRecords) FindByStaffIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDate(arg0, arg1, arg2)
	return fromNative6(result0), result1
}

func (adapter fromInstanceStaffRecords) Update(arg0 context.Context, arg1 *InstanceStaff) error {
	result0 := adapter.value.Update(arg0, toNative31(arg1))
	return result0
}

func (adapter fromInstanceStaffRecords) UpdateColumns(arg0 context.Context, arg1 *InstanceStaff, arg2 ...string) (int64, error) {
	result0, result1 := adapter.value.UpdateColumns(arg0, toNative31(arg1), arg2...)
	return result0, result1
}

type intoInstanceStaffRecords struct {
	value InstanceStaffRecords
}

func IntoInstanceStaffRecords(value InstanceStaffRecords) ports.InstanceStaffRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.InstanceStaffRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeInstanceStaffRecords() ports.InstanceStaffRecords
	}); ok {
		return original.nativeInstanceStaffRecords()
	}
	if value == nil {
		return nil
	}
	return intoInstanceStaffRecords{value: value}
}

func (adapter intoInstanceStaffRecords) CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error) {
	result0, result1 := adapter.value.CountNonAbsentByInstanceIDs(arg0, arg1)
	return result0, result1
}

func (adapter intoInstanceStaffRecords) Create(arg0 context.Context, arg1 *records.InstanceStaff) error {
	fixtureArg1 := fromNative31(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStaff
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}

func (adapter intoInstanceStaffRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter intoInstanceStaffRecords) DeleteByInstanceID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByInstanceID(arg0, arg1)
	return result0
}

func (adapter intoInstanceStaffRecords) FindByID(arg0 context.Context, arg1 int64) (*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative31(result0), result1
}

func (adapter intoInstanceStaffRecords) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative6(result0), result1
}

func (adapter intoInstanceStaffRecords) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative6(result0), result1
}

func (adapter intoInstanceStaffRecords) FindByStaffAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDate(arg0, arg1, arg2)
	return toNative6(result0), result1
}

func (adapter intoInstanceStaffRecords) FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDateRange(arg0, arg1, arg2, arg3)
	return toNative6(result0), result1
}

func (adapter intoInstanceStaffRecords) FindByStaffIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDate(arg0, arg1, arg2)
	return toNative6(result0), result1
}

func (adapter intoInstanceStaffRecords) Update(arg0 context.Context, arg1 *records.InstanceStaff) error {
	fixtureArg1 := fromNative31(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStaff
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}

func (adapter intoInstanceStaffRecords) UpdateColumns(arg0 context.Context, arg1 *records.InstanceStaff, arg2 ...string) (int64, error) {
	fixtureArg1 := fromNative31(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStaff
		}
	}()
	result0, result1 := adapter.value.UpdateColumns(arg0, fixtureArg1, arg2...)
	return result0, result1
}

func (adapter fromInstanceStaffRecords) nativeInstanceStaffRecords() ports.InstanceStaffRecords {
	return adapter.value
}

func (adapter intoInstanceStaffRecords) fixtureInstanceStaffRecords() InstanceStaffRecords {
	return adapter.value
}

func (adapter fromInstanceStaffRecords) nativeRecordSource() any { return adapter.value }
