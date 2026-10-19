//go:build testhooks

package fixturetest

import (
	context "context"
	time "time"

	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ParticipantFixtureAttendance interface {
	BulkUpdateStatus(arg0 context.Context, arg1 int64, arg2 string, arg3 string, arg4 []int64) (int, error)
	CloseOpenCheckoutsByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int, error)

	ReconcileAttendanceInterval(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time, arg4 *time.Time, arg5 time.Time, arg6 *time.Time) (bool, error)

	UpdateAttendanceCheckout(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) error
	UpdateAttendanceCheckoutBatch(arg0 context.Context, arg1 []InstanceStudentKey, arg2 time.Time) error
	UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 AttendanceFieldPatch) error
	UpdateAttendanceFromCheckin(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (bool, error)
	UpdateAttendanceFromCheckinBatch(arg0 context.Context, arg1 []InstanceStudentKey, arg2 time.Time) error
}

type ParticipantFixtureExcusal interface {
	ApplyActivePartialAbsencesForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error)
	ApplyActiveStatusDaysForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error)
	ApplyPartialAbsence(arg0 context.Context, arg1 int64) (int, error)
	ApplyStatusDay(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 int64, arg4 string) (int, error)

	ReleasePartialAbsence(arg0 context.Context, arg1 int64) (int, error)
	ReleaseStatusDay(arg0 context.Context, arg1 int64) (int, error)
}

type ParticipantFixtureLookup interface {
	FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*InstanceStudent, error)
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStudent, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
	FindByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*InstanceStudent, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*InstanceStudent, error)

	FindInstancesWithAttendanceByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ScheduledInstanceRow, error)

	FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]ParallelPresence, error)
}

type ParticipantFixturePlanning interface {
	CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error)

	FindCurrentCandidates(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 time.Time) ([]*InstanceStudent, error)
	FindCurrentCandidatesByStudentIDs(arg0 context.Context, arg1 []int64, arg2 calendar.Date, arg3 time.Time) ([]*InstanceStudent, error)
	FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)

	FindNotScheduledCandidatesByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
	FindPlannedStudentIDsByDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]int64, error)

	HasPlannedSlotsInRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) (bool, error)
}

type ParticipantFixtureRecords interface {
	Create(arg0 context.Context, arg1 *InstanceStudent) error
	CreateUnplannedPresentIfAbsent(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (*InstanceStudent, error)
	Delete(arg0 context.Context, arg1 int64) error
	DeleteByInstanceID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*InstanceStudent, error)

	Update(arg0 context.Context, arg1 *InstanceStudent) error
}

type ParticipantFixtureRetention interface {
	ArchivePlannedByStudentIDsFrom(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date, arg4 time.Time) (int, error)

	ListStudentInstanceRefsBefore(arg0 context.Context, arg1 calendar.Date) ([]StudentInstanceRef, error)
	MarkExpectedAbsentByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time, arg3 []StudentInstanceRef) error
	MarkNotScheduled(arg0 context.Context, arg1 []StudentInstanceRef) error

	RestoreArchivedByTransition(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date) (int, error)
}

type InstanceStudentRecords interface {
	ParticipantFixtureAttendance
	ParticipantFixtureExcusal
	ParticipantFixtureLookup
	ParticipantFixturePlanning
	ParticipantFixtureRecords
	ParticipantFixtureRetention
}

type fromInstanceStudentRecords struct {
	value ports.InstanceStudentRecords
}

func FromInstanceStudentRecords(value ports.InstanceStudentRecords) InstanceStudentRecords {
	if original, ok := value.(interface{ fixtureInstanceStudentRecords() InstanceStudentRecords }); ok {
		return original.fixtureInstanceStudentRecords()
	}
	if value == nil {
		return nil
	}
	return fromInstanceStudentRecords{value: value}
}

