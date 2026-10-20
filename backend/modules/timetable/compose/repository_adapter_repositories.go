package compose

// The retained activities and schedule repository contracts are served over
// the Timetable & Activities owner (#2743). The instance and participant
// adapters also compose Student Presence, which owns the execution and the
// attendance since #2762; what other owners hold arrives through the
// consumer-owned RepositoryDirectories.

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
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
	// Calendar lists the active calendar periods of the template capacity.
	Calendar    schoolcalendar.Query
	Directories ports.RepositoryDirectories
}

// Repositories are the retained repositories of the Timetable owner; the
// capability-typed entries are the owner capability itself.
type Repositories struct {
	ActivityCategory    ports.CategoryRows
	ActivityGroup       ports.ActivityGroupRecords
	ActivitySchedule    timetable.ScheduleCapability
	ActivitySupervisor  ports.PlannedSupervisorRecords
	StudentEnrollment   ports.ActivityEnrollmentRecords
	Timeframe           timetable.TimeframeCapability
	ActivityException   timetable.ActivityExceptionCapability
	ActivityInstance    ports.ActivityInstanceRecords
	InstanceIdempotency timetable.ActivityInstanceCommand
	InstanceStaff       ports.InstanceStaffRecords
	InstanceStudent     ports.InstanceStudentRecords
}

// NewRepositories builds the retained schedule adapters over the Timetable
// owner: the capability-only adapters from the application, the instance and
// participant adapters over the database and Student Presence here.
func NewRepositories(dependencies RepositoryDependencies, reads application.PresenceReads) Repositories {
	capability := dependencies.Timetable
	rows := application.NewCapabilityRepositories(capability, dependencies.Calendar, dependencies.Directories)
	return Repositories{
		ActivityCategory: capability, ActivitySchedule: capability, Timeframe: capability, ActivityException: capability, InstanceIdempotency: capability,
		ActivityGroup: rows.ActivityGroup, ActivitySupervisor: rows.ActivitySupervisor, StudentEnrollment: rows.StudentEnrollment, InstanceStaff: rows.InstanceStaff,
		ActivityInstance: application.NewActivityInstanceRepository(reads, capability, dependencies.Presence),
		InstanceStudent:  application.NewInstanceStudentRepository(reads, capability, dependencies.Presence, dependencies.PartialAbsences),
	}
}

// NewActivityInstanceRepository serves the retained activity instance
// repository over the Timetable owner and Student Presence, which runs the
// blocks since #2762.
func NewActivityInstanceRepository(reads application.PresenceReads, capability timetable.ActivityInstanceCapability, presence application.ActivityInstanceSessions) ports.ActivityInstanceRecords {
	return application.NewActivityInstanceRepository(reads, capability, presence)
}

// NewActivityGroupRecords serves the activity group rows of the owner
// without the name enrichment, for readers that only load groups by ID or
// list them with their category.
func NewActivityGroupRecords(capability timetable.Capability) ports.ActivityGroupRecords {
	return application.NewActivityGroupRecords(capability)
}

// GroupReads are the template reads of the same consumers.
func (r Repositories) GroupReads() application.TimetableGroupReads {
	return application.NewTimetableGroupReads(r.ActivityGroup)
}

// NativeRecords exposes native session and template ports over these same wiring values.
func (r Repositories) NativeRecords() interface {
	timetable.SessionRecords
	timetable.TemplateCatalog
} {
	return application.SessionRecords{Instances: r.ActivityInstance, Staff: r.InstanceStaff, Groups: r.ActivityGroup}
}
