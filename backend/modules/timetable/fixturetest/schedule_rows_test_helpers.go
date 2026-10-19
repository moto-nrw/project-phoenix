//go:build testhooks

package fixturetest

import (
	context "context"
	time "time"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ActivityCompletionSnapshot is a test fixture over the owner's native row, not a second row schema.
type ActivityCompletionSnapshot struct {
	records.ActivityCompletionSnapshot
}

func NewActivityCompletionSnapshot(configure func(*ActivityCompletionSnapshot)) *ActivityCompletionSnapshot {
	value := new(ActivityCompletionSnapshot)
	if configure != nil {
		configure(value)
	}
	return value
}

// ActivityException is a test fixture over the owner's native row, not a second row schema.
type ActivityException struct{ records.ActivityException }

func NewActivityException(configure func(*ActivityException)) *ActivityException {
	value := new(ActivityException)
	if configure != nil {
		configure(value)
	}
	return value
}
func ActivityExceptionCancelled() string { return records.ActivityExceptionCancelled }
func ActivityExceptionModified() string  { return records.ActivityExceptionModified }

// ActivityInstance is a test fixture over the owner's native row, not a second row schema.
type ActivityInstance struct{ records.ActivityInstance }

func NewActivityInstance(configure func(*ActivityInstance)) *ActivityInstance {
	value := new(ActivityInstance)
	if configure != nil {
		configure(value)
	}
	return value
}

// AttendanceFieldPatch is a test fixture over the owner's native row, not a second row schema.
type AttendanceFieldPatch struct{ records.AttendanceFieldPatch }

func NewAttendanceFieldPatch(configure func(*AttendanceFieldPatch)) *AttendanceFieldPatch {
	value := new(AttendanceFieldPatch)
	if configure != nil {
		configure(value)
	}
	return value
}

// AttendancePatchFieldError is a test fixture over the owner's native row, not a second row schema.

func AttendanceStatusAbsent() string       { return records.AttendanceStatusAbsent }
func AttendanceStatusExpected() string     { return records.AttendanceStatusExpected }
func AttendanceStatusPresent() string      { return records.AttendanceStatusPresent }
func AttendanceSubstatusExcused() string   { return records.AttendanceSubstatusExcused }
func AttendanceSubstatusFieldTrip() string { return records.AttendanceSubstatusFieldTrip }
func AttendanceSubstatusLate() string      { return records.AttendanceSubstatusLate }

func AttendanceSubstatusSick() string { return records.AttendanceSubstatusSick }

// CalendarPeriod is a test fixture over the owner's native row, not a second row schema.
type CalendarPeriod struct{ records.CalendarPeriod }

func NewCalendarPeriod(configure func(*CalendarPeriod)) *CalendarPeriod {
	value := new(CalendarPeriod)
	if configure != nil {
		configure(value)
	}
	return value
}

// CalendarPeriodUsage is a test fixture over the owner's native row, not a second row schema.
type CalendarPeriodUsage struct{ records.CalendarPeriodUsage }

// CareRequestDecisionSnapshot is a test fixture over the owner's native row, not a second row schema.

// CareRequestSnapshotEntry is a test fixture over the owner's native row, not a second row schema.

// CareScheduleChangeRequest is a test fixture over the owner's native row, not a second row schema.

// ClassArrivalException is a test fixture over the owner's native row, not a second row schema.

// ClosingDay is a test fixture over the owner's native row, not a second row schema.
type ClosingDay struct{ records.ClosingDay }

func NewClosingDay(configure func(*ClosingDay)) *ClosingDay {
	value := new(ClosingDay)
	if configure != nil {
		configure(value)
	}
	return value
}

// CompletionAttendanceSnapshot is a test fixture over the owner's native row, not a second row schema.

// Dateframe is a test fixture over the owner's native row, not a second row schema.
type Dateframe struct{ records.Dateframe }

func NewDateframe(configure func(*Dateframe)) *Dateframe {
	value := new(Dateframe)
	if configure != nil {
		configure(value)
	}
	return value
}

func FrequencyDaily() string   { return records.FrequencyDaily }
func FrequencyMonthly() string { return records.FrequencyMonthly }
func FrequencyWeekly() string  { return records.FrequencyWeekly }

// InstanceStaff is a test fixture over the owner's native row, not a second row schema.
type InstanceStaff struct{ records.InstanceStaff }

func NewInstanceStaff(configure func(*InstanceStaff)) *InstanceStaff {
	value := new(InstanceStaff)
	if configure != nil {
		configure(value)
	}
	return value
}
func InstanceStatusActive() string    { return records.InstanceStatusActive }
func InstanceStatusCancelled() string { return records.InstanceStatusCancelled }
func InstanceStatusCompleted() string { return records.InstanceStatusCompleted }
func InstanceStatusPlanned() string   { return records.InstanceStatusPlanned }

// InstanceStudent is a test fixture over the owner's native row, not a second row schema.
type InstanceStudent struct{ records.InstanceStudent }

func NewInstanceStudent(configure func(*InstanceStudent)) *InstanceStudent {
	value := new(InstanceStudent)
	if configure != nil {
		configure(value)
	}
	return value
}

// InstanceStudentKey is a test fixture over the owner's native row, not a second row schema.
type InstanceStudentKey struct{ records.InstanceStudentKey }

func NewInstanceStudentKey(configure func(*InstanceStudentKey)) *InstanceStudentKey {
	value := new(InstanceStudentKey)
	if configure != nil {
		configure(value)
	}
	return value
}
func InstanceStudentNoteMaxLength() int { return records.InstanceStudentNoteMaxLength }

// Model is a test fixture over the owner's native row, not a second row schema.

// ParallelPresence is a test fixture over the owner's native row, not a second row schema.
type ParallelPresence struct{ records.ParallelPresence }

func NewParallelPresence(configure func(*ParallelPresence)) *ParallelPresence {
	value := new(ParallelPresence)
	if configure != nil {
		configure(value)
	}
	return value
}

// PartialAbsenceBlock is a test fixture over the owner's native row, not a second row schema.
type PartialAbsenceBlock struct{ records.PartialAbsenceBlock }

func PeriodTypeCustom() string     { return records.PeriodTypeCustom }
func PeriodTypeHoliday() string    { return records.PeriodTypeHoliday }
func PeriodTypeSchoolYear() string { return records.PeriodTypeSchoolYear }

// PlanningTrack is a test fixture over the owner's native row, not a second row schema.
type PlanningTrack struct{ records.PlanningTrack }

func NewPlanningTrack(configure func(*PlanningTrack)) *PlanningTrack {
	value := new(PlanningTrack)
	if configure != nil {
		configure(value)
	}
	return value
}

// RecurrenceRule is a test fixture over the owner's native row, not a second row schema.
type RecurrenceRule struct{ records.RecurrenceRule }

func NewRecurrenceRule(configure func(*RecurrenceRule)) *RecurrenceRule {
	value := new(RecurrenceRule)
	if configure != nil {
		configure(value)
	}
	return value
}

// ScheduledInstanceRow is a test fixture over the owner's native row, not a second row schema.
type ScheduledInstanceRow struct{ records.ScheduledInstanceRow }

// ShiftType is a test fixture over the owner's native row, not a second row schema.

// StaffNotice is a test fixture over the owner's native row, not a second row schema.

// StaffNoticeAck is a test fixture over the owner's native row, not a second row schema.

// StaffNoticeAcknowledger is a test fixture over the owner's native row, not a second row schema.

// StaffNoticeView is a test fixture over the owner's native row, not a second row schema.

// StaffShift is a test fixture over the owner's native row, not a second row schema.
type StaffShift struct{ records.StaffShift }

func NewStaffShift(configure func(*StaffShift)) *StaffShift {
	value := new(StaffShift)
	if configure != nil {
		configure(value)
	}
	return value
}

// StaffShiftSeries is a test fixture over the owner's native row, not a second row schema.

// StaffShiftSeriesException is a test fixture over the owner's native row, not a second row schema.

// StudentArrivalException is a test fixture over the owner's native row, not a second row schema.
type StudentArrivalException struct {
	records.StudentArrivalException
}

func NewStudentArrivalException(configure func(*StudentArrivalException)) *StudentArrivalException {
	value := new(StudentArrivalException)
	if configure != nil {
		configure(value)
	}
	return value
}

// StudentArrivalNote is a test fixture over the owner's native row, not a second row schema.
type StudentArrivalNote struct{ records.StudentArrivalNote }

func NewStudentArrivalNote(configure func(*StudentArrivalNote)) *StudentArrivalNote {
	value := new(StudentArrivalNote)
	if configure != nil {
		configure(value)
	}
	return value
}

// StudentArrivalSchedule is a test fixture over the owner's native row, not a second row schema.
type StudentArrivalSchedule struct{ records.StudentArrivalSchedule }

func NewStudentArrivalSchedule(configure func(*StudentArrivalSchedule)) *StudentArrivalSchedule {
	value := new(StudentArrivalSchedule)
	if configure != nil {
		configure(value)
	}
	return value
}

// StudentInstanceRef is a test fixture over the owner's native row, not a second row schema.
type StudentInstanceRef struct{ records.StudentInstanceRef }

func NewStudentInstanceRef(configure func(*StudentInstanceRef)) *StudentInstanceRef {
	value := new(StudentInstanceRef)
	if configure != nil {
		configure(value)
	}
	return value
}

// StudentPickupException is a test fixture over the owner's native row, not a second row schema.
type StudentPickupException struct{ records.StudentPickupException }

func NewStudentPickupException(configure func(*StudentPickupException)) *StudentPickupException {
	value := new(StudentPickupException)
	if configure != nil {
		configure(value)
	}
	return value
}

// StudentPickupNote is a test fixture over the owner's native row, not a second row schema.
type StudentPickupNote struct{ records.StudentPickupNote }

func NewStudentPickupNote(configure func(*StudentPickupNote)) *StudentPickupNote {
	value := new(StudentPickupNote)
	if configure != nil {
		configure(value)
	}
	return value
}

// StudentPickupSchedule is a test fixture over the owner's native row, not a second row schema.
type StudentPickupSchedule struct{ records.StudentPickupSchedule }

func NewStudentPickupSchedule(configure func(*StudentPickupSchedule)) *StudentPickupSchedule {
	value := new(StudentPickupSchedule)
	if configure != nil {
		configure(value)
	}
	return value
}

// TenantModel is a test fixture over the owner's native row, not a second row schema.

// Timeframe is a test fixture over the owner's native row, not a second row schema.
type Timeframe struct{ records.Timeframe }

func NewTimeframe(configure func(*Timeframe)) *Timeframe {
	value := new(Timeframe)
	if configure != nil {
		configure(value)
	}
	return value
}

func WeekdayFriday() int { return records.WeekdayFriday }
func WeekdayMonday() int { return records.WeekdayMonday }

func WeekdayThursday() int  { return records.WeekdayThursday }
func WeekdayTuesday() int   { return records.WeekdayTuesday }
func WeekdayWednesday() int { return records.WeekdayWednesday }

type ActivityExceptionRecords interface {
	Create(arg0 context.Context, arg1 *ActivityException) error
	Delete(arg0 context.Context, arg1 int64) error
	DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error)
	FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*ActivityException, error)
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityException, error)
	FindByActivityGroupID(arg0 context.Context, arg1 int64) ([]*ActivityException, error)
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityException, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityException, error)
	OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error)
	Update(arg0 context.Context, arg1 *ActivityException) error
}
type fromActivityExceptionRecords struct {
	value ports.ActivityExceptionRecords
}

