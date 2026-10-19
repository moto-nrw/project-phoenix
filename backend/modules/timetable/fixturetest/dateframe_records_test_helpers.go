//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type DateframeRecords interface {
	Create(arg0 context.Context, arg1 *Dateframe) error
	Delete(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*Dateframe, error)
	FindByName(arg0 context.Context, arg1 string) (*Dateframe, error)
	Update(arg0 context.Context, arg1 *Dateframe) error
}

type fromDateframeRecords struct {
	value ports.DateframeRecords
}

func FromDateframeRecords(value ports.DateframeRecords) DateframeRecords {
	if original, ok := value.(interface{ fixtureDateframeRecords() DateframeRecords }); ok {
		return original.fixtureDateframeRecords()
	}
	if value == nil {
		return nil
	}
	return fromDateframeRecords{value: value}
}

func (adapter fromDateframeRecords) Create(arg0 context.Context, arg1 *Dateframe) error {
	result0 := adapter.value.Create(arg0, toNative29(arg1))
	return result0
}

func (adapter fromDateframeRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromDateframeRecords) FindByID(arg0 context.Context, arg1 int64) (*Dateframe, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative29(result0), result1
}

func (adapter fromDateframeRecords) FindByName(arg0 context.Context, arg1 string) (*Dateframe, error) {
	result0, result1 := adapter.value.FindByName(arg0, arg1)
	return fromNative29(result0), result1
}

func (adapter fromDateframeRecords) Update(arg0 context.Context, arg1 *Dateframe) error {
	result0 := adapter.value.Update(arg0, toNative29(arg1))
	return result0
}

func (adapter fromDateframeRecords) nativeRecordSource() any { return adapter.value }
