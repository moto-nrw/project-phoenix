//go:build testhooks

package fixturetest

import (
	context "context"
	time "time"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type ActivityRecoveryRecords interface {
	CompletionTimestamp(arg0 context.Context) (time.Time, error)
	LockAttendance(arg0 context.Context, arg1 int64) error
	LockOpenSupervisors(arg0 context.Context, arg1 int64) error
	LockOpenVisits(arg0 context.Context, arg1 int64) error
	LockSupervisors(arg0 context.Context, arg1 []int64) error
	Restore(arg0 context.Context, arg1 int64, arg2 ActivityCompletionSnapshot, arg3 time.Time) error
}

type fromActivityRecoveryRecords struct {
	value ports.ActivityRecoveryRecords
}

func FromActivityRecoveryRecords(value ports.ActivityRecoveryRecords) ActivityRecoveryRecords {
	if original, ok := value.(interface {
		fixtureActivityRecoveryRecords() ActivityRecoveryRecords
	}); ok {
		return original.fixtureActivityRecoveryRecords()
	}
	if value == nil {
		return nil
	}
	return fromActivityRecoveryRecords{value: value}
}

func (adapter fromActivityRecoveryRecords) CompletionTimestamp(arg0 context.Context) (time.Time, error) {
	result0, result1 := adapter.value.CompletionTimestamp(arg0)
	return result0, result1
}

func (adapter fromActivityRecoveryRecords) LockAttendance(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.LockAttendance(arg0, arg1)
	return result0
}

func (adapter fromActivityRecoveryRecords) LockOpenSupervisors(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.LockOpenSupervisors(arg0, arg1)
	return result0
}

func (adapter fromActivityRecoveryRecords) LockOpenVisits(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.LockOpenVisits(arg0, arg1)
	return result0
}

func (adapter fromActivityRecoveryRecords) LockSupervisors(arg0 context.Context, arg1 []int64) error {
	result0 := adapter.value.LockSupervisors(arg0, arg1)
	return result0
}

func (adapter fromActivityRecoveryRecords) Restore(arg0 context.Context, arg1 int64, arg2 ActivityCompletionSnapshot, arg3 time.Time) error {
	result0 := adapter.value.Restore(arg0, arg1, toNative4(arg2), arg3)
	return result0
}

type intoActivityRecoveryRecords struct {
	value ActivityRecoveryRecords
}

func IntoActivityRecoveryRecords(value ActivityRecoveryRecords) ports.ActivityRecoveryRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ActivityRecoveryRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeActivityRecoveryRecords() ports.ActivityRecoveryRecords
	}); ok {
		return original.nativeActivityRecoveryRecords()
	}
	if value == nil {
		return nil
	}
	return intoActivityRecoveryRecords{value: value}
}

func (adapter intoActivityRecoveryRecords) CompletionTimestamp(arg0 context.Context) (time.Time, error) {
	result0, result1 := adapter.value.CompletionTimestamp(arg0)
	return result0, result1
}

func (adapter intoActivityRecoveryRecords) LockAttendance(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.LockAttendance(arg0, arg1)
	return result0
}

func (adapter intoActivityRecoveryRecords) LockOpenSupervisors(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.LockOpenSupervisors(arg0, arg1)
	return result0
}

func (adapter intoActivityRecoveryRecords) LockOpenVisits(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.LockOpenVisits(arg0, arg1)
	return result0
}

func (adapter intoActivityRecoveryRecords) LockSupervisors(arg0 context.Context, arg1 []int64) error {
	result0 := adapter.value.LockSupervisors(arg0, arg1)
	return result0
}

func (adapter intoActivityRecoveryRecords) Restore(arg0 context.Context, arg1 int64, arg2 records.ActivityCompletionSnapshot, arg3 time.Time) error {
	result0 := adapter.value.Restore(arg0, arg1, fromNative4(arg2), arg3)
	return result0
}

func (adapter fromActivityRecoveryRecords) nativeActivityRecoveryRecords() ports.ActivityRecoveryRecords {
	return adapter.value
}

func (adapter intoActivityRecoveryRecords) fixtureActivityRecoveryRecords() ActivityRecoveryRecords {
	return adapter.value
}

func (adapter fromActivityRecoveryRecords) nativeRecordSource() any { return adapter.value }