func FromActivityExceptionRecords(value ports.ActivityExceptionRecords) ActivityExceptionRecords {
	if original, ok := value.(interface {
		fixtureActivityExceptionRecords() ActivityExceptionRecords
	}); ok {
		return original.fixtureActivityExceptionRecords()
	}
	if value == nil {
		return nil
	}
	return fromActivityExceptionRecords{value: value}
}
func (adapter fromActivityExceptionRecords) Create(arg0 context.Context, arg1 *ActivityException) error {
	result0 := adapter.value.Create(arg0, toNative0(arg1))
	return result0
}
func (adapter fromActivityExceptionRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}
func (adapter fromActivityExceptionRecords) DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeleteOlderThan(arg0, arg1, arg2)
	return result0, result1
}
func (adapter fromActivityExceptionRecords) FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDate(arg0, arg1, arg2)
	return fromNative0(result0), result1
}
func (adapter fromActivityExceptionRecords) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative1(result0), result1
}
func (adapter fromActivityExceptionRecords) FindByActivityGroupID(arg0 context.Context, arg1 int64) ([]*ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupID(arg0, arg1)
	return fromNative1(result0), result1
}
func (adapter fromActivityExceptionRecords) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityException, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return fromNative1(result0), result1
}
func (adapter fromActivityExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*ActivityException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative0(result0), result1
}
func (adapter fromActivityExceptionRecords) OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error) {
	result0, result1 := adapter.value.OldestBefore(arg0, arg1, arg2)
	return result0, result1
}
func (adapter fromActivityExceptionRecords) Update(arg0 context.Context, arg1 *ActivityException) error {
	result0 := adapter.value.Update(arg0, toNative0(arg1))
	return result0
}

type intoActivityExceptionRecords struct {
	value ActivityExceptionRecords
}

func IntoActivityExceptionRecords(value ActivityExceptionRecords) ports.ActivityExceptionRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ActivityExceptionRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeActivityExceptionRecords() ports.ActivityExceptionRecords
	}); ok {
		return original.nativeActivityExceptionRecords()
	}
	if value == nil {
		return nil
	}
	return intoActivityExceptionRecords{value: value}
}
func (adapter intoActivityExceptionRecords) Create(arg0 context.Context, arg1 *records.ActivityException) error {
	fixtureArg1 := fromNative0(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityException
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}
func (adapter intoActivityExceptionRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}
func (adapter intoActivityExceptionRecords) DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeleteOlderThan(arg0, arg1, arg2)
	return result0, result1
}
func (adapter intoActivityExceptionRecords) FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDate(arg0, arg1, arg2)
	return toNative0(result0), result1
}
func (adapter intoActivityExceptionRecords) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return toNative1(result0), result1
}
func (adapter intoActivityExceptionRecords) FindByActivityGroupID(arg0 context.Context, arg1 int64) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupID(arg0, arg1)
	return toNative1(result0), result1
}
func (adapter intoActivityExceptionRecords) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return toNative1(result0), result1
}
func (adapter intoActivityExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative0(result0), result1
}
func (adapter intoActivityExceptionRecords) OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error) {
	result0, result1 := adapter.value.OldestBefore(arg0, arg1, arg2)
	return result0, result1
}
func (adapter intoActivityExceptionRecords) Update(arg0 context.Context, arg1 *records.ActivityException) error {
	fixtureArg1 := fromNative0(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.ActivityException
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}

type ActivityInstanceRecords interface {
	CompleteActiveByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int64, error)
	Create(arg0 context.Context, arg1 *ActivityInstance) error
	CreateTemplateBackedIfAbsent(arg0 context.Context, arg1 *ActivityInstance) (bool, error)
	Delete(arg0 context.Context, arg1 int64) error
	DeleteOlderThan(arg0 context.Context, arg1 string, arg2 calendar.Date) (int64, error)
	DeletePlannedNonSpontaneousInWindow(arg0 context.Context, arg1 calendar.Date, arg2 *calendar.Date, arg3 *int64, arg4 bool) (int64, error)
	FindByActiveGroupID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByActivityGroupAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*ActivityInstance, error)
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityInstance, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByIDs(arg0 context.Context, arg1 []int64) ([]*ActivityInstance, error)
	FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error)
	FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityInstance, error)
	FindPlannedTemplateBackedFrom(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error)
	MarkCompleted(arg0 context.Context, arg1 int64, arg2 time.Time) error
	MaxID(arg0 context.Context) (int64, error)
	OldestBefore(arg0 context.Context, arg1 string, arg2 *calendar.Date) (*calendar.Date, error)
	PropagateListKindToFutureInstances(arg0 context.Context, arg1 int64, arg2 *string, arg3 *string, arg4 calendar.Date) (int64, error)
	Update(arg0 context.Context, arg1 *ActivityInstance) error
	UpdateColumns(arg0 context.Context, arg1 *ActivityInstance, arg2 ...string) (int64, error)
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

type CalendarPeriodRecords interface {
	Create(arg0 context.Context, arg1 *CalendarPeriod) error
	CreateIfAbsent(arg0 context.Context, arg1 *CalendarPeriod) (bool, error)
	Delete(arg0 context.Context, arg1 int64) error
	FindActiveByTenantID(arg0 context.Context) ([]*CalendarPeriod, error)
	FindActiveOverlapping(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 int64) ([]*CalendarPeriod, error)
	FindActiveOverlappingByType(arg0 context.Context, arg1 string, arg2 calendar.Date, arg3 calendar.Date, arg4 int64) ([]*CalendarPeriod, error)
	FindByID(arg0 context.Context, arg1 int64) (*CalendarPeriod, error)
	FindByName(arg0 context.Context, arg1 string) (*CalendarPeriod, error)
	FindByTenantID(arg0 context.Context) ([]*CalendarPeriod, error)
	Update(arg0 context.Context, arg1 *CalendarPeriod) error
	UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error)
}
type fromCalendarPeriodRecords struct {
	value ports.CalendarPeriodRecords
}

func FromCalendarPeriodRecords(value ports.CalendarPeriodRecords) CalendarPeriodRecords {
	if original, ok := value.(interface{ fixtureCalendarPeriodRecords() CalendarPeriodRecords }); ok {
		return original.fixtureCalendarPeriodRecords()
	}
	if value == nil {
		return nil
	}
	return fromCalendarPeriodRecords{value: value}
}
func (adapter fromCalendarPeriodRecords) Create(arg0 context.Context, arg1 *CalendarPeriod) error {
	result0 := adapter.value.Create(arg0, toNative7(arg1))
	return result0
}
func (adapter fromCalendarPeriodRecords) CreateIfAbsent(arg0 context.Context, arg1 *CalendarPeriod) (bool, error) {
	result0, result1 := adapter.value.CreateIfAbsent(arg0, toNative7(arg1))
	return result0, result1
}
func (adapter fromCalendarPeriodRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}
func (adapter fromCalendarPeriodRecords) FindActiveByTenantID(arg0 context.Context) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveByTenantID(arg0)
	return fromNative8(result0), result1
}
func (adapter fromCalendarPeriodRecords) FindActiveOverlapping(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 int64) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlapping(arg0, arg1, arg2, arg3)
	return fromNative8(result0), result1
}
func (adapter fromCalendarPeriodRecords) FindActiveOverlappingByType(arg0 context.Context, arg1 string, arg2 calendar.Date, arg3 calendar.Date, arg4 int64) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlappingByType(arg0, arg1, arg2, arg3, arg4)
	return fromNative8(result0), result1
}
func (adapter fromCalendarPeriodRecords) FindByID(arg0 context.Context, arg1 int64) (*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative7(result0), result1
}
func (adapter fromCalendarPeriodRecords) FindByName(arg0 context.Context, arg1 string) (*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByName(arg0, arg1)
	return fromNative7(result0), result1
}
func (adapter fromCalendarPeriodRecords) FindByTenantID(arg0 context.Context) ([]*CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByTenantID(arg0)
	return fromNative8(result0), result1
}
func (adapter fromCalendarPeriodRecords) Update(arg0 context.Context, arg1 *CalendarPeriod) error {
	result0 := adapter.value.Update(arg0, toNative7(arg1))
	return result0
}
func (adapter fromCalendarPeriodRecords) UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.UsageCounts(arg0)
	return fromNative9(result0), result1
}

