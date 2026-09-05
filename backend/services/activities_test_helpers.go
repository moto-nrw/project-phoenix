package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

type ActivitiesTestModule struct {
	Activities  activities.ActivityService
	Schedule    schedule.Service
	Users       users.PersonService
	UserContext usercontext.UserContextService
}

func NewActivitiesTestModule(db *bun.DB) (ActivitiesTestModule, error) {
	r, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	categories, err := timetableCompose.New(timetableCompose.Dependencies{DB: db, Observe: func(timetableCompose.Observation) {}})
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	activityService, err := activities.NewService(categories, r.ActivityGroup, r.ActivitySchedule,
		r.ActivitySupervisor, r.StudentEnrollment, r.ActiveGroup, r.Staff, r.Student)
	if err != nil {
		return ActivitiesTestModule{}, err
	}
	// Activities consumes schedule reads and staff-directory reads only.
	return ActivitiesTestModule{
		Activities: activityService, UserContext: identity,
		Schedule: schedule.NewServiceWithConfig(schedule.ServiceConfig{DateframeRepo: r.Dateframe, TimeframeRepo: r.Timeframe, RecurrenceRuleRepo: r.RecurrenceRule}),
		Users:    users.NewPersonService(users.PersonServiceDependencies{PersonRepo: r.Person, StaffRepo: r.Staff, TeacherRepo: r.Teacher, DB: db, Logger: slog.Default()}),
	}, nil
}
