package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

type RFIDTestModule struct {
	FeedbackStudents devicescanCompose.FeedbackStudents
	Users            users.PersonService
	TagAssignments   devicescanCompose.TagAssignments
}

// NewRFIDTestModule provides identity lookup and tag assignment, without
// constructing attendance, timetable or enrollment services.
func NewRFIDTestModule(db *bun.DB) (RFIDTestModule, error) {
	r, err := repositories.NewRFIDTestRepositories(db)
	if err != nil {
		return RFIDTestModule{}, err
	}
	people := users.NewPersonService(users.PersonServiceDependencies{
		PersonRepo: r.Membership.Person, StaffRepo: r.Membership.Staff, TeacherRepo: r.Membership.Teacher,
		AccountRepo: r.Membership.Account, StudentRepo: r.Student, RFIDRepo: r.RFID,
		DB: db, Logger: slog.Default(),
	})
	return RFIDTestModule{Users: people, FeedbackStudents: devicescanCompose.NewFeedbackStudents(people), TagAssignments: devicescanCompose.NewTagAssignments(people, nil)}, nil
}