type intoCalendarPeriodRecords struct {
	value CalendarPeriodRecords
}

func IntoCalendarPeriodRecords(value CalendarPeriodRecords) ports.CalendarPeriodRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.CalendarPeriodRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeCalendarPeriodRecords() ports.CalendarPeriodRecords
	}); ok {
		return original.nativeCalendarPeriodRecords()
	}
	if value == nil {
		return nil
	}
	return intoCalendarPeriodRecords{value: value}
}
func (adapter intoCalendarPeriodRecords) Create(arg0 context.Context, arg1 *records.CalendarPeriod) error {
	fixtureArg1 := fromNative7(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.CalendarPeriod
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}
func (adapter intoCalendarPeriodRecords) CreateIfAbsent(arg0 context.Context, arg1 *records.CalendarPeriod) (bool, error) {
	fixtureArg1 := fromNative7(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.CalendarPeriod
		}
	}()
	result0, result1 := adapter.value.CreateIfAbsent(arg0, fixtureArg1)
	return result0, result1
}
func (adapter intoCalendarPeriodRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}
func (adapter intoCalendarPeriodRecords) FindActiveByTenantID(arg0 context.Context) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveByTenantID(arg0)
	return toNative8(result0), result1
}
func (adapter intoCalendarPeriodRecords) FindActiveOverlapping(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 int64) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlapping(arg0, arg1, arg2, arg3)
	return toNative8(result0), result1
}
func (adapter intoCalendarPeriodRecords) FindActiveOverlappingByType(arg0 context.Context, arg1 string, arg2 calendar.Date, arg3 calendar.Date, arg4 int64) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindActiveOverlappingByType(arg0, arg1, arg2, arg3, arg4)
	return toNative8(result0), result1
}
func (adapter intoCalendarPeriodRecords) FindByID(arg0 context.Context, arg1 int64) (*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative7(result0), result1
}
func (adapter intoCalendarPeriodRecords) FindByName(arg0 context.Context, arg1 string) (*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByName(arg0, arg1)
	return toNative7(result0), result1
}
func (adapter intoCalendarPeriodRecords) FindByTenantID(arg0 context.Context) ([]*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByTenantID(arg0)
	return toNative8(result0), result1
}
func (adapter intoCalendarPeriodRecords) Update(arg0 context.Context, arg1 *records.CalendarPeriod) error {
	fixtureArg1 := fromNative7(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.CalendarPeriod
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}
func (adapter intoCalendarPeriodRecords) UsageCounts(arg0 context.Context) (map[int64]records.CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.UsageCounts(arg0)
	return toNative9(result0), result1
}

type CalendarPeriodUsageRepository interface {
	Usage(arg0 context.Context) (map[int64]ports.CalendarPeriodUsage, error)
	UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error)
}
type fromCalendarPeriodUsageRepository struct {
	value ports.CalendarPeriodUsageRepository
}

func FromCalendarPeriodUsageRepository(value ports.CalendarPeriodUsageRepository) CalendarPeriodUsageRepository {
	if original, ok := value.(interface {
		fixtureCalendarPeriodUsageRepository() CalendarPeriodUsageRepository
	}); ok {
		return original.fixtureCalendarPeriodUsageRepository()
	}
	if value == nil {
		return nil
	}
	return fromCalendarPeriodUsageRepository{value: value}
}
func (adapter fromCalendarPeriodUsageRepository) Usage(arg0 context.Context) (map[int64]ports.CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.Usage(arg0)
	return result0, result1
}
func (adapter fromCalendarPeriodUsageRepository) UsageCounts(arg0 context.Context) (map[int64]CalendarPeriodUsage, error) {
	result0, result1 := adapter.value.UsageCounts(arg0)
	return fromNative9(result0), result1
}

type ConflictArrivalExceptions interface {
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalException, error)
}

type intoConflictArrivalExceptions struct {
	value ConflictArrivalExceptions
}

func IntoConflictArrivalExceptions(value ConflictArrivalExceptions) ports.ConflictArrivalExceptions {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictArrivalExceptions); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictArrivalExceptions() ports.ConflictArrivalExceptions
	}); ok {
		return original.nativeConflictArrivalExceptions()
	}
	if value == nil {
		return nil
	}
	return intoConflictArrivalExceptions{value: value}
}
func (adapter intoConflictArrivalExceptions) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*records.StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return toNative13(result0), result1
}

type ConflictCalendarPeriods interface {
	FindByID(arg0 context.Context, arg1 int64) (*CalendarPeriod, error)
}

type intoConflictCalendarPeriods struct {
	value ConflictCalendarPeriods
}

func IntoConflictCalendarPeriods(value ConflictCalendarPeriods) ports.ConflictCalendarPeriods {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictCalendarPeriods); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictCalendarPeriods() ports.ConflictCalendarPeriods
	}); ok {
		return original.nativeConflictCalendarPeriods()
	}
	if value == nil {
		return nil
	}
	return intoConflictCalendarPeriods{value: value}
}
func (adapter intoConflictCalendarPeriods) FindByID(arg0 context.Context, arg1 int64) (*records.CalendarPeriod, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative7(result0), result1
}

type ConflictExceptions interface {
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityException, error)
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityException, error)
}

type intoConflictExceptions struct {
	value ConflictExceptions
}

func IntoConflictExceptions(value ConflictExceptions) ports.ConflictExceptions {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictExceptions); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictExceptions() ports.ConflictExceptions
	}); ok {
		return original.nativeConflictExceptions()
	}
	if value == nil {
		return nil
	}
	return intoConflictExceptions{value: value}
}
func (adapter intoConflictExceptions) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return toNative1(result0), result1
}
func (adapter intoConflictExceptions) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.ActivityException, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return toNative1(result0), result1
}

type ConflictInstanceStaff interface {
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error)
}

type intoConflictInstanceStaff struct {
	value ConflictInstanceStaff
}

func IntoConflictInstanceStaff(value ConflictInstanceStaff) ports.ConflictInstanceStaff {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictInstanceStaff); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictInstanceStaff() ports.ConflictInstanceStaff
	}); ok {
		return original.nativeConflictInstanceStaff()
	}
	if value == nil {
		return nil
	}
	return intoConflictInstanceStaff{value: value}
}
func (adapter intoConflictInstanceStaff) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative6(result0), result1
}
func (adapter intoConflictInstanceStaff) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative6(result0), result1
}

type ConflictInstanceStudents interface {
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStudent, error)
	FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
}

type intoConflictInstanceStudents struct {
	value ConflictInstanceStudents
}

func IntoConflictInstanceStudents(value ConflictInstanceStudents) ports.ConflictInstanceStudents {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictInstanceStudents); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictInstanceStudents() ports.ConflictInstanceStudents
	}); ok {
		return original.nativeConflictInstanceStudents()
	}
	if value == nil {
		return nil
	}
	return intoConflictInstanceStudents{value: value}
}
func (adapter intoConflictInstanceStudents) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative26(result0), result1
}
func (adapter intoConflictInstanceStudents) FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindExpectedByInstanceIDs(arg0, arg1)
	return toNative26(result0), result1
}

type ConflictInstances interface {
	FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ActivityInstance, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*ActivityInstance, error)
}

type intoConflictInstances struct {
	value ConflictInstances
}

func IntoConflictInstances(value ConflictInstances) ports.ConflictInstances {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictInstances); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeConflictInstances() ports.ConflictInstances
	}); ok {
		return original.nativeConflictInstances()
	}
	if value == nil {
		return nil
	}
	return intoConflictInstances{value: value}
}
func (adapter intoConflictInstances) FindByActivityGroupAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActivityGroupAndDateRange(arg0, arg1, arg2, arg3)
	return toNative3(result0), result1
}
func (adapter intoConflictInstances) FindByID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative2(result0), result1
}
func (adapter intoConflictInstances) FindByTenantAndDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDateRange(arg0, arg1, arg2)
	return toNative3(result0), result1
}

type ConflictShifts interface {
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*StaffShift, error)
	FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*StaffShift, error)
	FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error)
}

type intoConflictShifts struct {
	value ConflictShifts
}

func IntoConflictShifts(value ConflictShifts) ports.ConflictShifts {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.ConflictShifts); ok {
			return capability
		}
	}
	if original, ok := value.(interface{ nativeConflictShifts() ports.ConflictShifts }); ok {
		return original.nativeConflictShifts()
	}
	if value == nil {
		return nil
	}
	return intoConflictShifts{value: value}
}
func (adapter intoConflictShifts) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*records.StaffShift, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return toNative10(result0), result1
}
func (adapter intoConflictShifts) FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*records.StaffShift, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDates(arg0, arg1, arg2)
	return toNative10(result0), result1
}
func (adapter intoConflictShifts) FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error) {
	result0, result1 := adapter.value.FindUsedCalendarWeeks(arg0, arg1, arg2)
	return result0, result1
}

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

