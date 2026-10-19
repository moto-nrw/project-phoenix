//go:build testhooks

package fixturetest

import (
	context "context"
	time "time"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
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
	value ports.ActivityInstanceRecords
}

func FromActivityInstanceRecords(value ports.ActivityInstanceRecords) ActivityInstanceRecords {
	if original, ok := value.(interface {
		fixtureActivityInstanceRecords() ActivityInstanceRecords
	}); ok {
		return original.fixtureActivityInstanceRecords()
	}
	if value == nil {
		return nil
	}
	if listing, ok := value.(ports.InstanceListing); ok {
		return fromListedActivityInstances{fromActivityInstanceRecords{value: value}, listing}
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

type intoActivityInstanceRecords struct {
	value ActivityInstanceRecords
}

func IntoActivityInstanceRecords(value ActivityInstanceRecords) ports.ActivityInstanceRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ActivityInstanceRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeActivityInstanceRecords() ports.ActivityInstanceRecords
	}); ok {
		return original.nativeActivityInstanceRecords()
	}
	if value == nil {
		return nil
	}
	if listing, ok := value.(InstanceListing); ok {
		return intoListedActivityInstances{intoActivityInstanceRecords{value: value}, listing}
	}
	return intoActivityInstanceRecords{value: value}
}

func (adapter intoActivityInstanceRecords) CompleteActiveByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int64, error) {
	result0, result1 := adapter.value.CompleteActiveByActiveGroupIDs(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoActivityInstanceRecords) Create(arg0 context.Context, arg1 *records.ActivityInstance) error {
	fixtureArg1 := fromNative2(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityInstance
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}

func (adapter intoActivityInstanceRecords) CreateTemplateBackedIfAbsent(arg0 context.Context, arg1 *records.ActivityInstance) (bool, error) {
	fixtureArg1 := fromNative2(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityInstance
		}
	}()
	result0, result1 := adapter.value.CreateTemplateBackedIfAbsent(arg0, fixtureArg1)
	return result0, result1
}

func (adapter intoActivityInstanceRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter intoActivityInstanceRecords) DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeleteOlderThan(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoActivityInstanceRecords) DeletePlannedNonSpontaneousInWindow(arg0 context.Context, arg1 calendar.Date, arg2 *calendar.Date, arg3 *int64, arg4 bool) (int64, error) {
	result0, result1 := adapter.value.DeletePlannedNonSpontaneousInWindow(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter intoActivityInstanceRecords) FindByActiveGroupID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActiveGroupID(arg0, arg1)
	return toNative2(result0), result1
}

func (adapter intoActivityInstanceRecords) FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDate(arg0, arg1, arg2)
	return toNative3(result0), result1
}

func (adapter intoActivityInstanceRecords) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return toNative3(result0), result1
}

func (adapter intoActivityInstanceRecords) FindByID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative2(result0), result1
}

func (adapter intoActivityInstanceRecords) FindByIDs(arg0 context.Context, arg1 []int64) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByIDs(arg0, arg1)
	return toNative3(result0), result1
}

func (adapter intoActivityInstanceRecords) FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDate(arg0, arg1)
	return toNative3(result0), result1
}

func (adapter intoActivityInstanceRecords) FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDateRange(arg0, arg1, arg2)
	return toNative3(result0), result1
}

func (adapter intoActivityInstanceRecords) FindPlannedTemplateBackedFrom(arg0 context.Context, arg1 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindPlannedTemplateBackedFrom(arg0, arg1)
	return toNative3(result0), result1
}

func (adapter intoActivityInstanceRecords) MarkCompleted(arg0 context.Context, arg1 int64, arg2 time.Time) error {
	result0 := adapter.value.MarkCompleted(arg0, arg1, arg2)
	return result0
}

func (adapter intoActivityInstanceRecords) MaxID(arg0 context.Context) (int64, error) {
	result0, result1 := adapter.value.MaxID(arg0)
	return result0, result1
}

func (adapter intoActivityInstanceRecords) OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error) {
	result0, result1 := adapter.value.OldestBefore(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoActivityInstanceRecords) PropagateListKindToFutureInstances(arg0 context.Context, arg1 int64, arg2 *string, arg3 *string, arg4 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.PropagateListKindToFutureInstances(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter intoActivityInstanceRecords) Update(arg0 context.Context, arg1 *records.ActivityInstance) error {
	fixtureArg1 := fromNative2(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityInstance
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}

func (adapter intoActivityInstanceRecords) UpdateColumns(arg0 context.Context, arg1 *records.ActivityInstance, arg2 ...string) (int64, error) {
	fixtureArg1 := fromNative2(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityInstance
		}
	}()
	result0, result1 := adapter.value.UpdateColumns(arg0, fixtureArg1, arg2...)
	return result0, result1
}

func (adapter fromActivityInstanceRecords) nativeActivityInstanceRecords() ports.ActivityInstanceRecords {
	return adapter.value
}

func (adapter intoActivityInstanceRecords) fixtureActivityInstanceRecords() ActivityInstanceRecords {
	return adapter.value
}

func (adapter fromActivityInstanceRecords) nativeRecordSource() any { return adapter.value }
