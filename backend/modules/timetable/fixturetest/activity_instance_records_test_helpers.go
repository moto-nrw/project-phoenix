//go:build testhooks

package fixturetest

import (
	context "context"
	time "time"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type InstanceFixtureCommands interface {
	CompleteActiveByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int64, error)
	Create(arg0 context.Context, arg1 *ActivityInstance) error
	CreateTemplateBackedIfAbsent(arg0 context.Context, arg1 *ActivityInstance) (bool, error)
	Delete(arg0 context.Context, arg1 int64) error

	DeletePlannedNonSpontaneousInWindow(arg0 context.Context, arg1 calendar.Date, arg2 *calendar.Date, arg3 *int64, arg4 bool) (int64, error)

	MarkCompleted(arg0 context.Context, arg1 int64, arg2 time.Time) error

	PropagateListKindToFutureInstances(arg0 context.Context, arg1 int64, arg2 *string, arg3 *string, arg4 calendar.Date) (int64, error)
	Update(arg0 context.Context, arg1 *ActivityInstance) error
	UpdateColumns(arg0 context.Context, arg1 *ActivityInstance, arg2 ...string) (int64, error)
}

type InstanceFixtureQueries interface {
	FindByActiveGroupID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*ActivityInstance, error)
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityInstance, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByIDs(arg0 context.Context, arg1 []int64) ([]*ActivityInstance, error)
	FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error)
	FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityInstance, error)
	FindPlannedTemplateBackedFrom(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error)
}

type InstanceFixtureRetention interface {
	DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error)

	MaxID(arg0 context.Context) (int64, error)
	OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error)
}

type ActivityInstanceRecords interface {
	InstanceFixtureCommands
	InstanceFixtureQueries
	InstanceFixtureRetention
}

type fromActivityInstanceRecords struct {
	value ports.NativeInstanceRecords
}

func FromActivityInstanceRecords(value ports.NativeInstanceRecords) ActivityInstanceRecords {
	if value == nil {
		return nil
	}
	return fromActivityInstanceRecords{value: value}
}

func (adapter fromActivityInstanceRecords) CompleteActiveByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int64, error) {
	result0, result1 := adapter.value.CompleteActiveByActiveGroupIDs(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) Create(arg0 context.Context, arg1 *ActivityInstance) error {
	result0 := adapter.value.Create(arg0, toNative2(arg1))
	return result0
}

func (adapter fromActivityInstanceRecords) CreateTemplateBackedIfAbsent(arg0 context.Context, arg1 *ActivityInstance) (bool, error) {
	result0, result1 := adapter.value.CreateTemplateBackedIfAbsent(arg0, toNative2(arg1))
	return result0, result1
}

func (adapter fromActivityInstanceRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromActivityInstanceRecords) DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeleteOlderThan(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) DeletePlannedNonSpontaneousInWindow(arg0 context.Context, arg1 calendar.Date, arg2 *calendar.Date, arg3 *int64, arg4 bool) (int64, error) {
	result0, result1 := adapter.value.DeletePlannedNonSpontaneousInWindow(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) FindByActiveGroupID(arg0 context.Context, arg1 int64) (*ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActiveGroupID(arg0, arg1)
	return fromNative2(result0), result1
}

func (adapter fromActivityInstanceRecords) FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDate(arg0, arg1, arg2)
	return fromNative3(result0), result1
}

func (adapter fromActivityInstanceRecords) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative3(result0), result1
}

func (adapter fromActivityInstanceRecords) FindByID(arg0 context.Context, arg1 int64) (*ActivityInstance, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative2(result0), result1
}

func (adapter fromActivityInstanceRecords) FindByIDs(arg0 context.Context, arg1 []int64) ([]*ActivityInstance, error) {
	result0, result1 := adapter.value.FindByIDs(arg0, arg1)
	return fromNative3(result0), result1
}

func (adapter fromActivityInstanceRecords) FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDate(arg0, arg1)
	return fromNative3(result0), result1
}

func (adapter fromActivityInstanceRecords) FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDateRange(arg0, arg1, arg2)
	return fromNative3(result0), result1
}

func (adapter fromActivityInstanceRecords) FindPlannedTemplateBackedFrom(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error) {
	result0, result1 := adapter.value.FindPlannedTemplateBackedFrom(arg0, arg1)
	return fromNative3(result0), result1
}

func (adapter fromActivityInstanceRecords) MarkCompleted(arg0 context.Context, arg1 int64, arg2 time.Time) error {
	result0 := adapter.value.MarkCompleted(arg0, arg1, arg2)
	return result0
}

func (adapter fromActivityInstanceRecords) MaxID(arg0 context.Context) (int64, error) {
	result0, result1 := adapter.value.MaxID(arg0)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error) {
	result0, result1 := adapter.value.OldestBefore(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) PropagateListKindToFutureInstances(arg0 context.Context, arg1 int64, arg2 *string, arg3 *string, arg4 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.PropagateListKindToFutureInstances(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) Update(arg0 context.Context, arg1 *ActivityInstance) error {
	result0 := adapter.value.Update(arg0, toNative2(arg1))
	return result0
}

func (adapter fromActivityInstanceRecords) UpdateColumns(arg0 context.Context, arg1 *ActivityInstance, arg2 ...string) (int64, error) {
	result0, result1 := adapter.value.UpdateColumns(arg0, toNative2(arg1), arg2...)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) nativeRecordSource() any { return adapter.value }