type InstanceLifecycle interface {
	AcknowledgeUnderstaffed(arg0 context.Context, arg1 int64, arg2 bool, arg3 *string, arg4 *int64) (*timetable.LifecycleInstance, error)
	AnnounceStaffingChanged(arg0 int64, arg1 string) error
	BulkCancelPlanned(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 timetable.BulkCancelOptions, arg4 *int64) (*timetable.BulkCancelResult, error)
	Cancel(arg0 context.Context, arg1 int64, arg2 *string, arg3 *int64) (*timetable.LifecycleInstance, error)
	CancelWithNotice(arg0 context.Context, arg1 timetable.CancelInstanceInput) (*timetable.CancelInstanceResult, error)
	ClearUnderstaffedAckIfStaffed(arg0 context.Context, arg1 int64, arg2 *int64) error
	Complete(arg0 context.Context, arg1 int64) (*timetable.LifecycleInstance, error)
	CompleteInstanceRow(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	CreateInstance(arg0 context.Context, arg1 timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error)
	DeleteCancelled(arg0 context.Context, arg1 int64) error
	DeviationLifecycle() ports.DeviationLifecycle
	GetPlannedStudentIDsByDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]int64, error)
	GuardianNoticeReachFor(arg0 context.Context, arg1 int64) (*timetable.GuardianNoticeReach, error)
	MoveStaffBetweenBlocks(arg0 context.Context, arg1 int64, arg2 timetable.MoveStaffInput) (*timetable.MoveStaffResult, error)
	QueueActivityUpdates(arg0 context.Context, arg1 timetable.TouchedActivities)
	Reopen(arg0 context.Context, arg1 int64, arg2 int64, arg3 bool) (*timetable.StartInstanceResult, error)
	ReplanWeek(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date, arg3 *int64, arg4 *int64) (*timetable.ReplanWeekResult, error)
	SeriesDeviations() ports.SeriesDeviations
	SetUnderstaffedAck(arg0 context.Context, arg1 int64, arg2 bool, arg3 *string, arg4 *int64) (*timetable.LifecycleInstance, error)
	Start(arg0 context.Context, arg1 int64, arg2 int64) (*timetable.StartInstanceResult, error)
	UpdatePlanned(arg0 context.Context, arg1 int64, arg2 timetable.UpdateInstanceInput, arg3 *int64) (*timetable.LifecycleInstance, error)
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

type InstanceListing interface {
	ListInstances(arg0 context.Context, arg1 ports.LegacyInstanceFilter) ([]*ActivityInstance, error)
}

type InstanceStaffRecords interface {
	CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error)
	Create(arg0 context.Context, arg1 *InstanceStaff) error
	Delete(arg0 context.Context, arg1 int64) error
	DeleteByInstanceID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*InstanceStaff, error)
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error)
	FindByStaffAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*InstanceStaff, error)
	FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*InstanceStaff, error)
	FindByStaffIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*InstanceStaff, error)
	Update(arg0 context.Context, arg1 *InstanceStaff) error
	UpdateColumns(arg0 context.Context, arg1 *InstanceStaff, arg2 ...string) (int64, error)
}
type fromInstanceStaffRecords struct {
	value ports.InstanceStaffRecords
}

func FromInstanceStaffRecords(value ports.InstanceStaffRecords) InstanceStaffRecords {
	if original, ok := value.(interface{ fixtureInstanceStaffRecords() InstanceStaffRecords }); ok {
		return original.fixtureInstanceStaffRecords()
	}
	if value == nil {
		return nil
	}
	return fromInstanceStaffRecords{value: value}
}
func (adapter fromInstanceStaffRecords) CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error) {
	result0, result1 := adapter.value.CountNonAbsentByInstanceIDs(arg0, arg1)
	return result0, result1
}
func (adapter fromInstanceStaffRecords) Create(arg0 context.Context, arg1 *InstanceStaff) error {
	result0 := adapter.value.Create(arg0, toNative31(arg1))
	return result0
}
func (adapter fromInstanceStaffRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}
func (adapter fromInstanceStaffRecords) DeleteByInstanceID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByInstanceID(arg0, arg1)
	return result0
}
func (adapter fromInstanceStaffRecords) FindByID(arg0 context.Context, arg1 int64) (*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative31(result0), result1
}
func (adapter fromInstanceStaffRecords) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return fromNative6(result0), result1
}
func (adapter fromInstanceStaffRecords) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return fromNative6(result0), result1
}
func (adapter fromInstanceStaffRecords) FindByStaffAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDate(arg0, arg1, arg2)
	return fromNative6(result0), result1
}
func (adapter fromInstanceStaffRecords) FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative6(result0), result1
}
func (adapter fromInstanceStaffRecords) FindByStaffIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDate(arg0, arg1, arg2)
	return fromNative6(result0), result1
}
func (adapter fromInstanceStaffRecords) Update(arg0 context.Context, arg1 *InstanceStaff) error {
	result0 := adapter.value.Update(arg0, toNative31(arg1))
	return result0
}
func (adapter fromInstanceStaffRecords) UpdateColumns(arg0 context.Context, arg1 *InstanceStaff, arg2 ...string) (int64, error) {
	result0, result1 := adapter.value.UpdateColumns(arg0, toNative31(arg1), arg2...)
	return result0, result1
}

type intoInstanceStaffRecords struct {
	value InstanceStaffRecords
}

func IntoInstanceStaffRecords(value InstanceStaffRecords) ports.InstanceStaffRecords {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.InstanceStaffRecords); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeInstanceStaffRecords() ports.InstanceStaffRecords
	}); ok {
		return original.nativeInstanceStaffRecords()
	}
	if value == nil {
		return nil
	}
	return intoInstanceStaffRecords{value: value}
}
func (adapter intoInstanceStaffRecords) CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error) {
	result0, result1 := adapter.value.CountNonAbsentByInstanceIDs(arg0, arg1)
	return result0, result1
}
func (adapter intoInstanceStaffRecords) Create(arg0 context.Context, arg1 *records.InstanceStaff) error {
	fixtureArg1 := fromNative31(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStaff
		}
	}()
	result0 := adapter.value.Create(arg0, fixtureArg1)
	return result0
}
func (adapter intoInstanceStaffRecords) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}
func (adapter intoInstanceStaffRecords) DeleteByInstanceID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByInstanceID(arg0, arg1)
	return result0
}
func (adapter intoInstanceStaffRecords) FindByID(arg0 context.Context, arg1 int64) (*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative31(result0), result1
}
func (adapter intoInstanceStaffRecords) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative6(result0), result1
}
func (adapter intoInstanceStaffRecords) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative6(result0), result1
}
func (adapter intoInstanceStaffRecords) FindByStaffAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDate(arg0, arg1, arg2)
	return toNative6(result0), result1
}
func (adapter intoInstanceStaffRecords) FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffAndDateRange(arg0, arg1, arg2, arg3)
	return toNative6(result0), result1
}
func (adapter intoInstanceStaffRecords) FindByStaffIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDate(arg0, arg1, arg2)
	return toNative6(result0), result1
}
func (adapter intoInstanceStaffRecords) Update(arg0 context.Context, arg1 *records.InstanceStaff) error {
	fixtureArg1 := fromNative31(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStaff
		}
	}()
	result0 := adapter.value.Update(arg0, fixtureArg1)
	return result0
}
func (adapter intoInstanceStaffRecords) UpdateColumns(arg0 context.Context, arg1 *records.InstanceStaff, arg2 ...string) (int64, error) {
	fixtureArg1 := fromNative31(arg1)
	defer func() {
		if arg1 != nil {
			*arg1 = fixtureArg1.InstanceStaff
		}
	}()
	result0, result1 := adapter.value.UpdateColumns(arg0, fixtureArg1, arg2...)
	return result0, result1
}

type InstanceStudentRecords interface {
	ApplyActivePartialAbsencesForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error)
	ApplyActiveStatusDaysForInstance(arg0 context.Context, arg1 int64, arg2 calendar.Date) (int, error)
	ApplyPartialAbsence(arg0 context.Context, arg1 int64) (int, error)
	ApplyStatusDay(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 int64, arg4 string) (int, error)
	ArchivePlannedByStudentIDsFrom(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date, arg4 time.Time) (int, error)
	BulkUpdateStatus(arg0 context.Context, arg1 int64, arg2 string, arg3 string, arg4 []int64) (int, error)
	CloseOpenCheckoutsByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time) (int, error)
	CountNonAbsentByInstanceIDs(arg0 context.Context, arg1 []int64) (map[int64]int, error)
	Create(arg0 context.Context, arg1 *InstanceStudent) error
	CreateUnplannedPresentIfAbsent(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (*InstanceStudent, error)
	Delete(arg0 context.Context, arg1 int64) error
	DeleteByInstanceID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*InstanceStudent, error)
	FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*InstanceStudent, error)
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStudent, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
	FindByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*InstanceStudent, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*InstanceStudent, error)
	FindCurrentCandidates(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 time.Time) ([]*InstanceStudent, error)
	FindCurrentCandidatesByStudentIDs(arg0 context.Context, arg1 []int64, arg2 calendar.Date, arg3 time.Time) ([]*InstanceStudent, error)
	FindExpectedByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
	FindInstancesWithAttendanceByStudentAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*ScheduledInstanceRow, error)
	FindNotScheduledCandidatesByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
	FindPlannedStudentIDsByDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]int64, error)
	FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]ParallelPresence, error)
	HasPlannedSlotsInRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) (bool, error)
	ListStudentInstanceRefsBefore(arg0 context.Context, arg1 calendar.Date) ([]StudentInstanceRef, error)
	MarkExpectedAbsentByActiveGroupIDs(arg0 context.Context, arg1 []int64, arg2 time.Time, arg3 []StudentInstanceRef) error
	MarkNotScheduled(arg0 context.Context, arg1 []StudentInstanceRef) error
	ReconcileAttendanceInterval(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time, arg4 *time.Time, arg5 time.Time, arg6 *time.Time) (bool, error)
	ReleasePartialAbsence(arg0 context.Context, arg1 int64) (int, error)
	ReleaseStatusDay(arg0 context.Context, arg1 int64) (int, error)
	RestoreArchivedByTransition(arg0 context.Context, arg1 int64, arg2 []int64, arg3 calendar.Date) (int, error)
	Update(arg0 context.Context, arg1 *InstanceStudent) error
	UpdateAttendanceCheckout(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) error
	UpdateAttendanceCheckoutBatch(arg0 context.Context, arg1 []InstanceStudentKey, arg2 time.Time) error
	UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 AttendanceFieldPatch) error
	UpdateAttendanceFromCheckin(arg0 context.Context, arg1 int64, arg2 int64, arg3 time.Time) (bool, error)
	UpdateAttendanceFromCheckinBatch(arg0 context.Context, arg1 []InstanceStudentKey, arg2 time.Time) error
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

