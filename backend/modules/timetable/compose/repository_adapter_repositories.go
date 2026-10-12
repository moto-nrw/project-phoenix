package compose

// The retained activities and schedule repository contracts are served over
// the Timetable & Activities owner (#2743). The instance and participant
// adapters also compose Student Presence, which owns the execution and the
// attendance since #2762; what other owners hold arrives through the
// consumer-owned RepositoryDirectories.

import (
	"github.com/uptrace/bun"

	activities "github.com/moto-nrw/project-phoenix/models/activities"
	schedule "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// RepositoryDependencies are what the retained Timetable adapters compose.
type RepositoryDependencies struct {
	DB        *bun.DB
	Timetable timetable.Capability
	// Presence runs the blocks and records their attendance.
	Presence *studentpresence.Module
	// PartialAbsences previews the blocks an earlier pickup excuses.
	PartialAbsences PartialAbsences
	// Calendar lists the active calendar periods of the template capacity.
	Calendar    schoolcalendar.Query
	Directories RepositoryDirectories
}

// Repositories are the retained activities and schedule repositories of the
// Timetable owner.
type Repositories struct {
	ActivityCategory    activities.CategoryRepository
	ActivityGroup       activities.GroupRepository
	ActivitySchedule    activities.ScheduleRepository
	ActivitySupervisor  activities.SupervisorPlannedRepository
	StudentEnrollment   activities.StudentEnrollmentRepository
	Timeframe           schedule.TimeframeRepository
	PlanningTrack       schedule.PlanningTrackRepository
	RecurrenceRule      schedule.RecurrenceRuleRepository
	ActivityException   schedule.ActivityExceptionRepository
	ActivityInstance    schedule.ActivityInstanceRepository
	InstanceIdempotency schedule.InstanceIdempotencyRepository
	InstanceStaff       schedule.InstanceStaffRepository
	InstanceStudent     schedule.InstanceStudentRepository
}

// NewRepositories builds the retained schedule adapters over the Timetable owner. The
// activity group and supervisor rows are enriched with the group, staff and
// person names the RepositoryDirectories resolve.
func NewRepositories(dependencies RepositoryDependencies) Repositories {
	capability, directories := dependencies.Timetable, dependencies.Directories
	groupRows := timetableActivityGroupRepository{
		timetable: capability, groups: directories.GroupNames, rooms: directories.RoomNames,
		calendar: dependencies.Calendar, shiftTypes: directories.ShiftTypes,
	}
	groupProjection := groupActivityGroupRepository{activityGroupTargets: groupRows, groups: directories.GroupNames}
	staffGroups := newStaffActivityGroupRepository(groupProjection, directories.StaffPersons)
	supervisors := staffSupervisorPlannedRepository{SupervisorPlannedRepository: timetableActivitySupervisorRepository{timetable: capability}, membership: directories.StaffPersons}
	instances := newTimetableActivityInstanceRepository(dependencies.DB, capability, dependencies.Presence)
	return Repositories{
		ActivityCategory:   timetableActivityCategoryRepository{timetable: capability},
		ActivityGroup:      newPersonActivityGroupRepository(staffGroups, directories.PersonNames),
		ActivitySchedule:   timetableActivityScheduleRepository{timetable: capability},
		ActivitySupervisor: personSupervisorPlannedRepository{SupervisorPlannedRepository: supervisors, persons: directories.PersonNames},
		StudentEnrollment:  timetableStudentEnrollmentRepository{timetable: capability, alumni: directories.StudentAlumni, names: directories.StudentNames},
		Timeframe:          timetableTimeframeRepository{timetable: capability},
		PlanningTrack:      timetablePlanningTrackRepository{timetable: capability},
		RecurrenceRule:     timetableRecurrenceRuleRepository{timetable: capability},
		ActivityException:  timetableActivityExceptionRepository{timetable: capability},
		ActivityInstance:   instances, InstanceIdempotency: instances,
		InstanceStaff:   timetableInstanceStaffRepository{timetable: capability},
		InstanceStudent: newTimetableInstanceStudentRepository(dependencies.DB, capability, dependencies.Presence, dependencies.PartialAbsences),
	}
}

// NewActivityGroupRecords serves the activity group rows of the owner
// without the name enrichment, for readers that only load groups by ID or
// list them with their category.
func NewActivityGroupRecords(capability timetable.Capability) activities.GroupRepository {
	return timetableActivityGroupRepository{timetable: capability}
}

// InstanceReads are the block reads of the Workforce planning and the
// shift-plan-sync workflow, in the owner's public vocabulary.
func (r Repositories) InstanceReads() TimetableInstanceReads {
	return NewTimetableInstanceReads(r.ActivityInstance)
}

// InstanceStaffReads are the staffing reads of the same consumers.
func (r Repositories) InstanceStaffReads() TimetableInstanceStaffReads {
	return NewTimetableInstanceStaffReads(r.InstanceStaff)
}

// GroupReads are the template reads of the same consumers.
func (r Repositories) GroupReads() TimetableGroupReads {
	return NewTimetableGroupReads(r.ActivityGroup)
}
