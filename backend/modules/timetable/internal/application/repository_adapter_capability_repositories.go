package application

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// CapabilityRepositories are the retained activities and schedule
// repositories served over the Timetable capability alone (#2743); the
// composition adds the instance and participant adapters, which also run on
// the database and Student Presence.
type CapabilityRepositories struct {
	ActivityCategory   ports.ActivityCategoryRecords
	ActivityGroup      ports.ActivityGroupRecords
	ActivitySchedule   ports.ActivityScheduleRecords
	ActivitySupervisor ports.PlannedSupervisorRecords
	StudentEnrollment  ports.ActivityEnrollmentRecords
	Timeframe          ports.TimeframeRecords
	PlanningTrack      ports.PlanningTrackRecords
	RecurrenceRule     ports.RecurrenceRuleRecords
	ActivityException  ports.ActivityExceptionRecords
	InstanceStaff      ports.InstanceStaffRecords
}

// NewCapabilityRepositories builds the retained adapters over the Timetable
// owner. The activity group and supervisor rows are enriched with the group,
// staff and person names the RepositoryDirectories resolve.
func NewCapabilityRepositories(capability timetable.Capability, calendar schoolcalendar.Query, directories ports.RepositoryDirectories) CapabilityRepositories {
	groupRows := timetableActivityGroupRepository{
		timetable: capability, groups: directories.GroupNames, rooms: directories.RoomNames,
		calendar: calendar, shiftTypes: directories.ShiftTypes,
	}
	groupProjection := groupActivityGroupRepository{activityGroupTargets: groupRows, groups: directories.GroupNames}
	staffGroups := newStaffActivityGroupRepository(groupProjection, directories.StaffPersons)
	supervisors := staffSupervisorPlannedRepository{PlannedSupervisorRecords: timetableActivitySupervisorRepository{timetable: capability}, membership: directories.StaffPersons}
	return CapabilityRepositories{
		ActivityCategory:   timetableActivityCategoryRepository{timetable: capability},
		ActivityGroup:      newPersonActivityGroupRepository(staffGroups, directories.PersonNames),
		ActivitySchedule:   timetableActivityScheduleRepository{timetable: capability},
		ActivitySupervisor: personSupervisorPlannedRepository{PlannedSupervisorRecords: supervisors, persons: directories.PersonNames},
		StudentEnrollment:  timetableStudentEnrollmentRepository{timetable: capability, alumni: directories.StudentAlumni, names: directories.StudentNames},
		Timeframe:          timetableTimeframeRepository{timetable: capability},
		PlanningTrack:      timetablePlanningTrackRepository{timetable: capability},
		RecurrenceRule:     timetableRecurrenceRuleRepository{timetable: capability},
		ActivityException:  timetableActivityExceptionRepository{timetable: capability},
		InstanceStaff:      timetableInstanceStaffRepository{timetable: capability},
	}
}

// NewActivityGroupRecords serves the activity group rows of the owner
// without the name enrichment, for readers that only load groups by ID or
// list them with their category.
func NewActivityGroupRecords(capability timetable.Capability) ports.ActivityGroupRecords {
	return timetableActivityGroupRepository{timetable: capability}
}