type OperationInstanceStaff interface {
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStaff, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStaff, error)
}

type intoOperationInstanceStaff struct {
	value OperationInstanceStaff
}

func IntoOperationInstanceStaff(value OperationInstanceStaff) ports.OperationInstanceStaff {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.OperationInstanceStaff); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeOperationInstanceStaff() ports.OperationInstanceStaff
	}); ok {
		return original.nativeOperationInstanceStaff()
	}
	if value == nil {
		return nil
	}
	return intoOperationInstanceStaff{value: value}
}
func (adapter intoOperationInstanceStaff) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative6(result0), result1
}
func (adapter intoOperationInstanceStaff) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStaff, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative6(result0), result1
}

type OperationInstances interface {
	FindByActiveGroupID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByID(arg0 context.Context, arg1 int64) (*ActivityInstance, error)
	FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*ActivityInstance, error)
}

type intoOperationInstances struct {
	value OperationInstances
}

func IntoOperationInstances(value OperationInstances) ports.OperationInstances {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.OperationInstances); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeOperationInstances() ports.OperationInstances
	}); ok {
		return original.nativeOperationInstances()
	}
	if value == nil {
		return nil
	}
	return intoOperationInstances{value: value}
}
func (adapter intoOperationInstances) FindByActiveGroupID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByActiveGroupID(arg0, arg1)
	return toNative2(result0), result1
}
func (adapter intoOperationInstances) FindByID(arg0 context.Context, arg1 int64) (*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return toNative2(result0), result1
}
func (adapter intoOperationInstances) FindByTenantAndDate(arg0 context.Context, arg1 calendar.Date) ([]*records.ActivityInstance, error) {
	result0, result1 := adapter.value.FindByTenantAndDate(arg0, arg1)
	return toNative3(result0), result1
}

type OperationParticipants interface {
	FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*InstanceStudent, error)
	FindByInstanceID(arg0 context.Context, arg1 int64) ([]*InstanceStudent, error)
	FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*InstanceStudent, error)
	FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]ParallelPresence, error)
	UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 AttendanceFieldPatch) error
}

type intoOperationParticipants struct {
	value OperationParticipants
}

func IntoOperationParticipants(value OperationParticipants) ports.OperationParticipants {
	if original, ok := value.(interface{ nativeRecordSource() any }); ok {
		if capability, ok := original.nativeRecordSource().(ports.OperationParticipants); ok {
			return capability
		}
	}
	if original, ok := value.(interface {
		nativeOperationParticipants() ports.OperationParticipants
	}); ok {
		return original.nativeOperationParticipants()
	}
	if value == nil {
		return nil
	}
	return intoOperationParticipants{value: value}
}
func (adapter intoOperationParticipants) FindByInstanceAndStudent(arg0 context.Context, arg1 int64, arg2 int64) (*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceAndStudent(arg0, arg1, arg2)
	return toNative27(result0), result1
}
func (adapter intoOperationParticipants) FindByInstanceID(arg0 context.Context, arg1 int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceID(arg0, arg1)
	return toNative26(result0), result1
}
func (adapter intoOperationParticipants) FindByInstanceIDs(arg0 context.Context, arg1 []int64) ([]*records.InstanceStudent, error) {
	result0, result1 := adapter.value.FindByInstanceIDs(arg0, arg1)
	return toNative26(result0), result1
}
func (adapter intoOperationParticipants) FindPresentInOtherActiveInstances(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 []int64) ([]records.ParallelPresence, error) {
	result0, result1 := adapter.value.FindPresentInOtherActiveInstances(arg0, arg1, arg2, arg3)
	return toNative32(result0), result1
}
func (adapter intoOperationParticipants) UpdateAttendanceFields(arg0 context.Context, arg1 int64, arg2 records.AttendanceFieldPatch) error {
	result0 := adapter.value.UpdateAttendanceFields(arg0, arg1, fromNative28(arg2))
	return result0
}

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

type StudentArrivalExceptionRecords interface {
	Create(arg0 context.Context, arg1 *StudentArrivalException) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	DeletePastExceptions(arg0 context.Context, arg1 calendar.Date) (int64, error)
	FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalException, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentArrivalException, error)
	FindByStudentIDAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StudentArrivalException, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalException, error)
	FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error)
	Update(arg0 context.Context, arg1 *StudentArrivalException) error
}
type fromStudentArrivalExceptionRecords struct {
	value ports.StudentArrivalExceptionRecords
}

func FromStudentArrivalExceptionRecords(value ports.StudentArrivalExceptionRecords) StudentArrivalExceptionRecords {
	if original, ok := value.(interface {
		fixtureStudentArrivalExceptionRecords() StudentArrivalExceptionRecords
	}); ok {
		return original.fixtureStudentArrivalExceptionRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentArrivalExceptionRecords{value: value}
}
func (adapter fromStudentArrivalExceptionRecords) Create(arg0 context.Context, arg1 *StudentArrivalException) error {
	result0 := adapter.value.Create(arg0, toNative12(arg1))
	return result0
}
func (adapter fromStudentArrivalExceptionRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}
func (adapter fromStudentArrivalExceptionRecords) DeletePastExceptions(arg0 context.Context, arg1 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeletePastExceptions(arg0, arg1)
	return result0, result1
}
func (adapter fromStudentArrivalExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative12(result0), result1
}
func (adapter fromStudentArrivalExceptionRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative13(result0), result1
}
func (adapter fromStudentArrivalExceptionRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative12(result0), result1
}
func (adapter fromStudentArrivalExceptionRecords) FindByStudentIDAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative13(result0), result1
}
func (adapter fromStudentArrivalExceptionRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative13(result0), result1
}
func (adapter fromStudentArrivalExceptionRecords) FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalException, error) {
	result0, result1 := adapter.value.FindUpcomingByStudentID(arg0, arg1)
	return fromNative13(result0), result1
}
func (adapter fromStudentArrivalExceptionRecords) Update(arg0 context.Context, arg1 *StudentArrivalException) error {
	result0 := adapter.value.Update(arg0, toNative12(arg1))
	return result0
}

type StudentArrivalNoteRecords interface {
	Create(arg0 context.Context, arg1 *StudentArrivalNote) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error)
	FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalNote, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalNote, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentArrivalNote, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalNote, error)
	Update(arg0 context.Context, arg1 *StudentArrivalNote) error
}
type fromStudentArrivalNoteRecords struct {
	value ports.StudentArrivalNoteRecords
}

func FromStudentArrivalNoteRecords(value ports.StudentArrivalNoteRecords) StudentArrivalNoteRecords {
	if original, ok := value.(interface {
		fixtureStudentArrivalNoteRecords() StudentArrivalNoteRecords
	}); ok {
		return original.fixtureStudentArrivalNoteRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentArrivalNoteRecords{value: value}
}
func (adapter fromStudentArrivalNoteRecords) Create(arg0 context.Context, arg1 *StudentArrivalNote) error {
	result0 := adapter.value.Create(arg0, toNative41(arg1))
	return result0
}
func (adapter fromStudentArrivalNoteRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}
func (adapter fromStudentArrivalNoteRecords) DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeletePastNotes(arg0, arg1)
	return result0, result1
}
func (adapter fromStudentArrivalNoteRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative41(result0), result1
}
func (adapter fromStudentArrivalNoteRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative42(result0), result1
}
func (adapter fromStudentArrivalNoteRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative42(result0), result1
}
func (adapter fromStudentArrivalNoteRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentArrivalNote, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative42(result0), result1
}
func (adapter fromStudentArrivalNoteRecords) Update(arg0 context.Context, arg1 *StudentArrivalNote) error {
	result0 := adapter.value.Update(arg0, toNative41(arg1))
	return result0
}

type StudentArrivalScheduleRecords interface {
	Create(arg0 context.Context, arg1 *StudentArrivalSchedule) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalSchedule, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalSchedule, error)
	FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentArrivalSchedule, error)
	FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentArrivalSchedule, error)
	Update(arg0 context.Context, arg1 *StudentArrivalSchedule) error
	UpsertSchedule(arg0 context.Context, arg1 *StudentArrivalSchedule) error
}
type fromStudentArrivalScheduleRecords struct {
	value ports.StudentArrivalScheduleRecords
}

