//go:build testhooks

package fixturetest

import (
	context "context"
	time "time"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type RecurrenceRuleRecords interface {
	Create(arg0 context.Context, arg1 *RecurrenceRule) error
	Delete(arg0 context.Context, arg1 int64) error
	FindByDateRange(arg0 context.Context, arg1 time.Time, arg2 time.Time) ([]*RecurrenceRule, error)
	FindByID(arg0 context.Context, arg1 int64) (*RecurrenceRule, error)
	Update(arg0 context.Context, arg1 *RecurrenceRule) error
}

type fromRecurrenceRuleRecords struct {
	value ports.RecurrenceRuleRecords
}

func FromRecurrenceRuleRecords(value ports.RecurrenceRuleRecords) RecurrenceRuleRecords {
	if original, ok := value.(interface{ fixtureRecurrenceRuleRecords() RecurrenceRuleRecords }); ok {
		return original.fixtureRecurrenceRuleRecords()
	}
	if value == nil {
		return nil
	}
	return fromRecurrenceRuleRecords{value: value}
}

func (adapter fromRecurrenceRuleRecords) Create(arg0 context.Context, arg1 *RecurrenceRule) error {
	result0 := adapter.value.Create(arg0, toNative36(arg1))
	return result0
}

func (adapter fromRecurrenceRuleRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromRecurrenceRuleRecords) FindByDateRange(arg0 context.Context, arg1 time.Time, arg2 time.Time) ([]*RecurrenceRule, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return fromNative37(result0), result1
}

func (adapter fromRecurrenceRuleRecords) FindByID(arg0 context.Context, arg1 int64) (*RecurrenceRule, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative36(result0), result1
}

func (adapter fromRecurrenceRuleRecords) Update(arg0 context.Context, arg1 *RecurrenceRule) error {
	result0 := adapter.value.Update(arg0, toNative36(arg1))
	return result0
}

func (adapter fromRecurrenceRuleRecords) nativeRecordSource() any { return adapter.value }
