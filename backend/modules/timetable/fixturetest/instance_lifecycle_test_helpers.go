//go:build testhooks

package fixturetest

import (
	context "context"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type LifecycleFixtureNotices interface {
	AnnounceStaffingChanged(arg0 int64, arg1 string) error

	DeviationLifecycle() ports.DeviationLifecycle
	GetPlannedStudentIDsByDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]int64, error)
	GuardianNoticeReachFor(arg0 context.Context, arg1 int64) (*timetable.GuardianNoticeReach, error)

	QueueActivityUpdates(arg0 context.Context, arg1 timetable.TouchedActivities)

	SeriesDeviations() ports.SeriesDeviations
}

type LifecycleFixturePlanning interface {
	BulkCancelPlanned(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 timetable.BulkCancelOptions, arg4 *int64) (*timetable.BulkCancelResult, error)
	Cancel(arg0 context.Context, arg1 int64, arg2 *string, arg3 *int64) (*timetable.LifecycleInstance, error)
	CancelWithNotice(arg0 context.Context, arg1 timetable.CancelInstanceInput) (*timetable.CancelInstanceResult, error)

	CreateInstance(arg0 context.Context, arg1 timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error)
	DeleteCancelled(arg0 context.Context, arg1 int64) error

	ReplanWeek(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 *int64, arg4 *int64) (*timetable.ReplanWeekResult, error)

	UpdatePlanned(arg0 context.Context, arg1 int64, arg2 timetable.UpdateInstanceInput, arg3 *int64) (*timetable.LifecycleInstance, error)
}

type LifecycleFixtureStaffing interface {
	AcknowledgeUnderstaffed(arg0 context.Context, arg1 int64, arg2 bool, arg3 *string, arg4 *int64) (*timetable.LifecycleInstance, error)

	ClearUnderstaffedAckIfStaffed(arg0 context.Context, arg1 int64, arg2 *int64) error

	MoveStaffBetweenBlocks(arg0 context.Context, arg1 int64, arg2 timetable.MoveStaffInput) (*timetable.MoveStaffResult, error)

	SetUnderstaffedAck(arg0 context.Context, arg1 int64, arg2 bool, arg3 *string, arg4 *int64) (*timetable.LifecycleInstance, error)
}

type LifecycleFixtureTransitions interface {
	Start(arg0 context.Context, arg1 int64, arg2 int64) (*timetable.StartInstanceResult, error)
	Complete(arg0 context.Context, arg1 int64) (*timetable.LifecycleInstance, error)
	CompleteInstanceRow(arg0 context.Context, arg1 int64) (*ActivityInstance, error)

	Reopen(arg0 context.Context, arg1 int64, arg2 int64, arg3 bool) (*timetable.StartInstanceResult, error)
}

type InstanceLifecycle interface {
	LifecycleFixtureNotices
	LifecycleFixturePlanning
	LifecycleFixtureStaffing
	LifecycleFixtureTransitions
}

type fromInstanceLifecycle struct {
	value ports.InstanceLifecycle
}

func FromInstanceLifecycle(value ports.InstanceLifecycle) InstanceLifecycle {
	if original, ok := value.(interface{ fixtureInstanceLifecycle() InstanceLifecycle }); ok {
		return original.fixtureInstanceLifecycle()
	}
	if value == nil {
		return nil
	}
	return fromInstanceLifecycle{value: value}
}