func FromStudentArrivalScheduleRecords(value ports.StudentArrivalScheduleRecords) StudentArrivalScheduleRecords {
	if original, ok := value.(interface {
		fixtureStudentArrivalScheduleRecords() StudentArrivalScheduleRecords
	}); ok {
		return original.fixtureStudentArrivalScheduleRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentArrivalScheduleRecords{value: value}
}
func (adapter fromStudentArrivalScheduleRecords) Create(arg0 context.Context, arg1 *StudentArrivalSchedule) error {
	result0 := adapter.value.Create(arg0, toNative14(arg1))
	return result0
}
func (adapter fromStudentArrivalScheduleRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}
func (adapter fromStudentArrivalScheduleRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative14(result0), result1
}
func (adapter fromStudentArrivalScheduleRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative15(result0), result1
}
func (adapter fromStudentArrivalScheduleRecords) FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDAndWeekday(arg0, arg1, arg2)
	return fromNative14(result0), result1
}
func (adapter fromStudentArrivalScheduleRecords) FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentArrivalSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndWeekday(arg0, arg1, arg2)
	return fromNative15(result0), result1
}
func (adapter fromStudentArrivalScheduleRecords) Update(arg0 context.Context, arg1 *StudentArrivalSchedule) error {
	result0 := adapter.value.Update(arg0, toNative14(arg1))
	return result0
}
func (adapter fromStudentArrivalScheduleRecords) UpsertSchedule(arg0 context.Context, arg1 *StudentArrivalSchedule) error {
	result0 := adapter.value.UpsertSchedule(arg0, toNative14(arg1))
	return result0
}

type StudentPickupExceptionRecords interface {
	Create(arg0 context.Context, arg1 *StudentPickupException) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	DeletePastExceptions(arg0 context.Context, arg1 calendar.Date) (int64, error)
	FindByID(arg0 context.Context, arg1 int64) (*StudentPickupException, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentPickupException, error)
	FindByStudentIDAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StudentPickupException, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupException, error)
	FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error)
	Update(arg0 context.Context, arg1 *StudentPickupException) error
}
type fromStudentPickupExceptionRecords struct {
	value ports.StudentPickupExceptionRecords
}

func FromStudentPickupExceptionRecords(value ports.StudentPickupExceptionRecords) StudentPickupExceptionRecords {
	if original, ok := value.(interface {
		fixtureStudentPickupExceptionRecords() StudentPickupExceptionRecords
	}); ok {
		return original.fixtureStudentPickupExceptionRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentPickupExceptionRecords{value: value}
}
func (adapter fromStudentPickupExceptionRecords) Create(arg0 context.Context, arg1 *StudentPickupException) error {
	result0 := adapter.value.Create(arg0, toNative16(arg1))
	return result0
}
func (adapter fromStudentPickupExceptionRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}
func (adapter fromStudentPickupExceptionRecords) DeletePastExceptions(arg0 context.Context, arg1 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeletePastExceptions(arg0, arg1)
	return result0, result1
}
func (adapter fromStudentPickupExceptionRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative16(result0), result1
}
func (adapter fromStudentPickupExceptionRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative17(result0), result1
}
func (adapter fromStudentPickupExceptionRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) (*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative16(result0), result1
}
func (adapter fromStudentPickupExceptionRecords) FindByStudentIDAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative17(result0), result1
}
func (adapter fromStudentPickupExceptionRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupException, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative17(result0), result1
}
func (adapter fromStudentPickupExceptionRecords) FindUpcomingByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupException, error) {
	result0, result1 := adapter.value.FindUpcomingByStudentID(arg0, arg1)
	return fromNative17(result0), result1
}
func (adapter fromStudentPickupExceptionRecords) Update(arg0 context.Context, arg1 *StudentPickupException) error {
	result0 := adapter.value.Update(arg0, toNative16(arg1))
	return result0
}

type StudentPickupNoteRecords interface {
	Create(arg0 context.Context, arg1 *StudentPickupNote) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error)
	FindByID(arg0 context.Context, arg1 int64) (*StudentPickupNote, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupNote, error)
	FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentPickupNote, error)
	FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupNote, error)
	Update(arg0 context.Context, arg1 *StudentPickupNote) error
}
type fromStudentPickupNoteRecords struct {
	value ports.StudentPickupNoteRecords
}

func FromStudentPickupNoteRecords(value ports.StudentPickupNoteRecords) StudentPickupNoteRecords {
	if original, ok := value.(interface {
		fixtureStudentPickupNoteRecords() StudentPickupNoteRecords
	}); ok {
		return original.fixtureStudentPickupNoteRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentPickupNoteRecords{value: value}
}
func (adapter fromStudentPickupNoteRecords) Create(arg0 context.Context, arg1 *StudentPickupNote) error {
	result0 := adapter.value.Create(arg0, toNative18(arg1))
	return result0
}
func (adapter fromStudentPickupNoteRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}
func (adapter fromStudentPickupNoteRecords) DeletePastNotes(arg0 context.Context, arg1 calendar.Date) (int64, error) {
	result0, result1 := adapter.value.DeletePastNotes(arg0, arg1)
	return result0, result1
}
func (adapter fromStudentPickupNoteRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative18(result0), result1
}
func (adapter fromStudentPickupNoteRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative19(result0), result1
}
func (adapter fromStudentPickupNoteRecords) FindByStudentIDAndDate(arg0 context.Context, arg1 int64, arg2 calendar.Date) ([]*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByStudentIDAndDate(arg0, arg1, arg2)
	return fromNative19(result0), result1
}
func (adapter fromStudentPickupNoteRecords) FindByStudentIDsAndDate(arg0 context.Context, arg1 []int64, arg2 calendar.Date) ([]*StudentPickupNote, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndDate(arg0, arg1, arg2)
	return fromNative19(result0), result1
}
func (adapter fromStudentPickupNoteRecords) Update(arg0 context.Context, arg1 *StudentPickupNote) error {
	result0 := adapter.value.Update(arg0, toNative18(arg1))
	return result0
}

type StudentPickupScheduleRecords interface {
	Create(arg0 context.Context, arg1 *StudentPickupSchedule) error
	DeleteByStudentID(arg0 context.Context, arg1 int64) error
	FindByID(arg0 context.Context, arg1 int64) (*StudentPickupSchedule, error)
	FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupSchedule, error)
	FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentPickupSchedule, error)
	FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentPickupSchedule, error)
	Update(arg0 context.Context, arg1 *StudentPickupSchedule) error
	UpsertSchedule(arg0 context.Context, arg1 *StudentPickupSchedule) error
}
type fromStudentPickupScheduleRecords struct {
	value ports.StudentPickupScheduleRecords
}

func FromStudentPickupScheduleRecords(value ports.StudentPickupScheduleRecords) StudentPickupScheduleRecords {
	if original, ok := value.(interface {
		fixtureStudentPickupScheduleRecords() StudentPickupScheduleRecords
	}); ok {
		return original.fixtureStudentPickupScheduleRecords()
	}
	if value == nil {
		return nil
	}
	return fromStudentPickupScheduleRecords{value: value}
}
func (adapter fromStudentPickupScheduleRecords) Create(arg0 context.Context, arg1 *StudentPickupSchedule) error {
	result0 := adapter.value.Create(arg0, toNative20(arg1))
	return result0
}
func (adapter fromStudentPickupScheduleRecords) DeleteByStudentID(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.DeleteByStudentID(arg0, arg1)
	return result0
}
func (adapter fromStudentPickupScheduleRecords) FindByID(arg0 context.Context, arg1 int64) (*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative20(result0), result1
}
func (adapter fromStudentPickupScheduleRecords) FindByStudentID(arg0 context.Context, arg1 int64) ([]*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByStudentID(arg0, arg1)
	return fromNative21(result0), result1
}
func (adapter fromStudentPickupScheduleRecords) FindByStudentIDAndWeekday(arg0 context.Context, arg1 int64, arg2 int) (*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDAndWeekday(arg0, arg1, arg2)
	return fromNative20(result0), result1
}
func (adapter fromStudentPickupScheduleRecords) FindByStudentIDsAndWeekday(arg0 context.Context, arg1 []int64, arg2 int) ([]*StudentPickupSchedule, error) {
	result0, result1 := adapter.value.FindByStudentIDsAndWeekday(arg0, arg1, arg2)
	return fromNative21(result0), result1
}
func (adapter fromStudentPickupScheduleRecords) Update(arg0 context.Context, arg1 *StudentPickupSchedule) error {
	result0 := adapter.value.Update(arg0, toNative20(arg1))
	return result0
}
func (adapter fromStudentPickupScheduleRecords) UpsertSchedule(arg0 context.Context, arg1 *StudentPickupSchedule) error {
	result0 := adapter.value.UpsertSchedule(arg0, toNative20(arg1))
	return result0
}

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

type WorkforceShiftRows interface {
	Create(arg0 context.Context, arg1 *StaffShift) error
	Delete(arg0 context.Context, arg1 int64) error
	FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*StaffShift, error)
	FindByID(arg0 context.Context, arg1 int64) (*StaffShift, error)
	FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StaffShift, error)
	FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*StaffShift, error)
	FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error)
	Update(arg0 context.Context, arg1 *StaffShift) error
}
type fromWorkforceShiftRows struct {
	value ports.WorkforceShiftRows
}

