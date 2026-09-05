package repositories

import (
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type CareLifecycleTestRepositories struct {
	TimetableTestRepositories
	CareExit         usersModels.CareExitRepository
	CareExitCleanup  usersModels.CareExitCleanupRepository
	CareWithdrawal   usersModels.CareWithdrawalCompletionRepository
	GradeTransition  educationModels.GradeTransitionRepository
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
		CareExit: usersRepo.NewCareExitRepository(db), CareExitCleanup: usersRepo.NewCareExitCleanupRepository(db),
		CareWithdrawal: usersRepo.NewCareWithdrawalCompletionRepository(db), GradeTransition: educationRepo.NewGradeTransitionRepository(db),
	}
	r.BindPeopleDirectory(people)
	r.bindDefaultFacilities(db)
	calendar, err := NewSchoolCalendar(db)
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	r.bindSchoolCalendarAdapters(calendar)
	r.bindCarePlanAdapters(care)
	bookings, err := timetableCompose.New(timetableCompose.Dependencies{DB: db, Observe: func(timetableCompose.Observation) {}})
	if err != nil {
		return CareLifecycleTestRepositories{}, err
	}
	r.BindTimetable(bookings)
	return CareLifecycleTestRepositories{TimetableTestRepositories: tt, CareExit: r.CareExit, CareExitCleanup: r.CareExitCleanup,
		CareWithdrawal: r.CareWithdrawal, GradeTransition: r.GradeTransition,
		StudentFieldEdit: studentFieldEditCommand{auditRepo.NewStudentFieldEditRepository(newTestAuditRuntime(db)), command}}, nil
}
