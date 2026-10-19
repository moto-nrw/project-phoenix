//go:build testhooks

package fixturetest

import (
	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
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

func ActivityExceptionModified() string { return records.ActivityExceptionModified }

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

func AttendanceStatusAbsent() string { return records.AttendanceStatusAbsent }

func AttendanceStatusExpected() string { return records.AttendanceStatusExpected }

func AttendanceStatusPresent() string { return records.AttendanceStatusPresent }

func AttendanceSubstatusExcused() string { return records.AttendanceSubstatusExcused }

func AttendanceSubstatusFieldTrip() string { return records.AttendanceSubstatusFieldTrip }

func AttendanceSubstatusLate() string { return records.AttendanceSubstatusLate }

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

// ClosingDay is a test fixture over the owner's native row, not a second row schema.
type ClosingDay struct{ records.ClosingDay }

func NewClosingDay(configure func(*ClosingDay)) *ClosingDay {
	value := new(ClosingDay)
	if configure != nil {
		configure(value)
	}
	return value
}

// Dateframe is a test fixture over the owner's native row, not a second row schema.
type Dateframe struct{ records.Dateframe }

func NewDateframe(configure func(*Dateframe)) *Dateframe {
	value := new(Dateframe)
	if configure != nil {
		configure(value)
	}
	return value
}

func FrequencyDaily() string { return records.FrequencyDaily }

func FrequencyMonthly() string { return records.FrequencyMonthly }

func FrequencyWeekly() string { return records.FrequencyWeekly }

// InstanceStaff is a test fixture over the owner's native row, not a second row schema.
type InstanceStaff struct{ records.InstanceStaff }

func NewInstanceStaff(configure func(*InstanceStaff)) *InstanceStaff {
	value := new(InstanceStaff)
	if configure != nil {
		configure(value)
	}
	return value
}

func InstanceStatusActive() string { return records.InstanceStatusActive }

func InstanceStatusCancelled() string { return records.InstanceStatusCancelled }

func InstanceStatusCompleted() string { return records.InstanceStatusCompleted }

func InstanceStatusPlanned() string { return records.InstanceStatusPlanned }

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

func PeriodTypeCustom() string { return records.PeriodTypeCustom }

func PeriodTypeHoliday() string { return records.PeriodTypeHoliday }

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

// StaffShift is a test fixture over the owner's native row, not a second row schema.
type StaffShift struct{ records.StaffShift }

func NewStaffShift(configure func(*StaffShift)) *StaffShift {
	value := new(StaffShift)
	if configure != nil {
		configure(value)
	}
	return value
}

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

func WeekdayThursday() int { return records.WeekdayThursday }

func WeekdayTuesday() int { return records.WeekdayTuesday }

func WeekdayWednesday() int { return records.WeekdayWednesday }