func FromWorkforceShiftRows(value ports.WorkforceShiftRows) WorkforceShiftRows {
	if original, ok := value.(interface{ fixtureWorkforceShiftRows() WorkforceShiftRows }); ok {
		return original.fixtureWorkforceShiftRows()
	}
	if value == nil {
		return nil
	}
	return fromWorkforceShiftRows{value: value}
}
func (adapter fromWorkforceShiftRows) Create(arg0 context.Context, arg1 *StaffShift) error {
	result0 := adapter.value.Create(arg0, toNative45(arg1))
	return result0
}
func (adapter fromWorkforceShiftRows) Delete(arg0 context.Context, arg1 int64) error {
	result0 := adapter.value.Delete(arg0, arg1)
	return result0
}
func (adapter fromWorkforceShiftRows) FindByDateRange(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]*StaffShift, error) {
	result0, result1 := adapter.value.FindByDateRange(arg0, arg1, arg2)
	return fromNative10(result0), result1
}
func (adapter fromWorkforceShiftRows) FindByID(arg0 context.Context, arg1 int64) (*StaffShift, error) {
	result0, result1 := adapter.value.FindByID(arg0, arg1)
	return fromNative45(result0), result1
}
func (adapter fromWorkforceShiftRows) FindByStaffAndDateRange(arg0 context.Context, arg1 int64, arg2 calendar.Date, arg3 calendar.Date) ([]*StaffShift, error) {
	result0, result1 := adapter.value.FindByStaffAndDateRange(arg0, arg1, arg2, arg3)
	return fromNative10(result0), result1
}
func (adapter fromWorkforceShiftRows) FindByStaffIDsAndDates(arg0 context.Context, arg1 []int64, arg2 []calendar.Date) ([]*StaffShift, error) {
	result0, result1 := adapter.value.FindByStaffIDsAndDates(arg0, arg1, arg2)
	return fromNative10(result0), result1
}
func (adapter fromWorkforceShiftRows) FindUsedCalendarWeeks(arg0 context.Context, arg1 calendar.Date, arg2 calendar.Date) ([]calendar.Date, error) {
	result0, result1 := adapter.value.FindUsedCalendarWeeks(arg0, arg1, arg2)
	return result0, result1
}
func (adapter fromWorkforceShiftRows) Update(arg0 context.Context, arg1 *StaffShift) error {
	result0 := adapter.value.Update(arg0, toNative45(arg1))
	return result0
}

func fromNative0(value *records.ActivityException) *ActivityException {
	if value == nil {
		return nil
	}
	return &ActivityException{*value}
}
func toNative0(value *ActivityException) *records.ActivityException {
	if value == nil {
		return nil
	}
	return &value.ActivityException
}
func fromNative1(value []*records.ActivityException) []*ActivityException {
	if value == nil {
		return nil
	}
	result := make([]*ActivityException, len(value))
	for index, row := range value {
		result[index] = fromNative0(row)
	}
	return result
}
func toNative1(value []*ActivityException) []*records.ActivityException {
	if value == nil {
		return nil
	}
	result := make([]*records.ActivityException, len(value))
	for index, row := range value {
		result[index] = toNative0(row)
	}
	return result
}
func fromNative2(value *records.ActivityInstance) *ActivityInstance {
	if value == nil {
		return nil
	}
	return &ActivityInstance{*value}
}
func toNative2(value *ActivityInstance) *records.ActivityInstance {
	if value == nil {
		return nil
	}
	return &value.ActivityInstance
}
func fromNative3(value []*records.ActivityInstance) []*ActivityInstance {
	if value == nil {
		return nil
	}
	result := make([]*ActivityInstance, len(value))
	for index, row := range value {
		result[index] = fromNative2(row)
	}
	return result
}
func toNative3(value []*ActivityInstance) []*records.ActivityInstance {
	if value == nil {
		return nil
	}
	result := make([]*records.ActivityInstance, len(value))
	for index, row := range value {
		result[index] = toNative2(row)
	}
	return result
}
func fromNative4(value records.ActivityCompletionSnapshot) ActivityCompletionSnapshot {
	return ActivityCompletionSnapshot{value}
}
func toNative4(value ActivityCompletionSnapshot) records.ActivityCompletionSnapshot {
	return value.ActivityCompletionSnapshot
}
func fromNative5(value []*records.ScheduledInstanceRow) []*ScheduledInstanceRow {
	if value == nil {
		return nil
	}
	result := make([]*ScheduledInstanceRow, len(value))
	for index, row := range value {
		result[index] = fromNative47(row)
	}
	return result
}
func toNative5(value []*ScheduledInstanceRow) []*records.ScheduledInstanceRow {
	if value == nil {
		return nil
	}
	result := make([]*records.ScheduledInstanceRow, len(value))
	for index, row := range value {
		result[index] = toNative47(row)
	}
	return result
}
func fromNative6(value []*records.InstanceStaff) []*InstanceStaff {
	if value == nil {
		return nil
	}
	result := make([]*InstanceStaff, len(value))
	for index, row := range value {
		result[index] = fromNative31(row)
	}
	return result
}
func toNative6(value []*InstanceStaff) []*records.InstanceStaff {
	if value == nil {
		return nil
	}
	result := make([]*records.InstanceStaff, len(value))
	for index, row := range value {
		result[index] = toNative31(row)
	}
	return result
}
func fromNative7(value *records.CalendarPeriod) *CalendarPeriod {
	if value == nil {
		return nil
	}
	return &CalendarPeriod{*value}
}
func toNative7(value *CalendarPeriod) *records.CalendarPeriod {
	if value == nil {
		return nil
	}
	return &value.CalendarPeriod
}
func fromNative8(value []*records.CalendarPeriod) []*CalendarPeriod {
	if value == nil {
		return nil
	}
	result := make([]*CalendarPeriod, len(value))
	for index, row := range value {
		result[index] = fromNative7(row)
	}
	return result
}
func toNative8(value []*CalendarPeriod) []*records.CalendarPeriod {
	if value == nil {
		return nil
	}
	result := make([]*records.CalendarPeriod, len(value))
	for index, row := range value {
		result[index] = toNative7(row)
	}
	return result
}
func fromNative9(value map[int64]records.CalendarPeriodUsage) map[int64]CalendarPeriodUsage {
	if value == nil {
		return nil
	}
	result := make(map[int64]CalendarPeriodUsage, len(value))
	for key, row := range value {
		result[key] = fromNative48(row)
	}
	return result
}
func toNative9(value map[int64]CalendarPeriodUsage) map[int64]records.CalendarPeriodUsage {
	if value == nil {
		return nil
	}
	result := make(map[int64]records.CalendarPeriodUsage, len(value))
	for key, row := range value {
		result[key] = toNative48(row)
	}
	return result
}
func fromNative10(value []*records.StaffShift) []*StaffShift {
	if value == nil {
		return nil
	}
	result := make([]*StaffShift, len(value))
	for index, row := range value {
		result[index] = fromNative45(row)
	}
	return result
}
func toNative10(value []*StaffShift) []*records.StaffShift {
	if value == nil {
		return nil
	}
	result := make([]*records.StaffShift, len(value))
	for index, row := range value {
		result[index] = toNative45(row)
	}
	return result
}

func fromNative12(value *records.StudentArrivalException) *StudentArrivalException {
	if value == nil {
		return nil
	}
	return &StudentArrivalException{*value}
}
func toNative12(value *StudentArrivalException) *records.StudentArrivalException {
	if value == nil {
		return nil
	}
	return &value.StudentArrivalException
}
func fromNative13(value []*records.StudentArrivalException) []*StudentArrivalException {
	if value == nil {
		return nil
	}
	result := make([]*StudentArrivalException, len(value))
	for index, row := range value {
		result[index] = fromNative12(row)
	}
	return result
}
func toNative13(value []*StudentArrivalException) []*records.StudentArrivalException {
	if value == nil {
		return nil
	}
	result := make([]*records.StudentArrivalException, len(value))
	for index, row := range value {
		result[index] = toNative12(row)
	}
	return result
}
func fromNative14(value *records.StudentArrivalSchedule) *StudentArrivalSchedule {
	if value == nil {
		return nil
	}
	return &StudentArrivalSchedule{*value}
}
func toNative14(value *StudentArrivalSchedule) *records.StudentArrivalSchedule {
	if value == nil {
		return nil
	}
	return &value.StudentArrivalSchedule
}
func fromNative15(value []*records.StudentArrivalSchedule) []*StudentArrivalSchedule {
	if value == nil {
		return nil
	}
	result := make([]*StudentArrivalSchedule, len(value))
	for index, row := range value {
		result[index] = fromNative14(row)
	}
	return result
}

func fromNative16(value *records.StudentPickupException) *StudentPickupException {
	if value == nil {
		return nil
	}
	return &StudentPickupException{*value}
}
func toNative16(value *StudentPickupException) *records.StudentPickupException {
	if value == nil {
		return nil
	}
	return &value.StudentPickupException
}
func fromNative17(value []*records.StudentPickupException) []*StudentPickupException {
	if value == nil {
		return nil
	}
	result := make([]*StudentPickupException, len(value))
	for index, row := range value {
		result[index] = fromNative16(row)
	}
	return result
}

func fromNative18(value *records.StudentPickupNote) *StudentPickupNote {
	if value == nil {
		return nil
	}
	return &StudentPickupNote{*value}
}
func toNative18(value *StudentPickupNote) *records.StudentPickupNote {
	if value == nil {
		return nil
	}
	return &value.StudentPickupNote
}
func fromNative19(value []*records.StudentPickupNote) []*StudentPickupNote {
	if value == nil {
		return nil
	}
	result := make([]*StudentPickupNote, len(value))
	for index, row := range value {
		result[index] = fromNative18(row)
	}
	return result
}

func fromNative20(value *records.StudentPickupSchedule) *StudentPickupSchedule {
	if value == nil {
		return nil
	}
	return &StudentPickupSchedule{*value}
}
func toNative20(value *StudentPickupSchedule) *records.StudentPickupSchedule {
	if value == nil {
		return nil
	}
	return &value.StudentPickupSchedule
}
func fromNative21(value []*records.StudentPickupSchedule) []*StudentPickupSchedule {
	if value == nil {
		return nil
	}
	result := make([]*StudentPickupSchedule, len(value))
	for index, row := range value {
		result[index] = fromNative20(row)
	}
	return result
}

