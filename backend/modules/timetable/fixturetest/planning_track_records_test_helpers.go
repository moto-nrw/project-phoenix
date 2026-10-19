//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type PlanningTrackRecords interface {
	Create(arg0 context.Context, arg1 *PlanningTrack) error
	Delete(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*PlanningTrack, error)
	FindByIDForShare(arg0 context.Context, arg1 int64) (*PlanningTrack, error)
	FindByIDs(arg0 context.Context, arg1 []int64) ([]*PlanningTrack, error)
	ListAll(arg0 context.Context) ([]*PlanningTrack, error)
	RestoreAtEnd(arg0 context.Context, arg1 *PlanningTrack) (bool, error)
	Update(arg0 context.Context, arg1 *PlanningTrack) error
	UpdateColumns(arg0 context.Context, arg1 *PlanningTrack, arg2 ...string) (int64, error)
	UpdateSortOrders(arg0 context.Context, arg1 []int64) error
}

type fromPlanningTrackRecords struct {
	value ports.PlanningTrackRecords
}

func FromPlanningTrackRecords(value ports.PlanningTrackRecords) PlanningTrackRecords {
	if original, ok := value.(interface{ fixturePlanningTrackRecords() PlanningTrackRecords }); ok {
		return original.fixturePlanningTrackRecords()
	}
	if value == nil {
		return nil
	}
	return fromPlanningTrackRecords{value: value}
}

func (adapter fromPlanningTrackRecords) Create(arg0 context.Context, arg1 *PlanningTrack) error {
	result0 := adapter.value.Create(arg0, toNative34(arg1))
	return result0
}

func (adapter fromPlanningTrackRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromPlanningTrackRecords) FindByID(arg0 context.Context, arg1 int64) (*PlanningTrack, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative34(result0), result1
}

func (adapter fromPlanningTrackRecords) FindByIDForShare(arg0 context.Context, arg1 int64) (*PlanningTrack, error) {
	result0, result1 := adapter.value.FindByIDForShare(arg0, arg1)
	return fromNative34(result0), result1
}

func (adapter fromPlanningTrackRecords) FindByIDs(arg0 context.Context, arg1 []int64) ([]*PlanningTrack, error) {
	result0, result1 := adapter.value.FindByIDs(arg0, arg1)
	return fromNative35(result0), result1
}

func (adapter fromPlanningTrackRecords) ListAll(arg0 context.Context) ([]*PlanningTrack, error) {
	result0, result1 := adapter.value.ListAll(arg0)
	return fromNative35(result0), result1
}

func (adapter fromPlanningTrackRecords) RestoreAtEnd(arg0 context.Context, arg1 *PlanningTrack) (bool, error) {
	result0, result1 := adapter.value.RestoreAtEnd(arg0, toNative34(arg1))
	return result0, result1
}

func (adapter fromPlanningTrackRecords) Update(arg0 context.Context, arg1 *PlanningTrack) error {
	result0 := adapter.value.Update(arg0, toNative34(arg1))
	return result0
}

func (adapter fromPlanningTrackRecords) UpdateColumns(arg0 context.Context, arg1 *PlanningTrack, arg2 ...string) (int64, error) {
	result0, result1 := adapter.value.UpdateColumns(arg0, toNative34(arg1), arg2...)
	return result0, result1
}

func (adapter fromPlanningTrackRecords) UpdateSortOrders(arg0 context.Context, arg1 []int64) error {
	result0 := adapter.value.UpdateSortOrders(arg0, arg1)
	return result0
}

func (adapter fromPlanningTrackRecords) nativeRecordSource() any { return adapter.value }