func (adapter fromInstanceStudentRecords) ApplyActivePartialAbsencesForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error) {
	result0, result1 := adapter.value.ApplyActivePartialAbsencesForInstance(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) ApplyActiveStatusDaysForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error) {
	result0, result1 := adapter.value.ApplyActiveStatusDaysForInstance(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) ApplyPartialAbsence(arg0 context.Context, arg1 int64) (int, error) {
	result0, result1 := adapter.value.ApplyPartialAbsence(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) ApplyStatusDay(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 int64, arg4 string) (int, error) {
	result0, result1 := adapter.value.ApplyStatusDay(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) ArchivePlannedByStudentIDsFrom(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date, arg4 time.Time) (int, error) {
	result0, result1 := adapter.value.ArchivePlannedByStudentIDsFrom(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) BulkUpdateStatus(arg0 context.Context, arg1 int64, arg2 string, arg3 string, arg4 []int64) (int, error) {
	result0, result1 := adapter.value.BulkUpdateStatus(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) CloseOpenCheckoutsByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int, error) {
	result0, result1 := adapter.value.CloseOpenCheckoutsByActiveGroupIDs(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error) {
	result0, result1 := adapter.value.CountNonAbsentByInstanceIDs(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) Create(arg0 context.Context, arg1 *InstanceStudent) error {
	result0 := adapter.value.Create(arg0, toNative27(arg1))
	return result0
}

func (adapter fromInstanceStudentRecords) CreateUnplannedPresentIfAbsent(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (*InstanceStudent, error) {
	result0, result1 := adapter.value.CreateUnplannedPresentIfAbsent(arg0, arg1, arg2, arg3)
	return fromNative27(result0), result1
}

func (adapter fromInstanceStudentRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter fromInstanceStudentRecords) DeleteByInstanceID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByInstanceID(arg0, arg1)
	return result0
}

func (adapter fromInstanceStudentRecords) FindByID(arg0 context.Context, arg1 int64) (*InstanceStudent, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative27(result0), result1
}

func (adapter fromInstanceStudentRecords) FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceAndStudent(arg0, arg1, arg2)
	return fromNative27(result0), result1
}

func (adapter fromInstanceStudentRecords) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindByStudentAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindCurrentCandidates(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 time.Time) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindCurrentCandidates(arg0, arg1, arg2, arg3)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindCurrentCandidatesByStudentIDs(arg0 context.Context, arg1 []int64, arg2 calendar.Date, arg3 time.Time) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindCurrentCandidatesByStudentIDs(arg0, arg1, arg2, arg3)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindExpectedByInstanceIDs(arg0, arg1)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindInstancesWithAttendanceByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ScheduledInstanceRow, error) {
	result0, result1 := adapter.value.FindInstancesWithAttendanceByStudentAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative5(result0), result1
}

func (adapter fromInstanceStudentRecords) FindNotScheduledCandidatesByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error) {
	result0, result1 := adapter.value.FindNotScheduledCandidatesByInstanceIDs(arg0, arg1)
	return fromNative26(result0), result1
}

func (adapter fromInstanceStudentRecords) FindPlannedStudentIDsByDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]int64, error) {
	result0, result1 := adapter.value.FindPlannedStudentIDsByDate(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]ParallelPresence, error) {
	result0, result1 := adapter.value.FindPresentInOtherActiveInstances(arg0, arg1, arg2, arg3)
	return fromNative32(result0), result1
}

func (adapter fromInstanceStudentRecords) HasPlannedSlotsInRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) (bool, error) {
	result0, result1 := adapter.value.HasPlannedSlotsInRange(arg0, arg1, arg2)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) ListStudentInstanceRefsBefore(arg0 context.Context, arg1 calendar.Date) ([]StudentInstanceRef, error) {
	result0, result1 := adapter.value.ListStudentInstanceRefsBefore(arg0, arg1)
	return fromNative30(result0), result1
}

func (adapter fromInstanceStudentRecords) MarkExpectedAbsentByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time, arg3 []StudentInstanceRef) error {
	result0 := adapter.value.MarkExpectedAbsentByActiveGroupIDs(arg0, arg1, arg2, toNative30(arg3))
	return result0
}

func (adapter fromInstanceStudentRecords) MarkNotScheduled(arg0 context.Context, arg1 []StudentInstanceRef) error {
	result0 := adapter.value.MarkNotScheduled(arg0, toNative30(arg1))
	return result0
}

func (adapter fromInstanceStudentRecords) ReconcileAttendanceInterval(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time, arg4 *time.Time, arg5 time.Time, arg6 *time.Time) (bool, error) {
	result0, result1 := adapter.value.ReconcileAttendanceInterval(arg0, arg1, arg2, arg3, arg4, arg5, arg6)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) ReleasePartialAbsence(arg0 context.Context, arg1 int64) (int, error) {
	result0, result1 := adapter.value.ReleasePartialAbsence(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) ReleaseStatusDay(arg0 context.Context, arg1 int64) (int, error) {
	result0, result1 := adapter.value.ReleaseStatusDay(arg0, arg1)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) RestoreArchivedByTransition(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date) (int, error) {
	result0, result1 := adapter.value.RestoreArchivedByTransition(arg0, arg1, arg2, arg3)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) Update(arg0 context.Context, arg1 *InstanceStudent) error {
	result0 := adapter.value.Update(arg0, toNative27(arg1))
	return result0
}

func (adapter fromInstanceStudentRecords) UpdateAttendanceCheckout(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) error {
	result0 := adapter.value.UpdateAttendanceCheckout(arg0, arg1, arg2, arg3)
	return result0
}

func (adapter fromInstanceStudentRecords) UpdateAttendanceCheckoutBatch(arg0 context.Context, arg1 []InstanceStudentKey, arg2 time.Time) error {
	result0 := adapter.value.UpdateAttendanceCheckoutBatch(arg0, toNative33(arg1), arg2)
	return result0
}

func (adapter fromInstanceStudentRecords) UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 AttendanceFieldPatch) error {
	result0 := adapter.value.UpdateAttendanceFields(arg0, arg1, toNative28(arg2))
	return result0
}