func fromNative26(value []*records.InstanceStudent) []*InstanceStudent {
	if value == nil {
		return nil
	}
	result := make([]*InstanceStudent, len(value))
	for index, row := range value {
		result[index] = fromNative27(row)
	}
	return result
}
func toNative26(value []*InstanceStudent) []*records.InstanceStudent {
	if value == nil {
		return nil
	}
	result := make([]*records.InstanceStudent, len(value))
	for index, row := range value {
		result[index] = toNative27(row)
	}
	return result
}
func fromNative27(value *records.InstanceStudent) *InstanceStudent {
	if value == nil {
		return nil
	}
	return &InstanceStudent{*value}
}
func toNative27(value *InstanceStudent) *records.InstanceStudent {
	if value == nil {
		return nil
	}
	return &value.InstanceStudent
}
func fromNative28(value records.AttendanceFieldPatch) AttendanceFieldPatch {
	return AttendanceFieldPatch{value}
}
func toNative28(value AttendanceFieldPatch) records.AttendanceFieldPatch {
	return value.AttendanceFieldPatch
}
func fromNative29(value *records.Dateframe) *Dateframe {
	if value == nil {
		return nil
	}
	return &Dateframe{*value}
}
func toNative29(value *Dateframe) *records.Dateframe {
	if value == nil {
		return nil
	}
	return &value.Dateframe
}
func fromNative30(value []records.StudentInstanceRef) []StudentInstanceRef {
	if value == nil {
		return nil
	}
	result := make([]StudentInstanceRef, len(value))
	for index, row := range value {
		result[index] = fromNative49(row)
	}
	return result
}
func toNative30(value []StudentInstanceRef) []records.StudentInstanceRef {
	if value == nil {
		return nil
	}
	result := make([]records.StudentInstanceRef, len(value))
	for index, row := range value {
		result[index] = toNative49(row)
	}
	return result
}
func fromNative31(value *records.InstanceStaff) *InstanceStaff {
	if value == nil {
		return nil
	}
	return &InstanceStaff{*value}
}
func toNative31(value *InstanceStaff) *records.InstanceStaff {
	if value == nil {
		return nil
	}
	return &value.InstanceStaff
}
func fromNative32(value []records.ParallelPresence) []ParallelPresence {
	if value == nil {
		return nil
	}
	result := make([]ParallelPresence, len(value))
	for index, row := range value {
		result[index] = fromNative50(row)
	}
	return result
}
func toNative32(value []ParallelPresence) []records.ParallelPresence {
	if value == nil {
		return nil
	}
	result := make([]records.ParallelPresence, len(value))
	for index, row := range value {
		result[index] = toNative50(row)
	}
	return result
}
func fromNative33(value []records.InstanceStudentKey) []InstanceStudentKey {
	if value == nil {
		return nil
	}
	result := make([]InstanceStudentKey, len(value))
	for index, row := range value {
		result[index] = fromNative51(row)
	}
	return result
}
func toNative33(value []InstanceStudentKey) []records.InstanceStudentKey {
	if value == nil {
		return nil
	}
	result := make([]records.InstanceStudentKey, len(value))
	for index, row := range value {
		result[index] = toNative51(row)
	}
	return result
}
func fromNative34(value *records.PlanningTrack) *PlanningTrack {
	if value == nil {
		return nil
	}
	return &PlanningTrack{*value}
}
func toNative34(value *PlanningTrack) *records.PlanningTrack {
	if value == nil {
		return nil
	}
	return &value.PlanningTrack
}
func fromNative35(value []*records.PlanningTrack) []*PlanningTrack {
	if value == nil {
		return nil
	}
	result := make([]*PlanningTrack, len(value))
	for index, row := range value {
		result[index] = fromNative34(row)
	}
	return result
}

func fromNative36(value *records.RecurrenceRule) *RecurrenceRule {
	if value == nil {
		return nil
	}
	return &RecurrenceRule{*value}
}
func toNative36(value *RecurrenceRule) *records.RecurrenceRule {
	if value == nil {
		return nil
	}
	return &value.RecurrenceRule
}
func fromNative37(value []*records.RecurrenceRule) []*RecurrenceRule {
	if value == nil {
		return nil
	}
	result := make([]*RecurrenceRule, len(value))
	for index, row := range value {
		result[index] = fromNative36(row)
	}
	return result
}

func fromNative41(value *records.StudentArrivalNote) *StudentArrivalNote {
	if value == nil {
		return nil
	}
	return &StudentArrivalNote{*value}
}
func toNative41(value *StudentArrivalNote) *records.StudentArrivalNote {
	if value == nil {
		return nil
	}
	return &value.StudentArrivalNote
}
func fromNative42(value []*records.StudentArrivalNote) []*StudentArrivalNote {
	if value == nil {
		return nil
	}
	result := make([]*StudentArrivalNote, len(value))
	for index, row := range value {
		result[index] = fromNative41(row)
	}
	return result
}

func fromNative43(value *records.Timeframe) *Timeframe {
	if value == nil {
		return nil
	}
	return &Timeframe{*value}
}
func toNative43(value *Timeframe) *records.Timeframe {
	if value == nil {
		return nil
	}
	return &value.Timeframe
}
func fromNative44(value []*records.Timeframe) []*Timeframe {
	if value == nil {
		return nil
	}
	result := make([]*Timeframe, len(value))
	for index, row := range value {
		result[index] = fromNative43(row)
	}
	return result
}
func toNative44(value []*Timeframe) []*records.Timeframe {
	if value == nil {
		return nil
	}
	result := make([]*records.Timeframe, len(value))
	for index, row := range value {
		result[index] = toNative43(row)
	}
	return result
}
func fromNative45(value *records.StaffShift) *StaffShift {
	if value == nil {
		return nil
	}
	return &StaffShift{*value}
}
func toNative45(value *StaffShift) *records.StaffShift {
	if value == nil {
		return nil
	}
	return &value.StaffShift
}

func fromNative47(value *records.ScheduledInstanceRow) *ScheduledInstanceRow {
	if value == nil {
		return nil
	}
	return &ScheduledInstanceRow{*value}
}
func toNative47(value *ScheduledInstanceRow) *records.ScheduledInstanceRow {
	if value == nil {
		return nil
	}
	return &value.ScheduledInstanceRow
}
func fromNative48(value records.CalendarPeriodUsage) CalendarPeriodUsage {
	return CalendarPeriodUsage{value}
}
func toNative48(value CalendarPeriodUsage) records.CalendarPeriodUsage {
	return value.CalendarPeriodUsage
}
func fromNative49(value records.StudentInstanceRef) StudentInstanceRef {
	return StudentInstanceRef{value}
}
func toNative49(value StudentInstanceRef) records.StudentInstanceRef {
	return value.StudentInstanceRef
}
func fromNative50(value records.ParallelPresence) ParallelPresence {
	return ParallelPresence{value}
}
func toNative50(value ParallelPresence) records.ParallelPresence {
	return value.ParallelPresence
}
func fromNative51(value records.InstanceStudentKey) InstanceStudentKey {
	return InstanceStudentKey{value}
}
func toNative51(value InstanceStudentKey) records.InstanceStudentKey {
	return value.InstanceStudentKey
}

func (adapter fromActivityExceptionRecords) nativeActivityExceptionRecords() ports.ActivityExceptionRecords {
	return adapter.value
}
func (adapter intoActivityExceptionRecords) fixtureActivityExceptionRecords() ActivityExceptionRecords {
	return adapter.value
}

func (adapter fromActivityInstanceRecords) nativeActivityInstanceRecords() ports.ActivityInstanceRecords {
	return adapter.value
}
func (adapter intoActivityInstanceRecords) fixtureActivityInstanceRecords() ActivityInstanceRecords {
	return adapter.value
}

func (adapter fromActivityRecoveryRecords) nativeActivityRecoveryRecords() ports.ActivityRecoveryRecords {
	return adapter.value
}
func (adapter intoActivityRecoveryRecords) fixtureActivityRecoveryRecords() ActivityRecoveryRecords {
	return adapter.value
}

func (adapter fromCalendarPeriodRecords) nativeCalendarPeriodRecords() ports.CalendarPeriodRecords {
	return adapter.value
}
func (adapter intoCalendarPeriodRecords) fixtureCalendarPeriodRecords() CalendarPeriodRecords {
	return adapter.value
}

func (adapter fromInstanceIdempotencyRecords) nativeInstanceIdempotencyRecords() ports.InstanceIdempotencyRecords {
	return adapter.value
}
func (adapter intoInstanceIdempotencyRecords) fixtureInstanceIdempotencyRecords() InstanceIdempotencyRecords {
	return adapter.value
}

func (adapter fromInstanceStaffRecords) nativeInstanceStaffRecords() ports.InstanceStaffRecords {
	return adapter.value
}
func (adapter intoInstanceStaffRecords) fixtureInstanceStaffRecords() InstanceStaffRecords {
	return adapter.value
}

func (adapter fromInstanceStudentRecords) nativeInstanceStudentRecords() ports.InstanceStudentRecords {
	return adapter.value
}
func (adapter intoInstanceStudentRecords) fixtureInstanceStudentRecords() InstanceStudentRecords {
	return adapter.value
}

func (adapter fromTimeframeRecords) nativeTimeframeRecords() ports.TimeframeRecords {
	return adapter.value
}
func (adapter intoTimeframeRecords) fixtureTimeframeRecords() TimeframeRecords { return adapter.value }

func (adapter fromActivityExceptionRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromActivityInstanceRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromActivityRecoveryRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromCalendarPeriodRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromCalendarPeriodUsageRepository) nativeRecordSource() any { return adapter.value }

func (adapter fromDateframeRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromInstanceIdempotencyRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromInstanceLifecycle) nativeRecordSource() any { return adapter.value }

func (adapter fromInstanceStaffRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromInstanceStudentRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromPlanningTrackRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromRecurrenceRuleRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromStudentArrivalExceptionRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromStudentArrivalNoteRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromStudentArrivalScheduleRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromStudentPickupExceptionRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromStudentPickupNoteRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromStudentPickupScheduleRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromTimeframeRecords) nativeRecordSource() any { return adapter.value }

func (adapter fromWorkforceShiftRows) nativeRecordSource() any { return adapter.value }
