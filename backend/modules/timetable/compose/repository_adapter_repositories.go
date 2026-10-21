package compose

// The retained activities and schedule repository contracts are served over
// the Timetable & Activities owner (#2743). The instance and participant
// adapters also compose Student Presence, which owns the execution and the
// attendance since #2762; what other owners hold arrives through the
// consumer-owned RepositoryDirectories.

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
)

// RepositoryDependencies are what the retained Timetable adapters compose.
type RepositoryDependencies struct {
	DB        *bun.DB
	Timetable timetable.Capability
	// Presence runs the blocks and records their attendance.
	Presence studentpresence.Owner
	// PartialAbsences previews the blocks an earlier pickup excuses.
	PartialAbsences application.PartialAbsences
	Directories     ports.RepositoryDirectories
}

// Repositories are the retained repositories of the Timetable owner; the
// capability-typed entries are the owner capability itself. Directories are
// the names other owners hold, which the Vorlagen list shows.
type Repositories struct {
	Timetable           timetable.Capability
	Directories         ports.RepositoryDirectories
	ActivityCategory    ports.CategoryRows
	ActivitySchedule    timetable.ScheduleCapability
	ActivitySupervisor  timetable.PlannedSupervisorCapability
	StudentEnrollment   timetable.StudentEnrollmentCapability
	Timeframe           timetable.TimeframeCapability
	ActivityException   timetable.ActivityExceptionCapability
	ActivityInstance    ports.InstanceRows
	InstanceIdempotency timetable.ActivityInstanceCommand
	InstanceStaff       timetable.InstanceStaffCapability
	InstanceStudent     ports.InstanceStudentRecords
}

// NewRepositories builds the retained schedule adapters over the Timetable
// owner: the capability-only adapters from the application, the instance and
// participant adapters over the database and Student Presence here.
func NewRepositories(dependencies RepositoryDependencies, reads application.PresenceReads) Repositories {
	capability := dependencies.Timetable
	return Repositories{
		Timetable: capability, Directories: dependencies.Directories,
		ActivityCategory: capability, ActivitySchedule: capability, Timeframe: capability, ActivityException: capability, InstanceIdempotency: capability,
		ActivitySupervisor: capability, StudentEnrollment: capability, InstanceStaff: capability,
		ActivityInstance: ports.InstanceRows{Plans: capability, Sessions: dependencies.Presence, Reads: reads},
		InstanceStudent:  application.NewInstanceStudentRepository(reads, capability, dependencies.Presence, dependencies.PartialAbsences),
	}
}

// NewSessionInstanceCompletion completes the blocks of sessions a cleanup ends.
func NewSessionInstanceCompletion(sessions ports.InstanceSessions) application.SessionInstanceCompletion {
	return application.NewSessionInstanceCompletion(sessions)
}

// GroupReads are the template reads of the same consumers.
func (r Repositories) GroupReads() application.TimetableGroupReads {
	return application.NewTimetableGroupReads(r.Timetable)
}

// NativeRecords exposes native session and template ports over these same wiring values.
func (r Repositories) NativeRecords() interface {
	timetable.SessionRecords
	timetable.TemplateCatalog
} {
	return application.SessionRecords{Instances: r.ActivityInstance, Staff: r.InstanceStaff, Groups: r.Timetable}
}
