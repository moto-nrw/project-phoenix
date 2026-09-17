package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

type CareLifecycleTestRepositories struct {
	TimetableTestRepositories
	CareExit         usersModels.CareExitRepository
	CareExitCleanup  usersModels.CareExitCleanupRepository
	CareWithdrawal   usersModels.CareWithdrawalCompletionRepository
	TagReleaser      StudentTagReleaser
	StudentFieldEdit auditModels.StudentFieldEditRepository
}

func NewCareLifecycleTestRepositories(db *bun.DB, command auditModels.Command) (CareLifecycleTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	care, err := NewCarePlan(db, people, tt.InstanceStudent)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	r := &Factory{db: db,
		CareExit: usersRepo.NewCareExitRepository(db), CareExitCleanup: usersRepo.NewCareExitCleanupRepository(db, NewEnrollmentBookingProjection(enrollmentCompose.New()), careExitAssignments{capability: tt.Timetable}, newStudentPresence(db)),
		CareWithdrawal: NewCareWithdrawalCompletionRepository(care, people),
	}
	r.BindPeopleDirectory(people)
	r.bindDefaultFacilities(db)
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	r.bindSchoolCalendarAdapters(calendar, NewCalendarPeriodUsage(enrollmentCompose.New(), tt.Timetable))
	r.bindCarePlanAdapters(care)
	r.CareExitCleanup.(*usersRepo.CareExitCleanupRepository).BindActivityBookings(activityBookingDirectory{capability: tt.Timetable})
	return CareLifecycleTestRepositories{TimetableTestRepositories: tt, CareExit: r.CareExit, CareExitCleanup: r.CareExitCleanup,
		CareWithdrawal: r.CareWithdrawal, TagReleaser: NewStudentTagReleaser(people),
		StudentFieldEdit: studentFieldEditCommand{auditRepo.NewStudentFieldEditRepository(newTestAuditRuntime(db)), command}}, nil
}
