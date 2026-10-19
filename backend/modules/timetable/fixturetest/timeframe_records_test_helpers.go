//go:build testhooks

package fixturetest

import (
	context "context"
	time "time"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type TimeframeRecords interface {
	Create(arg0 context.Context, arg1 *Timeframe) error
	Delete(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*Timeframe, error)
	FindByTimeRange(arg0 context.Context, arg1 time.Time, arg2 time.Time) ([]*Timeframe, error)
	ListAll(arg0 context.Context) ([]*Timeframe, error)
	Update(arg0 context.Context, arg1 *Timeframe) error
}

type fromTimeframeRecords struct {
	value ports.TimeframeRecords
}

func FromTimeframeRecords(value ports.TimeframeRecords) TimeframeRecords {
	if original, ok := value.(interface{ fixtureTimeframeRecords() TimeframeRecords }); ok {
		return original.fixtureTimeframeRecords()
	}
	if value == nil {
		return nil
	}
	return fromTimeframeRecords{value: value}
}

func (adapter fromTimeframeRecords) Create(arg0 context.Context, arg1 *Timeframe) error {
	result0 := adapter.value.Create(arg0, toNative43(arg1))
	return result0
}

func (adapter fromTimeframeRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromTimeframeRecords) FindByID(arg0 context.Context, arg1 int64) (*Timeframe, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative43(result0), result1
}

func (adapter fromTimeframeRecords) FindByTimeRange(arg0 context.Context, arg1 time.Time, arg2 time.Time) ([]*Timeframe, error) {
	result0, result1 := adapter.value.FindByTimeRange(arg0, arg1, arg2)
	return fromNative44(result0), result1
}

func (adapter fromTimeframeRecords) ListAll(arg0 context.Context) ([]*Timeframe, error) {
	result0, result1 := adapter.value.ListAll(arg0)
	return fromNative44(result0), result1
}

func (adapter fromTimeframeRecords) Update(arg0 context.Context, arg1 *Timeframe) error {
	result0 := adapter.value.Update(arg0, toNative43(arg1))
	return result0
}

type intoTimeframeRecords struct {
	value TimeframeRecords
}

func IntoTimeframeRecords(value TimeframeRecords) ports.TimeframeRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.TimeframeRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface{ nativeTimeframeRecords() ports.TimeframeRecords }); ok {
		return original.nativeTimeframeRecords()
	}
	if value == nil {
		return nil
	}
	return intoTimeframeRecords{value: value}
}

func (adapter intoTimeframeRecords) Create(arg0 context.Context, arg1 *records.Timeframe) error {
	fixtureArg1 := fromNative43(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.Timeframe
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}

func (adapter intoTimeframeRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter intoTimeframeRecords) FindByID(arg0 context.Context, arg1 int64) (*records.Timeframe, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative43(result0), result1
}

func (adapter intoTimeframeRecords) FindByTimeRange(arg0 context.Context, arg1 time.Time, arg2 time.Time) ([]*records.Timeframe, error) {
	result0, result1 := adapter.value.FindByTimeRange(arg0, arg1, arg2)
	return toNative44(result0), result1
}

func (adapter intoTimeframeRecords) ListAll(arg0 context.Context) ([]*records.Timeframe, error) {
	result0, result1 := adapter.value.ListAll(arg0)
	return toNative44(result0), result1
}

func (adapter intoTimeframeRecords) Update(arg0 context.Context, arg1 *records.Timeframe) error {
	fixtureArg1 := fromNative43(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.Timeframe
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}

func (adapter fromTimeframeRecords) nativeTimeframeRecords() ports.TimeframeRecords {
	return adapter.value
}

func (adapter intoTimeframeRecords) fixtureTimeframeRecords() TimeframeRecords { return adapter.value }

func (adapter fromTimeframeRecords) nativeRecordSource() any { return adapter.value }
