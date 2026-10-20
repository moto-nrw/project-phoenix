package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OwnerRows are the retained repository rows the Timetable owner's
// operational day, planner reads and ended-session completion run over
// (#3551). Their status carries the Student Presence session state the
// owner's own rows do not. The composition root fills them from its
// repository set and hands them to the owner's ports. The Facilities room
// names, School Structure group names and the Audit Platform's
// Änderungsprotokoll the owner may not name itself arrive as the ports the
// root binds over those owners.
type OwnerRows struct {
	Instances         ports.ActivityInstanceRecords
	InstanceStaff     ports.InstanceStaffRecords
	Participants      ports.InstanceStudentRecords
	Templates         ports.ActivityGroupRecords
	Categories        ports.DataCategories
	Students          RosterStudents
	EducationGroups   ports.EducationGroupNames
	Rooms             OwnerRooms
	PickupExceptions  ports.StudentPickupExceptionRecords
	ArrivalExceptions ports.StudentArrivalExceptionRecords
	// DeviationEvents reads and Deviations appends the Änderungsprotokoll.
	DeviationEvents ports.DeviationEventReader
	Deviations      ports.DeviationProtocol
	// Locks serializes attendance writes with a completion; optional.
	Locks ports.ActivityRecoveryRecords
}

// OwnerRooms names and locks the Facilities rooms for the operational day
// and the instance lifecycle.
type OwnerRooms interface {
	ports.RoomNames
	ports.LifecycleRooms
}

// OperationTemplates is the template read of the operational day, which the
// retained activity group repository serves beside its model interface.
func NewOperationTemplates(rows ports.ActivityGroupRecords) (ports.OperationTemplates, error) {
	return application.NewOperationTemplates(rows)
}

// DataTemplates is the Vorlagen read of the planner.
func NewDataTemplates(rows ports.ActivityGroupRecords) (ports.DataTemplates, error) {
	return application.NewDataTemplates(rows)
}

// EndedSessionInstances is the session listing of the ended-session
// completion.
func NewEndedSessionInstances(rows ports.ActivityInstanceRecords) (ports.EndedSessionInstances, error) {
	return application.NewEndedSessionInstances(rows)
}

// SickCascadeRows are the Betreuungsplan rows the #1843 sick cascade reads,
// with the day-wide staffing lock (#1840) the caller supplies.
func (r OwnerRows) SickCascadeRows(dayLock func(context.Context, calendar.Date) error) application.SickCascadeTimetableRows {
	return application.NewSickCascadeTimetableRows(
		r.Instances, r.InstanceStaff, dayLock,
	)
}

// TemplateRows are the retained repository rows the Timetable owner's
// template writes and materialization run over (#3424 slice S2). The
// composition root fills them from its repository set.
type TemplateRows struct {
	Groups          ports.ActivityGroupRecords
	Categories      timetable.CategoryQuery
	Schedules       timetable.ScheduleCapability
	Enrollments     ports.ActivityEnrollmentRecords
	Supervisors     ports.PlannedSupervisorRecords
	Instances       ports.ActivityInstanceRecords
	InstanceStaff   ports.InstanceStaffRecords
	Participants    ports.InstanceStudentRecords
	Timeframes      timetable.TimeframeCapability
	CalendarPeriods ports.CalendarPeriodRecords
	Exceptions      timetable.ActivityExceptionCapability
	EducationGroups ports.EducationGroupDirectory
}
