//go:build testhooks

package fixturetest

import (
	context "context"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type InstanceIdempotencyRecords interface {
	CreateIdempotent(arg0 context.Context, arg1 *ActivityInstance) (bool, error)
}

type fromInstanceIdempotencyRecords struct {
	value ports.InstanceIdempotencyRecords
}

func FromInstanceIdempotencyRecords(value ports.InstanceIdempotencyRecords) InstanceIdempotencyRecords {
	if original, ok := value.(interface {
		fixtureInstanceIdempotencyRecords() InstanceIdempotencyRecords
	}); ok {
		return original.fixtureInstanceIdempotencyRecords()
	}
	if value == nil {
		return nil
	}
	return fromInstanceIdempotencyRecords{value: value}
}

func (adapter fromInstanceIdempotencyRecords) CreateIdempotent(arg0 context.Context, arg1 *ActivityInstance) (bool, error) {
	result0, result1 := adapter.value.CreateIdempotent(arg0, toNative2(arg1))
	return result0, result1
}

type intoInstanceIdempotencyRecords struct {
	value InstanceIdempotencyRecords
}

func IntoInstanceIdempotencyRecords(value InstanceIdempotencyRecords) ports.InstanceIdempotencyRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.InstanceIdempotencyRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeInstanceIdempotencyRecords() ports.InstanceIdempotencyRecords
	}); ok {
		return original.nativeInstanceIdempotencyRecords()
	}
	if value == nil {
		return nil
	}
	return intoInstanceIdempotencyRecords{value: value}
}

func (adapter intoInstanceIdempotencyRecords) CreateIdempotent(arg0 context.Context, arg1 *records.ActivityInstance) (bool, error) {
	fixtureArg1 := fromNative2(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityInstance
		}
	}()
	result0, result1 := adapter.value.CreateIdempotent(arg0, fixtureArg1)
	return result0, result1
}

func (adapter fromInstanceIdempotencyRecords) nativeInstanceIdempotencyRecords() ports.InstanceIdempotencyRecords {
	return adapter.value
}

func (adapter intoInstanceIdempotencyRecords) fixtureInstanceIdempotencyRecords() InstanceIdempotencyRecords {
	return adapter.value
}

func (adapter fromInstanceIdempotencyRecords) nativeRecordSource() any { return adapter.value }
