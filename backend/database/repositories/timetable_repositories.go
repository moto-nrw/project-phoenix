package repositories

import (
	activities "github.com/moto-nrw/project-phoenix/models/activities"
	schedule "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableRepositories struct {
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

func newTimetableRepositories(capability timetable.Capability, people peopledirectory.Capability, groups schoolstructure.Query, rooms facilities.Query, calendar schoolcalendar.Query, membership schoolmembership.Capability, shiftTypes schedule.ShiftTypeRepository) timetableRepositories {
	groupRows := timetableActivityGroupRepository{timetable: capability, groups: groups, rooms: rooms, calendar: calendar, shiftTypes: shiftTypes}
	groupProjection := groupActivityGroupRepository{activityGroupTargets: groupRows, groups: groups}
	staffGroups := newStaffActivityGroupRepository(groupProjection, membership)
	supervisors := staffSupervisorPlannedRepository{SupervisorPlannedRepository: timetableActivitySupervisorRepository{timetable: capability}, membership: membership}
	instances := timetableActivityInstanceRepository{timetable: capability}
	return timetableRepositories{
		ActivityCategory:   timetableActivityCategoryRepository{timetable: capability},
		ActivityGroup:      newPersonActivityGroupRepository(staffGroups, people),
		ActivitySchedule:   timetableActivityScheduleRepository{timetable: capability},
		ActivitySupervisor: personSupervisorPlannedRepository{SupervisorPlannedRepository: supervisors, persons: people},
		StudentEnrollment:  timetableStudentEnrollmentRepository{timetable: capability, students: people},
		Timeframe:          timetableTimeframeRepository{timetable: capability},
		PlanningTrack:      timetablePlanningTrackRepository{timetable: capability},
		RecurrenceRule:     timetableRecurrenceRuleRepository{timetable: capability},
		ActivityException:  timetableActivityExceptionRepository{timetable: capability},
		ActivityInstance:   instances, InstanceIdempotency: instances,
		InstanceStaff:   timetableInstanceStaffRepository{timetable: capability},
		InstanceStudent: timetableInstanceStudentRepository{timetable: capability},
	}
}