func (adapter fromInstanceStudentRecords) UpdateAttendanceFromCheckin(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (bool, error) {
	result0, result1 := adapter.value.UpdateAttendanceFromCheckin(arg0, arg1, arg2, arg3)
	return result0, result1
}

func (adapter fromInstanceStudentRecords) UpdateAttendanceFromCheckinBatch(arg0 context.Context, arg1 []InstanceStudentKey, arg2 time.Time) error {
	result0 := adapter.value.UpdateAttendanceFromCheckinBatch(arg0, toNative33(arg1), arg2)
	return result0
}

type intoInstanceStudentRecords struct {
	value InstanceStudentRecords
}

func IntoInstanceStudentRecords(value InstanceStudentRecords) ports.InstanceStudentRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.InstanceStudentRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeInstanceStudentRecords() ports.InstanceStudentRecords
	}); ok {
		return original.nativeInstanceStudentRecords()
	}
	if value == nil {
		return nil
	}
	return intoInstanceStudentRecords{value: value}
}

func (adapter intoInstanceStudentRecords) ApplyActivePartialAbsencesForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error) {
	result0, result1 := adapter.value.ApplyActivePartialAbsencesForInstance(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) ApplyActiveStatusDaysForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error) {
	result0, result1 := adapter.value.ApplyActiveStatusDaysForInstance(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) ApplyPartialAbsence(arg0 context.Context, arg1 int64) (int, error) {
	result0, result1 := adapter.value.ApplyPartialAbsence(arg0, arg1)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) ApplyStatusDay(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 int64, arg4 string) (int, error) {
	result0, result1 := adapter.value.ApplyStatusDay(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) ArchivePlannedByStudentIDsFrom(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date, arg4 time.Time) (int, error) {
	result0, result1 := adapter.value.ArchivePlannedByStudentIDsFrom(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) BulkUpdateStatus(arg0 context.Context, arg1 int64, arg2 string, arg3 string, arg4 []int64) (int, error) {
	result0, result1 := adapter.value.BulkUpdateStatus(arg0, arg1, arg2, arg3, arg4)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) CloseOpenCheckoutsByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int, error) {
	result0, result1 := adapter.value.CloseOpenCheckoutsByActiveGroupIDs(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error) {
	result0, result1 := adapter.value.CountNonAbsentByInstanceIDs(arg0, arg1)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) Create(arg0 context.Context, arg1 *records.InstanceStudent) error {
	fixtureArg1 := fromNative27(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStudent
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}

func (adapter intoInstanceStudentRecords) CreateUnplannedPresentIfAbsent(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (*records.InstanceStudent, error) {
	result0, result1 := adapter.value.CreateUnplannedPresentIfAbsent(arg0, arg1, arg2, arg3)
	return toNative27(result0), result1
}

func (adapter intoInstanceStudentRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}

func (adapter intoInstanceStudentRecords) DeleteByInstanceID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByInstanceID(arg0, arg1)
	return result0
}

func (adapter intoInstanceStudentRecords) FindByID(arg0 context.Context, arg1 int64) (*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative27(result0), result1
}

func (adapter intoInstanceStudentRecords) FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceAndStudent(arg0, arg1, arg2)
	return toNative27(result0), result1
}

func (adapter intoInstanceStudentRecords) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByStudentAndDateRange(arg0, arg1, arg2, arg3)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindCurrentCandidates(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 time.Time) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindCurrentCandidates(arg0, arg1, arg2, arg3)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindCurrentCandidatesByStudentIDs(arg0 context.Context, arg1 []int64, arg2 calendar.Date, arg3 time.Time) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindCurrentCandidatesByStudentIDs(arg0, arg1, arg2, arg3)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindExpectedByInstanceIDs(arg0, arg1)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindInstancesWithAttendanceByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ScheduledInstanceRow, error) {
	result0, result1 := adapter.value.FindInstancesWithAttendanceByStudentAndDateRange(arg0, arg1, arg2, arg3)
	return toNative5(result0), result1
}

func (adapter intoInstanceStudentRecords) FindNotScheduledCandidatesByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindNotScheduledCandidatesByInstanceIDs(arg0, arg1)
	return toNative26(result0), result1
}

func (adapter intoInstanceStudentRecords) FindPlannedStudentIDsByDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]int64, error) {
	result0, result1 := adapter.value.FindPlannedStudentIDsByDate(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]records.ParallelPresence, error) {
	result0, result1 := adapter.value.FindPresentInOtherActiveInstances(arg0, arg1, arg2, arg3)
	return toNative32(result0), result1
}

