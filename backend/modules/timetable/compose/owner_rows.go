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
	Instances     ports.InstanceRows
	InstanceStaff timetable.InstanceStaffCapability
	Participants  ports.ParticipantRows
	Templates     ports.DataTemplates
	// Directories name what other owners hold on the Vorlagen list.
	Directories       ports.RepositoryDirectories
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

// SickCascadeRows are the Betreuungsplan rows the #1843 sick cascade reads,
// with the day-wide staffing lock (#1840) the caller supplies.
func (r OwnerRows) SickCascadeRows(dayLock func(context.Context, calendar.Date) error) application.SickCascadeTimetableRows {
	return application.NewSickCascadeTimetableRows(
		r.Instances.Reads, r.InstanceStaff, dayLock,
	)
}

// TemplateRows are the retained repository rows the Timetable owner's
// template writes and materialization run over (#3424 slice S2). The
// composition root fills them from its repository set.
type TemplateRows struct {
	Groups      ports.GroupRows
	Categories  timetable.CategoryQuery
	Schedules   timetable.ScheduleCapability
	Enrollments timetable.StudentEnrollmentCapability
	Supervisors timetable.PlannedSupervisorCapability
	// Directories name the enrolled children and the supervisors' persons.
	Directories     ports.RepositoryDirectories
	Instances       ports.InstanceRows
	InstanceStaff   timetable.InstanceStaffCapability
	Participants    ports.ParticipantRows
	Timeframes      timetable.TimeframeCapability
	CalendarPeriods ports.CalendarPeriodRecords
	Exceptions      timetable.ActivityExceptionCapability
	EducationGroups ports.EducationGroupDirectory
}