func (adapter fromInstanceLifecycle) AcknowledgeUnderstaffed(arg0 context.Context, arg1 int64, arg2 bool, arg3 *string, arg4 *int64) (*timetable.LifecycleInstance, error) {
	result0, result1 := adapter.value.AcknowledgeUnderstaffed(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromInstanceLifecycle) AnnounceStaffingChanged(arg0 int64, arg1 string) error {
	result0 := adapter.value.AnnounceStaffingChanged(arg0, arg1)
	return result0
}

func (adapter fromInstanceLifecycle) BulkCancelPlanned(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 timetable.BulkCancelOptions, arg4 *int64) (*timetable.BulkCancelResult, error) {
	result0, result1 := adapter.value.BulkCancelPlanned(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromInstanceLifecycle) Cancel(arg0 context.Context, arg1 int64, arg2 *string, arg3 *int64) (*timetable.LifecycleInstance, error) {
	result0, result1 := adapter.value.Cancel(arg0, arg1, arg2, arg3)
	return result0, result1
}

func (adapter fromInstanceLifecycle) CancelWithNotice(arg0 context.Context, arg1 timetable.CancelInstanceInput) (*timetable.CancelInstanceResult, error) {
	result0, result1 := adapter.value.CancelWithNotice(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceLifecycle) ClearUnderstaffedAckIfStaffed(arg0 context.Context, arg1 int64, arg2 *int64) error {
	result0 := adapter.value.ClearUnderstaffedAckIfStaffed(arg0, arg1, arg2)
	return result0
}

func (adapter fromInstanceLifecycle) Complete(arg0 context.Context, arg1 int64) (*timetable.LifecycleInstance, error) {
	result0, result1 := adapter.value.Complete(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceLifecycle) CompleteInstanceRow(arg0 context.Context, arg1 int64) (*ActivityInstance, error) {
	result0, result1 := adapter.value.CompleteInstanceRow(arg0, arg1)
	return fromNative2(result0), result1
}

func (adapter fromInstanceLifecycle) CreateInstance(arg0 context.Context, arg1 timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error) {
	result0, result1 := adapter.value.CreateInstance(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceLifecycle) DeleteCancelled(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteCancelled(arg0, arg1)
	return result0
}

func (adapter fromInstanceLifecycle) DeviationLifecycle() ports.DeviationLifecycle {
	result0 := adapter.value.DeviationLifecycle()
	return result0
}

func (adapter fromInstanceLifecycle) GetPlannedStudentIDsByDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]int64, error) {
	result0, result1 := adapter.value.GetPlannedStudentIDsByDate(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceLifecycle) GuardianNoticeReachFor(arg0 context.Context, arg1 int64) (*timetable.GuardianNoticeReach, error) {
	result0, result1 := adapter.value.GuardianNoticeReachFor(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceLifecycle) MoveStaffBetweenBlocks(arg0 context.Context, arg1 int64, arg2 timetable.MoveStaffInput) (*timetable.MoveStaffResult, error) {
	result0, result1 := adapter.value.MoveStaffBetweenBlocks(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceLifecycle) QueueActivityUpdates(arg0 context.Context, arg1 timetable.TouchedActivities) {
	adapter.value.QueueActivityUpdates(arg0, arg1)
}

func (adapter fromInstanceLifecycle) Reopen(arg0 context.Context, arg1 int64, arg2 int64, arg3 bool) (*timetable.StartInstanceResult, error) {
	result0, result1 := adapter.value.Reopen(arg0, arg1, arg2, arg3)
	return result0, result1
}

func (adapter fromInstanceLifecycle) ReplanWeek(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 *int64, arg4 *int64) (*timetable.ReplanWeekResult, error) {
	result0, result1 := adapter.value.ReplanWeek(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromInstanceLifecycle) SeriesDeviations() ports.SeriesDeviations {
	result0 := adapter.value.SeriesDeviations()
	return result0
}

func (adapter fromInstanceLifecycle) SetUnderstaffedAck(arg0 context.Context, arg1 int64, arg2 bool, arg3 *string, arg4 *int64) (*timetable.LifecycleInstance, error) {
	result0, result1 := adapter.value.SetUnderstaffedAck(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromInstanceLifecycle) Start(arg0 context.Context, arg1 int64, arg2 int64) (*timetable.StartInstanceResult, error) {
	result0, result1 := adapter.value.Start(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceLifecycle) UpdatePlanned(arg0 context.Context, arg1 int64, arg2 timetable.UpdateInstanceInput, arg3 *int64) (*timetable.LifecycleInstance, error) {
	result0, result1 := adapter.value.UpdatePlanned(arg0, arg1, arg2, arg3)
	return result0, result1
}

func (adapter fromInstanceLifecycle) nativeRecordSource() any { return adapter.value }