func (adapter intoInstanceStudentRecords) HasPlannedSlotsInRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) (bool, error) {
	result0, result1 := adapter.value.HasPlannedSlotsInRange(arg0, arg1, arg2)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) ListStudentInstanceRefsBefore(arg0 context.Context, arg1 calendar.Date) ([]records.StudentInstanceRef, error) {
	result0, result1 := adapter.value.ListStudentInstanceRefsBefore(arg0, arg1)
	return toNative30(result0), result1
}

func (adapter intoInstanceStudentRecords) MarkExpectedAbsentByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time, arg3 []records.StudentInstanceRef) error {
	result0 := adapter.value.MarkExpectedAbsentByActiveGroupIDs(arg0, arg1, arg2, fromNative30(arg3))
	return result0
}

func (adapter intoInstanceStudentRecords) MarkNotScheduled(arg0 context.Context, arg1 []records.StudentInstanceRef) error {
	result0 := adapter.value.MarkNotScheduled(arg0, fromNative30(arg1))
	return result0
}

func (adapter intoInstanceStudentRecords) ReconcileAttendanceInterval(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time, arg4 *time.Time, arg5 time.Time, arg6 *time.Time) (bool, error) {
	result0, result1 := adapter.value.ReconcileAttendanceInterval(arg0, arg1, arg2, arg3, arg4, arg5, arg6)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) ReleasePartialAbsence(arg0 context.Context, arg1 int64) (int, error) {
	result0, result1 := adapter.value.ReleasePartialAbsence(arg0, arg1)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) ReleaseStatusDay(arg0 context.Context, arg1 int64) (int, error) {
	result0, result1 := adapter.value.ReleaseStatusDay(arg0, arg1)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) RestoreArchivedByTransition(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date) (int, error) {
	result0, result1 := adapter.value.RestoreArchivedByTransition(arg0, arg1, arg2, arg3)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) Update(arg0 context.Context, arg1 *records.InstanceStudent) error {
	fixtureArg1 := fromNative27(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStudent
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}

func (adapter intoInstanceStudentRecords) UpdateAttendanceCheckout(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) error {
	result0 := adapter.value.UpdateAttendanceCheckout(arg0, arg1, arg2, arg3)
	return result0
}

func (adapter intoInstanceStudentRecords) UpdateAttendanceCheckoutBatch(arg0 context.Context, arg1 []records.InstanceStudentKey, arg2 time.Time) error {
	result0 := adapter.value.UpdateAttendanceCheckoutBatch(arg0, fromNative33(arg1), arg2)
	return result0
}

func (adapter intoInstanceStudentRecords) UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 records.AttendanceFieldPatch) error {
	result0 := adapter.value.UpdateAttendanceFields(arg0, arg1, fromNative28(arg2))
	return result0
}

func (adapter intoInstanceStudentRecords) UpdateAttendanceFromCheckin(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (bool, error) {
	result0, result1 := adapter.value.UpdateAttendanceFromCheckin(arg0, arg1, arg2, arg3)
	return result0, result1
}

func (adapter intoInstanceStudentRecords) UpdateAttendanceFromCheckinBatch(arg0 context.Context, arg1 []records.InstanceStudentKey, arg2 time.Time) error {
	result0 := adapter.value.UpdateAttendanceFromCheckinBatch(arg0, fromNative33(arg1), arg2)
	return result0
}

func (adapter fromInstanceStudentRecords) nativeInstanceStudentRecords() ports.InstanceStudentRecords {
	return adapter.value
}

func (adapter intoInstanceStudentRecords) fixtureInstanceStudentRecords() InstanceStudentRecords {
	return adapter.value
}

func (adapter fromInstanceStudentRecords) nativeRecordSource() any { return adapter.value }
