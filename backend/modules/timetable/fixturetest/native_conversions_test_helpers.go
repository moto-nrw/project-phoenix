//go:build testhooks

package fixturetest

import (
	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

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
